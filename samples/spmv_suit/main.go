package main

import (
	"flag"
    "gitlab.com/akita/mgpusim/v3/benchmarks/shoc/spmv_suit"
    //"gitlab.com/akita/mgpusim/v2/benchmarks/shoc/spmv"
//	"../../benchmarks/shoc/spmv_suit"
    "gitlab.com/akita/mgpusim/v3/samples/runner"
)

// Dim is dimension
//var Dim = flag.Int("dim", 128, "The number of rows in the input matrix.")

// Sparsity is sparsity
//var Sparsity = flag.Float64("sparsity", 0.01,
//	"The ratio between non-zero elements to all the elelements in the matrix")

var file_location = flag.String("matrix", "","the location of matrix")
func main() {
	flag.Parse()

	runner := new(runner.Runner).ParseFlag().Init()

	benchmark := spmv_suit.NewBenchmark(runner.Driver())
//	benchmark.Dim = int32(*Dim)
//	benchmark.Sparsity = *Sparsity
    benchmark.Sparsity = *file_location
//    benchmark2.Dim2 = "d"
//    benchmark2.matrixLocation2 = 1
    //benchmark2.Sparsity = ""
	runner.AddBenchmark(benchmark)
	runner.Run()
}
