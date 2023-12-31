#!/usr/bin/env python3.8
import argparse
import numpy as np
benchmarks_test=["matrixmultiplication"]
#patterns_test=["matrixmultiplication"]
import os

def extend(params_old):
    params=[]
    params.append( params_old[0] )
    for idx in range(len(params_old)-1):
        params.append( (params_old[idx+1]+params_old[idx])//2 )
        params.append( params_old[idx+1] )
    return params

pattern_parameters_dict = dict()
pattern_parameters_dict["ipcsampled"] = []
#for threshold in [0.025,0.25,2.5]:
for threshold in [0.25]:
    pattern_parameters_dict["ipcsampled"].append( "-timing -ipc-sampled -ipc-sampled-threshold=%(threshold)s"%locals() )

pattern_parameters_dict["branchsampled"] = ["-timing -branch-sampled"]
pattern_parameters_dict["mixedsampled"] = ["-timing -branch-sampled -sampled"]

pattern_parameters_dict["wgsampled"] = []

#for threshold in [0.05,0.1,0.2]:
#for threshold in [0.05]:
pattern_parameters_dict["wgsampled"].append( "-timing -sampled "%locals() )

pattern_parameters_dict["full"] = ["-timing"]
pattern_parameters_dict["inscount"] = ["-collect-instnum"]

pattern_parameters_dict["analysis"] = ["-only-analysis"]

benchmarks = dict()
benchmarks["conv2d"] = []
batch=1
enable_backward=False
convparams=[                                                     ##group inner
         [ batch,   3,224,224,64 ,3,3,1,1,1,1,enable_backward ],  ## 0,   0
         [ batch,  64,224,224,64 ,3,3,1,1,1,1,enable_backward ],  ## 0,   1
         [ batch,  64,112,112,128,3,3,1,1,1,1,enable_backward ],  ## 1,   0
         [ batch, 128,112,112,128,3,3,1,1,1,1,enable_backward ],  ## 1,   1,2
         [ batch, 128, 56, 56,256,3,3,1,1,1,1,enable_backward ],  ## 2,   0
         [ batch, 256, 56, 56,256,3,3,1,1,1,1,enable_backward ],  ## 2,   1,2
         [ batch, 256, 28, 28,512,3,3,1,1,1,1,enable_backward ],  ## 3,   0
         [ batch, 512, 28, 28,512,3,3,1,1,1,1,enable_backward ],  ## 3,   1,2
         [ batch, 512, 14, 14,512,3,3,1,1,1,1,enable_backward ],  ## 4,   0,1,2
        ]
for param in convparams:
    cmd = "./conv2d -N %d -C %d -H %d -W %d -output-channel %d -kernel-height %d -kernel-width %d -pad-x %d -pad-y %d -stride-x %d -stride-y %d"%(param[0],param[1],param[2],param[3],param[4],param[5],param[6],param[7],param[8],param[9],param[10])
    if param[11]:
        cmd += " -enable-backward"
    benchmarks["conv2d"].append( cmd )
benchmarks["maxpooling"] = []
params=[                                                     ##group inner
         [ batch,  64,224,224, 64 ,2,2,0,0,2,2,enable_backward ],  ## 0,   3
         [ batch, 128,112,112,128 ,2,2,0,0,2,2,enable_backward ],  ## 1,   4
         [ batch, 256, 56, 56,256 ,2,2,0,0,2,2,enable_backward ],  ## 2,   4
         [ batch, 512, 28, 28,512 ,2,2,0,0,2,2,enable_backward ],  ## 3,   4
         [ batch, 512, 14, 14,512 ,2,2,0,0,2,2,enable_backward ],  ## 4,   4
        ]
for param in params:
    cmd = "./maxpooling -N %d -C %d -H %d -W %d -output-channel %d -kernel-height %d -kernel-width %d -pad-x %d -pad-y %d -stride-x %d -stride-y %d"%(param[0],param[1],param[2],param[3],param[4],param[5],param[6],param[7],param[8],param[9],param[10])
    if param[11]:
        cmd += " -enable-backward"
    benchmarks["maxpooling"].append( cmd )

benchmarks["fulllayer"] = []
params=[                                                     ##group inner
         [ batch,  512*7*7, 512*2*2,enable_backward ],       ## 5,   0
         [ batch,  512*2*2,     200,enable_backward ],       ## 5,   1
        ]
for param in params:
    cmd = "./fulllayer -N %d -input-dim %d -output-dim %d "%(param[0],param[1],param[2])
    if param[-1]:
        cmd += " -enable-backward"
    benchmarks["fulllayer"].append( cmd )


general_parameter=["-magic-memory-copy"]

def get_args():
    parser = argparse.ArgumentParser(description="Executing All Benchmarks")

    parser.add_argument("--check" ,action="store_true" , default=False,help=" check the final result")
    parser.add_argument("--force" ,action="store_true" , default=False,help=" force to execute")
    parser.add_argument("--mode",type=str   ,nargs='+', default="all",help=" execution mode")
    parser.add_argument("--bench",type=str   , default="all",help=" benchmarks to execute")
    parser.add_argument("--arch",type=str   , default="r9nano",help="archtecture to simulate")
    parser.add_argument("--v",type=str   , default="0",help="version")
    parser.add_argument("--n",type=int   , default=16,help="parallel workloads")
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
results_name = dict()
results_name["full"] = "result.json"
results_name["inscount"] = "insnums.json"
results_name["wgsampled"] = "result.json"
results_name["branchsampled"] = "result.json"
results_name["mixedsampled"] = "result.json"
results_name["ipcsampled"] = "ipc_sampled_result.json"
results_name["analysis"] = "bbv_feature.json"

import copy


hash2filename = dict()
def remove_special_charactor(pattern_para):
    paras=copy.copy(pattern_para).replace(" ","_")
    paras=paras.replace("-","_")
    paras=paras.replace(".","_")
    paras=paras.replace("=","_")
    paras=paras.replace("/","_")
    return paras

def add_bench(bench,binary,parameter, pattern_para,pattern,v=None):
    list1= [ binary , parameter , pattern_para]
    #print(list1)
    cmd1 = " ".join( list1)
    
    cmdparas = remove_special_charactor(binary)
    pattern_paras=remove_special_charactor(pattern_para)

    oriname = results_name[pattern]
    oriname_rep=copy.copy(oriname).replace(".json","")
    if v==None:
        v=args.v
    else:
        v=str(v)
    if v=="0":
        final_name = oriname_rep +"_" + cmdparas+"_" + pattern_paras + ".json"
    else:
        final_name = oriname_rep +"_" + cmdparas+"_" + pattern_paras+"_v"+v + ".json"

    print(final_name)
    hash2filename[hash( cmd1 )] = final_name
#    os.system(cmd1)
    return final_name,cmd1
from sampledipc import processjson,extrapolate
from sampledanalysis import bbvanalysis
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
        elif pattern == "analysis":
            simtime,walltime= bbvanalysis( inscountfinal_name,file_name )
        else:
            simtime,walltime=0,0
    except FileNotFoundError:
        simtime,walltime=0,0

    return simtime,walltime 

pattern_order = ["full","mixedsampled","wgsampled","branchsampled","analysis","inscount","ipcsampled"]
output_all = []

#from pathlib import Path

home_dir = os.getenv('HOME')
#home_dir = Path.home()
result_dir=os.path.join(home_dir,"gpudata")
import tempfile
class RunBench:
    def __init__(self,command,result_name,final_name,binary_dir):
        self.command = command
        self.result_name = result_name
        self.final_name = os.path.join(result_dir,final_name)
        self.binary_dir = binary_dir
    def RunCommand(self):

        tmpdir_real=tempfile.TemporaryDirectory()
        tmpdir = tmpdir_real.name
        ###first copy all binary to this temperary directory
        print(tmpdir) 
        copy_binary_command = "cp -r %s/* %s"%(self.binary_dir,tmpdir)
        ##change workdir to tmpdir
        os.chdir( tmpdir )
        
        os.system(copy_binary_command)

        ##run command
        os.system(self.command)
        ##copy final result to gpudata directory
        copy_result_command = "mv %s %s"%(self.result_name,self.final_name) 
        #print(copy_result_command)
        
        os.system(copy_result_command)
        tmpdir_real.cleanup()
        ## print(tmpdir)


allcommands = []
def run_cmd(command,result_name,final_name,binary_dir):
    allcommands.append( RunBench(command,result_name,final_name,binary_dir) )

def run_bench_with_param( bench,bench_cmd ):
    binary_dir=os.path.join(root_path,bench)
    os.chdir( binary_dir )
    if not args.check:
        os.system("go build")

    output_each_bench = [bench_cmd]
    for pattern in pattern_order:
        if args.mode[0] != "all":
            if pattern not in args.mode :
                continue
        elif pattern == "analysis":
            continue

        for pattern_parameter in pattern_parameters_dict[pattern]:
            final_name,cmd = add_bench( bench, bench_cmd,"",pattern_parameter,pattern )
            if args.check:
                final_name = os.path.join(result_dir,final_name)

                if pattern == "ipcsampled":
                    pattern_parameter = pattern_parameters_dict["inscount"][0]
                    inscountfinal_name,_ = add_bench( bench, bench_cmd,"",pattern_parameter,"inscount" )

                    inscountfinal_name = os.path.join(result_dir,inscountfinal_name)
                elif pattern == "analysis":

                    pattern_parameter = pattern_parameters_dict["full"][0]
                    inscountfinal_name,_ = add_bench( bench, bench_cmd,"",pattern_parameter,"full" )
                    inscountfinal_name = os.path.join(result_dir,inscountfinal_name)
                else:
                    inscountfinal_name = None
                simtime,walltime = check_data( final_name,pattern,inscountfinal_name)
                if pattern not in ["inscount"]:
                    output_each_bench.append( str(simtime))
                    output_each_bench.append( str(walltime))
            else:
                isExisting = os.path.exists( os.path.join(result_dir,final_name) )
                if (not isExisting) or args.force:
                    run_cmd(cmd,results_name[pattern],final_name,binary_dir)

            output_all.append(output_each_bench)
from sampledanalysis import ExecuteEngine
execute_engine = ExecuteEngine()

cluster2simtime=dict()

def run_bench_with_sampled( bench,bench_cmd,pattern ):
    binary_dir=os.path.join(root_path,bench)
    os.chdir( binary_dir )


    pattern_parameter = pattern_parameters_dict["analysis"][0]
    analysis_name,_ = add_bench( bench, bench_cmd,"",pattern_parameter,"analysis" ,1)
    analysis_name = os.path.join(result_dir, analysis_name)
    print(analysis_name)
    kernels = execute_engine.decompose_data( analysis_name ) 
    walltime = 0
    simtime = 0
    if pattern == "kernelsampled":
        pattern = "full"
    else:
        pattern = "mixedsampled"
        pattern = "wgsampled"
    pattern_parameter = pattern_parameters_dict[pattern][0]
    full_name,_ = add_bench( bench, bench_cmd,"",pattern_parameter,pattern )

    full_name = os.path.join(result_dir, full_name)

    kernel_fulltimes = execute_engine.decompose_full_data( full_name )
    
    for idx,kernel in enumerate( kernels ):
        kerneltime = kernel_fulltimes[idx] 
        insnum = kernel.insnums
        ipc = insnum / (kerneltime*1e9)
        hitkernel, success = execute_engine.find_kernel(kernel)
        if success:
            cluster2simtime[ hitkernel ].append( ipc )
        else:
            simtime += kerneltime
            kernel.updatesim( kerneltime)
            execute_engine.update(  kernel )

            cluster2simtime[ kernel ] = [ ipc ]


if args.bench=="vgg16":
    benchparams = [] 
    conv2ds = benchmarks["conv2d"]
    conv2d_0_0 = conv2ds[0]
    conv2d_0_1 = conv2ds[1]
    conv2d_1_0 = conv2ds[2]
    conv2d_1_1 = conv2ds[3]
    conv2d_1_2 = conv2ds[3]
    conv2d_2_0 = conv2ds[4]
    conv2d_2_1 = conv2ds[5]
    conv2d_2_2 = conv2ds[5]
    conv2d_3_0 = conv2ds[6]
    conv2d_3_1 = conv2ds[7]
    conv2d_3_2 = conv2ds[7]
    conv2d_4_0 = conv2ds[8]
    conv2d_4_1 = conv2ds[8]
    conv2d_4_2 = conv2ds[8]
    maxpoolings = benchmarks[ "maxpooling" ]
    maxpooling_0 = maxpoolings[0]
    maxpooling_1 = maxpoolings[1]
    maxpooling_2 = maxpoolings[2]
    maxpooling_3 = maxpoolings[3]
    maxpooling_4 = maxpoolings[4]
    benchparams.append( ("conv2d", conv2d_0_0) )
    benchparams.append( ("conv2d", conv2d_0_1) )
    benchparams.append( ("maxpooling", maxpooling_0) )
    benchparams.append( ("conv2d", conv2d_1_0) )
    benchparams.append( ("conv2d", conv2d_1_1) )
    benchparams.append( ("conv2d", conv2d_1_2) )
    benchparams.append( ("maxpooling", maxpooling_1) )
    benchparams.append( ("conv2d", conv2d_2_0) )
    benchparams.append( ("conv2d", conv2d_2_1) )
    benchparams.append( ("conv2d", conv2d_2_2) )
    benchparams.append( ("maxpooling", maxpooling_2) )
    benchparams.append( ("conv2d", conv2d_3_0) )
    benchparams.append( ("conv2d", conv2d_3_1) )
    benchparams.append( ("conv2d", conv2d_3_2) )
    benchparams.append( ("maxpooling", maxpooling_3) )
    benchparams.append( ("conv2d", conv2d_4_0) )
    benchparams.append( ("conv2d", conv2d_4_1) )
    benchparams.append( ("conv2d", conv2d_4_2) )
    benchparams.append( ("maxpooling", maxpooling_4) )
    denselayers = benchmarks["fulllayer"]
    benchparams.append( ("fulllayer", denselayers[0]) )
    benchparams.append( ("fulllayer", denselayers[1]) )
    for bench,bench_cmd in benchparams:
        if args.mode[0] == "full":
            run_bench_with_sampled( bench,bench_cmd,args.mode[0] )
        elif args.mode[0] in ["sampled" ,"kernelsampled" ]:
            run_bench_with_sampled( bench,bench_cmd,args.mode[0] )
else:
    for bench,bench_cmds in benchmarks.items():
    #    if bench not in benchmarks_test:
    #        continue
        if args.bench != "all":
            if args.bench != bench:
                continue
        if type(bench_cmds) != list:
            bench_cmds = [bench_cmds]
        for bench_cmd in bench_cmds:
            run_bench_with_param( bench,bench_cmd )
#                    print(cmd)
#                    os.system(cmd)

#                    oriname = results_name[pattern]
#                    cmd1 = "mv %(oriname)s %(final_name)s"%locals()
#                    print(cmd1) 
#                    os.system(cmd1)
        
def run_command(command):
    command.RunCommand()
if not args.check:
    n_thread = args.n
    from multiprocessing import Pool
    with Pool(n_thread) as pool:
        pool.map(run_command,allcommands)
#    for command in allcommands:
#        run_command(command)
        
print(cluster2simtime)
for kernel,times in cluster2simtime.items():
    timesstr = [str(elem)for elem in times]
    print("\t".join(timesstr))

