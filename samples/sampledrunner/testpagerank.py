#!/usr/bin/env python3.8
import argparse
import numpy as np
benchmarks_test=["matrixmultiplication"]
#patterns_test=["matrixmultiplication"]
import os

from sampledanalysis import ExecuteEngine
execute_engine = ExecuteEngine()

def run_bench_with_sampled():
    analysis_name = "bbv_feature.json"
    kernels,walltimeanalysis = execute_engine.decompose_data( analysis_name )
    firstbbv = kernels[0].gpubbv
    for kernel_i in range(1,len(kernels)) :
        kernel = kernels[kernel_i]
        anotherbbv = kernel.gpubbv
        dis = firstbbv.dis(anotherbbv)
        print(dis)

run_bench_with_sampled()

