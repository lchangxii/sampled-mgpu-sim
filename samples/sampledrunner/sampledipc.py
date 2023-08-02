#!/usr/bin/env python2
import os, sys

import json
# class to hold representative stats

import argparse
def get_args():
    parser = argparse.ArgumentParser(description="Analysing data of sampled ipc method")
    
    parser.add_argument("--path", type=str, default="./", help="basic path")
    parser.add_argument("--profile", type=str, default="insnums.json", help="insnums")
    parser.add_argument("--sampled", type=str, default="ipc_sampled_result.json", help="sampled simulation data")
    args = parser.parse_args()
    return args
def processjson(path):
    with open( path ) as f:
        json_raw = f.read()
        record = json.loads(json_raw)
    return record

def extrapolate(profile_data , sampled_data):
    profile_data = processjson(profile_data)
    sampled_data = processjson(sampled_data)
    start_cycle = sampled_data["start_cycle"]
    end_cycle = sampled_data["end_cycle"]
    freq = sampled_data["freq"]
    avg_ipc = sampled_data["IPC"]
    prior_insts = sampled_data["insts_sum"]
    allinsts = profile_data["insts_num"]
    if (allinsts - prior_insts) == 0:
        extrapolate_cycles = 0
    else:
        if avg_ipc == 0:
            extrapolate_cycles = 0
        else:
            extrapolate_cycles = (allinsts - prior_insts) / avg_ipc
    final_cycles = end_cycle - start_cycle + extrapolate_cycles
    kernel_time = final_cycles * freq
    print(kernel_time)
    return kernel_time,sampled_data["walltime"]
if __name__ == "__main__":
    args = get_args()
    sampled_data = os.path.join( args.path,args.sampled)
    profile_data = os.path.join( args.path,args.profile )
    extrapolate(profile_data,sampled_data)

