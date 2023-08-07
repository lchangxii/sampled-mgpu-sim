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
pattern_order = ["full","mixedsampled","wgsampled","analysis"]
pattern_printorder = ["full","kernelSampled","warpKernelSampled","photon"]
mode2execute = set()
def get_args():
    parser = argparse.ArgumentParser(description="Executing All Benchmarks")

    parser.add_argument("--check" ,action="store_true" , default=False,help=" check the final result")
    parser.add_argument("--force" ,action="store_true" , default=False,help=" force to execute")
    parser.add_argument("--mode",type=str   ,nargs='+', default="all",help=" execution modes, including photon, kernelSampled, warpKernelSampled,full ")
    parser.add_argument("--bench",type=str   , default="vgg16",help=" benchmarks to execute, support vgg16,vgg19,resnet18,resnet32,resnet50,resnet101,resnet152")
    parser.add_argument("--arch",type=str   , default="r9nano",help="archtecture to simulate")
    parser.add_argument("--v",type=str   , default="0",help="version")
    parser.add_argument("--n",type=int   , default=16,help="parallel workloads")
    args = parser.parse_args()
    if args.mode[0] == "all":
        for pattern in pattern_printorder:
            mode2execute.add(pattern)
    else:
        for pattern in args.mode:
            if pattern == "full":
                mode2execute.add(pattern)
            elif  pattern == "photon":
                mode2execute.add("mixedsampled")
                mode2execute.add("analysis")
            elif pattern == "warpKernelSampled":
                mode2execute.add("wgsampled")
                mode2execute.add("analysis")
            elif pattern == "kernelSampled":
                mode2execute.add("full")
                mode2execute.add("analysis") 
            elif pattern == "analysis":
                mode2execute.add("analysis") 

    return args

args = get_args()

from vgg16config import init_vgg16, run_vgg16
from vgg19config import init_vgg19, run_vgg19
from resnet18config import init_resnet18, run_resnet18
from resnet34config import init_resnet34, run_resnet34
from resnet50config import init_resnet50_101_152,run_resnet50,run_resnet101,run_resnet152
if args.bench=="vgg16":
    benchmarks = init_vgg16()
elif args.bench=="vgg19":
    benchmarks = init_vgg19()
elif args.bench=="resnet18":
    benchmarks = init_resnet18()
elif args.bench=="resnet34":
    benchmarks = init_resnet34()
elif args.bench=="resnet50" or args.bench=="resnet101" or args.bench=="resnet152":
    benchmarks = init_resnet50_101_152()
else:
    print("Unknow benchmarks, we only support vgg16/19 and resnet18/34/50/101/152 now")
    print("If you want PageRank, run testpagerank.py script")
    exit(0)
general_parameter=["-magic-memory-copy"]



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
simtimesum = dict()
walltimesum = dict()
patternvecs = []
patternset = set()
def run_bench_with_param( bench,bench_cmd ,bench_i):
    binary_dir=os.path.join(root_path,bench)
    os.chdir( binary_dir )
    if not args.check:
        os.system("go build")

    #output_each_bench = [bench_cmd]
    output_each_bench = []


    for pattern in pattern_order:
        if pattern not in mode2execute:
            continue
        if pattern in pattern_printorder and pattern not in patternset:
            patternvecs.append(pattern)
            patternset.add(pattern)
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
                if pattern == "full":
                    output_each_bench.append( str(simtime))
                    output_each_bench.append( str(walltime))
                    if pattern not in simtimesum:
                        simtimesum[pattern] = simtime
                        walltimesum[pattern] = walltime
                    else:
                        simtimesum[ pattern ] += simtime
                        walltimesum[ pattern ] += walltime
            else:
                isExisting = os.path.exists( os.path.join(result_dir,final_name) )
                #print(isExisting)
                #if not isExisting:
                 #   exit(1)
                if (not isExisting) or args.force:
                    run_cmd(cmd,results_name[pattern],final_name,binary_dir)
            if len(output_all) < bench_i:
                output_all.append(output_each_bench)
            else:
                output_all[bench_i] += output_each_bench
from sampledanalysis import ExecuteEngine
execute_engine = ExecuteEngine()
def run_bench_with_sampled( bench,bench_cmd,pattern,bench_i ):
    binary_dir=os.path.join(root_path,bench)
    os.chdir( binary_dir )

    pattern_parameter = pattern_parameters_dict["analysis"][0]
    analysis_name,_ = add_bench( bench, bench_cmd,"",pattern_parameter,"analysis", args.v )
    analysis_name = os.path.join(result_dir, analysis_name)
#    print(analysis_name)
    kernels,walltimeanalysis = execute_engine.decompose_data( analysis_name ) 
    walltime = 0
    simtime = 0
    if pattern  in pattern_printorder and pattern not in patternset:
        patternvecs.append(pattern)
        patternset.add(pattern)
    pattern_in = pattern
    if pattern == "kernelSampled":
        pattern = "full"
    elif pattern == "photon":
        pattern = "mixedsampled"
    elif pattern == "warpKernelSampled":
        pattern = "wgsampled"
#    print("test",bench,bench_cmd)
    
    if "vgg"in args.bench and (bench == "fulllayer" and "25088" in bench_cmd) and pattern!="analysis":
        pattern = "full"
    
    pattern_parameter = pattern_parameters_dict[pattern][0]
    if pattern != "analysis":
        full_name,_ = add_bench( bench, bench_cmd,"",pattern_parameter,pattern,args.v )

        full_name = os.path.join(result_dir, full_name)

        kernel_fulltimes,kernel_walltimes = execute_engine.decompose_full_data( full_name )
    else:
        kernel_fulltimes = [0] * len(walltimeanalysis)
        kernel_walltimes = [0] * len(walltimeanalysis)
    sampled = []
    walltime = 0
    for idx,kernel in enumerate( kernels ):
#        print("inner kernel %d"%idx)
        predtime, success = execute_engine.find_similar_kernel(kernel) 
        walltime += walltimeanalysis[idx]
        if success:
            simtime += predtime
        else:
            walltime += kernel_walltimes[idx]
            kerneltime = kernel_fulltimes[idx] 
            simtime += kerneltime
            kernel.updatesim( kerneltime)
            execute_engine.update(  kernel )
        sampled.append(str(success))
    output= [simtime,walltime]

    if pattern_in not in simtimesum.keys():
        simtimesum[pattern_in] = simtime
        walltimesum[pattern_in] = walltime
    else:
        simtimesum[ pattern_in ] += simtime
        walltimesum[ pattern_in ] += walltime

    output = [str(elem)for elem in output]
    #output+=sampled + [str(elem) for elem in kernel_fulltimes ]
    if len(output_all) <= bench_i:
        output_all.append(output)
    else:
        output_all[bench_i] += output

    #output_all.append(output )
    return simtime,walltime

if args.bench=="vgg16":
    benchparams = run_vgg16(benchmarks)
elif args.bench=="vgg19":
    benchparams = run_vgg19(benchmarks)
elif args.bench=="resnet18":
    benchparams = run_resnet18(benchmarks)
elif args.bench=="resnet34":
    benchparams = run_resnet34(benchmarks)
elif args.bench=="resnet50":
    benchparams = run_resnet50(benchmarks)
elif args.bench=="resnet101":
    benchparams = run_resnet101(benchmarks)
elif args.bench=="resnet152":
    benchparams = run_resnet152(benchmarks)
bench_i = 0
for bench,bench_cmd in benchparams:
    if args.mode[0] == "all" :
        if args.bench=="vgg16":
            allmode = pattern_printorder
        else:
            allmode = ["photon","full"]
    else:
        allmode = args.mode
    if args.check:
        for mode in allmode:
            if mode == "full":
                kernelnum = run_bench_with_param( bench,bench_cmd,bench_i )
            elif mode in pattern_printorder:
            #    print("WWWWWWWWWWWWWWWW",mode)
                kernelnum = run_bench_with_sampled( bench,bench_cmd, mode,bench_i )
        bench_i +=1
    else:
        run_bench_with_param(bench,bench_cmd)
   
def run_command(command):
    command.RunCommand()

if not args.check:
    n_thread = args.n
    from multiprocessing import Pool
    with Pool(n_thread) as pool:
        pool.map(run_command,allcommands)
#    for command in allcommands:
#        run_command(command)

first_row = ["layers"]
last_row = ["Sum"]
print(patternvecs)
for pattern in patternvecs:
    if pattern == "full":
        first_row += [ "MGPUSim-Simtime","MGPUSim-Walltime" ]
    elif pattern == "kernelSampled":
        first_row += [ "Kernel-Simtime","Kernel-Walltime" ]
    elif pattern == "photon":
        first_row += [ "Photon-Simtime","Photon-Walltime" ]
    elif pattern == "warpKernelSampled":
        first_row += [ "Warp+Kernel-Simtime","Warp+Kernel-Walltime" ]
    simtime = simtimesum[pattern]
    walltime = walltimesum[pattern]
    last_row += [str(simtime),str(walltime)]

print(first_row)

print("\n#########Final Results\n")
print("\t".join(first_row))
for i, output_each_bench in enumerate( output_all ):
    print( "layer%d\t"%i + "\t".join(output_each_bench) )

print("\t".join(last_row))

