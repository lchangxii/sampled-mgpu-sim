#!/usr/bin/env python3.8
import argparse
import numpy as np
benchmarks_test=["matrixmultiplication"]
#patterns_test=["matrixmultiplication"]
import os

from testengine import RunBench,add_bench,results_name,remove_special_charactor,extend

pattern_parameters_dict = dict()
pattern_parameters_dict["ipcsampled"] = []
#for threshold in [0.025,0.25,2.5]:
for threshold in [0.25]:
    pattern_parameters_dict["ipcsampled"].append( "-timing -ipc-sampled -ipc-sampled-threshold=%(threshold)s"%locals() )

pattern_parameters_dict["branchsampled"] = ["-timing -branch-sampled"]
pattern_parameters_dict["mixedsampled"] = ["-timing -branch-sampled -sampled"]

pattern_parameters_dict["analysis"] = ["-only-analysis"]
pattern_parameters_dict["wgsampled"] = []

#for threshold in [0.05,0.1,0.2]:
#for threshold in [0.05]:
pattern_parameters_dict["wgsampled"].append( "-timing -sampled "%locals() )

pattern_parameters_dict["full"] = ["-timing"]
pattern_parameters_dict["inscount"] = ["-collect-instnum"]


benchmarks = dict()
benchmarks["relu"] =[]
reluparams_old=(2**np.array(range(2,2+5)))*64*1024
reluparams=extend(reluparams_old)

for param in reluparams:
    benchmarks["relu"].append( "./relu -length %s"%param )
convparams=["1022","2046","4094","8190","16382"]
benchmarks["simpleconvolution"] = []
for param in convparams:
    benchmarks["simpleconvolution"].append( "./simpleconvolution -width %s"%param)
benchmarks["matrixmultiplication"] = []
matmulparams=[4096,8192,16384,32768,65536]
matmulparams=np.array(matmulparams)*8

matmulparams=extend(matmulparams)
for param in matmulparams[:-1]:
   benchmarks["matrixmultiplication"].append( "./matrixmultiplication -x 64 -y 128 -z %s"%(param))

benchmarks["aes"] = "./aes -length 409600"
aesparams=[512*16*64,1048576,2097152,4194304,8388608]
aesparams=extend(aesparams)
benchmarks["aes"]=[]
for param in aesparams:
    benchmarks["aes"].append(  "./aes -length %d"%(param) )

benchmarks["fir"] =[]# "./fir -length 1024000"
firparams=[1,2,4,8,16,64]
firparams=np.array(firparams)*1024*64

firparams=extend(firparams)

for param in firparams:
    benchmarks["fir"].append(  "./fir -length %d"%(param) )



benchmarks["spmv"] = []

spmvparams=[8192,16384,32768,65536]

spmvparams=extend(spmvparams)
for param in spmvparams:
    benchmarks["spmv"].append("./spmv -dim %d"%param)


#benchmarks["bfs"] = []
#
##pagerankparams=[8192,16384,32768,65536]
##pagerankparams=[8192*8]
#pagerankparams=[8192*16]
#
#pagerankparams=extend(pagerankparams)
#for param in pagerankparams:
#    benchmarks["bfs"].append("./bfs -node %d --depth 1"%param)



general_parameter=["-magic-memory-copy"]

def get_args():
    parser = argparse.ArgumentParser(description="Executing All Benchmarks")

    parser.add_argument("-check" ,action="store_true" , default=False,help=" check the final result")
    parser.add_argument("-force" ,action="store_true" , default=False,help=" force to execute")
    parser.add_argument("-mode",type=str   ,nargs='+', default=["all"],help=" execution mode")
    parser.add_argument("-bench",type=str   , default="all",help=" benchmarks to execute")
    parser.add_argument("-arch",type=str   , default="r9nano",help="archtecture to simulate")
    parser.add_argument("-v",type=str   , default="0",help="version")
    parser.add_argument("-n",type=int   , default=8,help="core number")
    args = parser.parse_args()
    return args

args = get_args()


if args.arch!="r9nano":
    general_parameter.append( "--arch %s"%args.arch )
general_parameter=" ".join(general_parameter)
for bench,benchcmds in benchmarks.items():
    if type(benchcmds ) != list:
        benchcmds = [benchcmds]
        benchmarks[bench] = benchcmds
    for idx,benchcmd in enumerate(benchcmds):
        #print(benchcmd)
        benchmarks[bench][idx] = " ".join([ benchcmd, general_parameter] )

parameters = dict()
cwd=os.getcwd()
root_path = os.path.join(cwd,"..")
import copy



from sampledipc import processjson,extrapolate
def check_full_data( file_name ):
    data = processjson( file_name )
    return data["simtime"],data["walltime"]
def check_sampled_data(file_name):
    data = processjson( file_name )
    return data["simtime"],data["walltime"]

def check_data( file_name ,pattern,inscountfinal_name):

    try:
        #print(file_name)
        if pattern == "full":
            simtime,walltime = check_full_data(file_name)
        elif pattern in[ "wgsampled","mixedsampled","branchsampled"]:
            simtime,walltime = check_sampled_data(file_name)
        elif pattern == "ipcsampled":
            simtime,walltime=extrapolate( inscountfinal_name,file_name)
        else:
            simtime,walltime=0,0
    except FileNotFoundError:
        simtime,walltime=0,0

    return simtime,walltime 

pattern_order = ["full","mixedsampled","wgsampled","branchsampled","inscount","ipcsampled","analysis"]
output_all = []

#from pathlib import Path

home_dir = os.getenv('HOME')
#home_dir = Path.home()
result_dir=os.path.join(home_dir,"gpudata")
from my_utils import check_dir
check_dir(result_dir)

allcommands = []
def run_cmd(command,result_name,final_name,binary_dir):
    allcommands.append( RunBench(command,result_name,final_name,binary_dir) )

from sampledanalysis import bbvanalysis
benchid=-1
first_row = ["benchmark-command"]
for bench,bench_cmds in benchmarks.items():
#    if bench not in benchmarks_test:
#        continue
    if args.bench != "all":
        if args.bench != bench:
            continue
    binary_dir=os.path.join(root_path,bench)
    os.chdir( binary_dir )
    if not args.check:
        os.system("go build")
    if type(bench_cmds) != list:
        bench_cmds = [bench_cmds]
    for bench_cmd in bench_cmds:
        benchid += 1
        output_each_bench = [bench_cmd]
        for pattern in pattern_order:
            if args.mode[0] != "all":
                if pattern not in args.mode :
                    continue
            #print(pattern)
            if args.check:
                if pattern == "inscount" or pattern == "analysis":
                    continue
            for pattern_parameter in pattern_parameters_dict[pattern]:
                final_name,cmd = add_bench( bench, bench_cmd,"",pattern_parameter,pattern,args.v )
                final_name = os.path.join(result_dir,final_name)
                if args.check:

                    if pattern == "ipcsampled" :
                        pattern_parameter = pattern_parameters_dict["inscount"][0]
                        inscountfinal_name,_ = add_bench( bench, bench_cmd,"",pattern_parameter,"inscount",args.v )

                        inscountfinal_name = os.path.join(result_dir,inscountfinal_name)        

                    elif pattern == "analysis":
                        inscountfinal_name,_ = add_bench( bench, bench_cmd,"",pattern_parameter,"full",args.v )
                        inscountfinal_name = os.path.join(result_dir,inscountfinal_name)

                    else:
                        inscountfinal_name = None
                    if pattern == "analysis":
                        print(inscountfinal_name)
                        print(final_name)
                        simtime = 0
                        with open(final_name) as f:
                            data = f.read()
                            import json
                            data = json.loads(data) 
                            walltimes = data["analysistimes"]
                            walltime = sum(walltimes)
                    else:
                        simtime,walltime = check_data( final_name,pattern,inscountfinal_name)
                        if benchid == 0:
                            if pattern == "mixedsampled":
                                first_row += [ "Photon-Simtime","Photon-Walltime" ]
                            elif pattern == "full":
                                first_row += [ "MGPUSim-Simtime","MGPUSim-Walltime" ]
                            elif pattern == "branchsampled":
                                first_row += [ "BB-Simtime","BB-Walltime" ]
                            elif pattern == "wgsampled":
                                first_row += [ "Warp-Simtime","Warp-Walltime" ]
                            elif pattern == "ipcsampled":
                                first_row += [ "PKA-Simtime","PKA-Walltime" ]
                    if pattern not in ["inscount"]:
                        output_each_bench.append( str(simtime))
                        output_each_bench.append( str(walltime))
                else:
                    isExisting = os.path.exists( os.path.join(result_dir,final_name) )
                    if (not isExisting) or args.force:
                        run_cmd(cmd,results_name[pattern],final_name,binary_dir)
#                    print(cmd)
#                    os.system(cmd)

#                    oriname = results_name[pattern]
#                    cmd1 = "mv %(oriname)s %(final_name)s"%locals()
#                    print(cmd1) 
#                    os.system(cmd1)
        
        output_all.append(output_each_bench)
def run_command(command):
    command.RunCommand()
if not args.check:
    n_thread = args.n
    from multiprocessing import Pool
    with Pool(n_thread) as pool:
        pool.map(run_command,allcommands)
#    for command in allcommands:
#        run_command(command)
        

print( "\t".join(first_row) )
for output_each_bench in output_all:
    print( "\t".join(output_each_bench) )

####export excel
import copy
from to_excel import export_excel

if args.check and args.mode[0]=="all": ###only mode all export excel to use
    excel_dir=os.path.join(root_path,"sampledrunner")
    excel_export_base_path1 = os.path.join( home_dir , "artifact_evaluation")
    excel_export_base_path= os.path.join( excel_export_base_path1 ,"micro2023_figures")
    if not os.path.isdir( excel_export_base_path ):
        print( "Please git clone micro2023_figures repo" )
        print( "Run command \"git clone https://github.com/lchangxii/micro2023_figures.git\" in the directory \"%s\"."%excel_export_base_path1)
        exit(1)
    if args.arch == "r9nano":
        excel_dir = os.path.join( excel_export_base_path, "r9nano" )
        outputfile = "r9nano.xlsx"
    else:
        excel_dir = os.path.join( excel_export_base_path, "mi100" )
        outputfile = "mi100.xlsx"
    os.chdir( excel_dir )
    


    benchmarks = ["spmv","relu","matrix","aes","fir","conv"] 
    columns = [ "problemsize","simtime",	"walltime", "mixsimtime", "mixwalltime", "wfsimtime","wfwalltime" ,"bbsimtime","bbwalltime", "pkasimtime","pkawalltime"]

    problemsize = [
            [ "128","192","256","384","512","768","1K" ], ##spmv
            [ "4K","6K","8K","12K","16K","24K","32K","48K","64K"],##relu
            ["4K","6K","8K","12K","16K","24K","32K","64K"],##matmul
            ["512","768","1K","1.5K","2K","3K","4K","6K","8K"],
            ["1K","1.5K","2K","3K","4K","6K","8K","12K","16K","40K","64K"],
            ["4K","8K","16K","32K","64K"],
            ]
    bench2problemsize=dict()
    for bench_i,bench in enumerate( benchmarks ):
        bench2problemsize[bench] = problemsize[bench_i]
    #"mixaccuracy", "mixspeedup",  "wferror","wfspeedup","bbaccuracy","bbspeedup", "pkaaccuracy","pkaspeedup"]
    benchmark2data=dict()
   #pattern_order = ["full","mixedsampled","wgsampled","branchsampled","inscount","ipcsampled","analysis"]
    for bench in benchmarks:
       benchmark2data[bench] = []
    row_data = [ ]
    for output in output_all:
       ###bench name
    #    print(output)
        cmd = output[0]
        benchname = "spmv"
        for bench in benchmarks:
            if bench in cmd:
                benchname = bench
                break
     #   print(output)
        output_raw = output[1:]
      #  print(output_raw)
        output_raw = [float(elem) for elem in output_raw]
        row_data = copy.copy(output_raw)
        benchmark2data[bench].append(row_data)
    ##add problem size
    for bench,data in benchmark2data.items():
        problemsizes = bench2problemsize[bench]
        #print (bench)
        for problem_i, problem in enumerate( problemsizes):
            data[problem_i] = [problem] + data[problem_i]
##change matrix to matmul
    benchmark2data["matmul"] = benchmark2data["matrix"]
    del benchmark2data["matrix"]

    export_excel( outputfile, benchmark2data, columns )

    
           
















