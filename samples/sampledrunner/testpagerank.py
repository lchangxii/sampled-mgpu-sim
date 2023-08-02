#!/usr/bin/env python3.8
import argparse
import numpy as np
benchmarks_test=["matrixmultiplication"]
#patterns_test=["matrixmultiplication"]
import os

from sampledanalysis import ExecuteEngine
from testengine import RunBench,add_bench,results_name,remove_special_charactor
execute_engine = ExecuteEngine()
benchmarks = dict()
def run_bench_with_sampled(analysis_name):
    kernels,walltimeanalysis = execute_engine.decompose_data( analysis_name )
    firstbbv = kernels[0].gpubbv
    for kernel_i in range(1,len(kernels)) :
        kernel = kernels[kernel_i]
        anotherbbv = kernel.gpubbv
        dis = firstbbv.dis(anotherbbv)
        if not dis[1]:
            print( "Assumption Wrong" )
            exit(1)
    return len(kernels)
benchmarks["pagerank"] = []

#pagerankparams=[8192,16384,32768,65536]

#pagerankparams=extend(pagerankparams)
pagerankparams = [8192]
for param in pagerankparams:
    benchmarks["pagerank"].append("./pagerank -node %d "%param)

general_parameter=["-magic-memory-copy"]
allcommands = []

pattern_parameters_dict = dict()
pattern_parameters_dict["mixedsampled"] = ["-timing -branch-sampled -sampled -iterations 1"]

pattern_parameters_dict["full"] = ["-timing"]
pattern_parameters_dict["analysis"] = ["-only-analysis"]

def get_args():
    parser = argparse.ArgumentParser(description="Executing All Benchmarks")

    parser.add_argument("--check" ,action="store_true" , default=False,help=" check the final result")
    parser.add_argument("--force" ,action="store_true" , default=False,help=" force to execute")
    parser.add_argument("--mode",type=str   ,nargs='+', default="all",help=" execution mode; photon or full ")
    parser.add_argument("--bench",type=str   , default="pagerank",help=" benchmarks to execute")
    parser.add_argument("--arch",type=str   , default="r9nano",help="archtecture to simulate")
    parser.add_argument("--v",type=str   , default="0",help="version")
    parser.add_argument("--n",type=int   , default=8,help="core number")
    args = parser.parse_args()
    return args

args = get_args()


def run_cmd(command, result_name,final_name,binary_dir):
    allcommands.append( RunBench(command,result_name,final_name,binary_dir) )

from sampledanalysis import bbvanalysis
pattern_order = ["full","photon"]
import os
cwd=os.getcwd()
root_path = os.path.join(cwd,"..")
home_dir = os.getenv('HOME')


output_all = []
#home_dir = Path.home()
result_dir=os.path.join(home_dir,"gpudata")
from my_utils import check_dir
check_dir(result_dir)
from sampledipc import processjson,extrapolate
def check_full_data( file_name ):
    data = processjson( file_name )
    return data["simtime"],data["walltime"]
def check_sampled_data(file_name):
    data = processjson( file_name )
    return data["simtime"],data["walltime"]
def check_analysis_data(file_name):
    data = processjson( file_name )
    return sum(data["analysistimes"])

def check_data( file_name ,pattern,inscountfinal_name):

    try:
        #print(file_name)
        if pattern == "full":
            simtime,walltime = check_full_data(file_name)
        elif pattern in[ "wgsampled","mixedsampled","branchsampled"]:
            simtime,walltime = check_sampled_data(file_name)
        elif pattern == "analysis":
            walltime= check_analysis_data( file_name)
            simtime = run_bench_with_sampled(file_name) ##simtime is the kernel number for pagerank

        else:
            simtime,walltime=0,0
    except FileNotFoundError:
        simtime,walltime=0,0

    return simtime,walltime 

benchid = -1
first_row = []
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
        output_each_bench = [ "\""+ bench_cmd+"\"" ]
        for pattern in pattern_order:
            if args.mode[0] != "all":
                if pattern not in args.mode :
                    continue
            if args.check:
                ###check results
                
                if pattern == "photon":
                    if benchid == 0:
                        first_row += [ "Photon-Simtime","Photon-Walltime" ]
                    pattern = "mixedsampled"
                    pattern_parameter = pattern_parameters_dict[pattern][0]
                    
                    final_name,cmd = add_bench( bench, bench_cmd,"",pattern_parameter,pattern )
                    final_name = os.path.join(result_dir,final_name)
                    simtime,walltime = check_data( final_name,pattern,None)

                    pattern = "analysis"
                    pattern_parameter = pattern_parameters_dict[pattern][0]
                    
                    final_name,cmd = add_bench( bench, bench_cmd,"",pattern_parameter,pattern )
                    final_name = os.path.join(result_dir,final_name)
                    kernelnum, walltime2 = check_data( final_name,pattern,None)
                    walltime += walltime2
                    simtime = simtime * kernelnum
                else:
                    if benchid == 0:
                        first_row += [ "MGPUSim-Simtime","MGPUSim-Walltime" ]

                    pattern_parameter = pattern_parameters_dict[pattern][0]
                    
                    final_name,cmd = add_bench( bench, bench_cmd,"",pattern_parameter,pattern )
                    final_name = os.path.join(result_dir,final_name)
                    simtime,walltime = check_data( final_name,pattern,None)
                output_each_bench.append( str(simtime))
                output_each_bench.append( str(walltime))


            else:
                if pattern == "full":
                    patterns = [pattern]
                else:
                    patterns = ["mixedsampled","analysis"]
                for pattern in patterns:
                    for pattern_parameter in pattern_parameters_dict[pattern]:
                        final_name,cmd = add_bench( bench, bench_cmd,"",pattern_parameter,pattern )
                        isExisting = os.path.exists( os.path.join(result_dir,final_name) )
                        final_name = os.path.join(result_dir,final_name)
                        if (not isExisting) or args.force:
                            run_cmd(cmd, results_name[pattern],final_name,binary_dir)

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
first_row = ["benchmark-command"] + first_row
print(first_row)
print("\t".join(first_row))
for output_each_bench in output_all:
    print( "\t".join(output_each_bench) )


