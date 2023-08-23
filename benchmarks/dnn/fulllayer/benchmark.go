// Package conv2d defines a benchmark for the Convolutional Layer.
package fulllayer

import (
	"gitlab.com/akita/dnn/layers"
	"gitlab.com/akita/dnn/tensor"
	gpuTensor "gitlab.com/akita/mgpusim/v3/benchmarks/dnn/tensor"
	"gitlab.com/akita/mgpusim/v3/driver"
)

// A Benchmark is a benchmark for the Convolutional Layer.
type Benchmark struct {
	driver           *driver.Driver
	context          *driver.Context
	gpus             []int
	useUnifiedMemory bool

	N, Inputdim,Outputdim                    int
	EnableBackward                           bool

	layer    *layers.FullyConnectedLayer
	operator *gpuTensor.GPUOperator

	forwardIn  tensor.Tensor
	backwardIn tensor.Tensor
}

// NewBenchmark creates a new Avg Pooling benchmark. It requires the GPU driver as an argument.
func NewBenchmark(driver *driver.Driver) *Benchmark {
	b := &Benchmark{
		driver: driver,
	}

	b.context = b.driver.Init()
	b.operator = gpuTensor.NewGPUOperator(b.driver, b.context)
	b.operator.ReportTime()

	return b
}

// EnableVerification configures the benchmark to verify the result.
func (b *Benchmark) EnableVerification() {
	b.operator.EnableVerification()
}

// SelectGPU selects the GPU to run the benchmark on.
func (b *Benchmark) SelectGPU(gpus []int) {
	if len(gpus) > 1 {
		panic("Conv2D benchmark can only run on a single GPU for now.")
	}

	b.gpus = gpus
}

// SetUnifiedMemory configures the benchmark to use unified memory.
func (b *Benchmark) SetUnifiedMemory() {
	b.useUnifiedMemory = true
}

// Run runs the benchmark.
func (b *Benchmark) Run() {
	b.driver.SelectGPU(b.context, b.gpus[0])
	b.initMem()
	b.exec()
}


func (b *Benchmark) initMem() {
	b.layer = layers.NewFullyConnectedLayer(
		b.operator,
        b.Inputdim,
        b.Outputdim,
	)
	b.layer.Randomize()

	b.forwardIn = b.operator.Zeros([]int{b.N, b.Inputdim})

	if b.EnableBackward {
		b.backwardIn = b.operator.Zeros(
			[]int{b.N, b.Outputdim})
	}
}

func (b *Benchmark) exec() {
	b.layer.Forward(b.forwardIn)

	if b.EnableBackward {
		b.layer.Backward(b.backwardIn)
	}
}

// Verify does nothing for now.
func (b *Benchmark) Verify() {
}