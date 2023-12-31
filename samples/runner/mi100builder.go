package runner

import (
	"fmt"
	//"log"

	rob2 "gitlab.com/akita/mgpusim/v3/timing/rob"

	"gitlab.com/akita/akita/v3/monitoring"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/sim/bottleneckanalysis"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/cache/writearound"
	"gitlab.com/akita/mem/v3/cache/writeback"
	"gitlab.com/akita/mem/v3/cache/writethrough"
	"gitlab.com/akita/mem/v3/dram"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/addresstranslator"
	"gitlab.com/akita/mem/v3/vm/mmu"
	"gitlab.com/akita/mem/v3/vm/tlb"
	"gitlab.com/akita/mgpusim/v3/timing/cp"
	"gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mgpusim/v3/timing/cu"
	"gitlab.com/akita/mgpusim/v3/timing/pagemigrationcontroller"
	"gitlab.com/akita/mgpusim/v3/timing/rdma"
    //"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcu"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcache"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualtlb"
	//"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"

)

// MI100GPUBuilder can build R9 Nano GPUs.
type MI100GPUBuilder struct {
	engine                         sim.Engine
	freq                           sim.Freq
	memAddrOffset                  uint64
	mmu                            *mmu.MMU
	mmuVirtualMMU                  * virtualtlb.VirtualMMU
	numShaderArray                 int
	numCUPerShaderArray            int
	numMemoryBank                  int
	dramSize                       uint64
	l2CacheSize                    uint64
	log2PageSize                   uint64
	log2CacheLineSize              uint64
	log2MemoryBankInterleavingSize uint64

	enableISADebugging bool
	enableMemTracing   bool
	enableVisTracing   bool
	visTracer          tracing.Tracer
	memTracer          tracing.Tracer
	monitor            *monitoring.Monitor
	bufferAnalyzer     *bottleneckanalysis.BufferAnalyzer

	gpuName                 string
	gpu                     *GPU
	gpuID                   uint64
	cp                      *cp.CommandProcessor
	cus                     []*cu.ComputeUnit
	l1vReorderBuffers       []*rob2.ReorderBuffer
	l1iReorderBuffers       []*rob2.ReorderBuffer
	l1sReorderBuffers       []*rob2.ReorderBuffer
	l1vCaches               []*writearound.Cache
	l1sCaches               []*writethrough.Cache
	l1iCaches               []*writethrough.Cache
	l1vVirtualCaches [] virtualcache.VirtualCache
    l1iVirtualCaches [] virtualcache.VirtualCache
	l1sVirtualCaches [] virtualcache.VirtualCache
	l2Caches                []*writeback.Cache
	l2VirtualCaches         []virtualcache.VirtualCache
	l1vAddrTrans            []*addresstranslator.AddressTranslator
	l1sAddrTrans            []*addresstranslator.AddressTranslator
	l1iAddrTrans            []*addresstranslator.AddressTranslator
	l1vTLBs                 []*tlb.TLB
	l1sTLBs                 []*tlb.TLB
	l1iTLBs                 []*tlb.TLB
	l1vVirtualTLBs []* virtualtlb.VirtualTLB
	l1sVirtualTLBs []* virtualtlb.VirtualTLB
	l1iVirtualTLBs []* virtualtlb.VirtualTLB
	l2TLBs                  []*tlb.TLB
	l2VirtualTLBs           []* virtualtlb.VirtualTLB
	drams                   []*dram.MemController
	dramVirtualDrams        []*virtualcache.VirtualDRAM
	lowModuleFinderForL1    *mem.InterleavedLowModuleFinder
	lowModuleFinderForL2    *mem.InterleavedLowModuleFinder
	lowModuleFinderForPMC   *mem.InterleavedLowModuleFinder
	dmaEngine               *cp.DMAEngine
	rdmaEngine              *rdma.Engine
	pageMigrationController *pagemigrationcontroller.PageMigrationController
	globalStorage           *mem.Storage

	internalConn           *sim.DirectConnection
	l1TLBToL2TLBConnection *sim.DirectConnection
	l1ToL2Connection       *sim.DirectConnection
	l2ToDramConnection     *sim.DirectConnection
}


// MakeMI100GPUBuilder provides a GPU builder that can builds the R9Nano GPU.
func MakeMI100GPUBuilder() MI100GPUBuilder {
	b := MI100GPUBuilder{
		freq:                           1 * sim.GHz,
		numShaderArray:                 30,
		numCUPerShaderArray:            4,
		numMemoryBank:                  16,
		log2CacheLineSize:              6,
		log2PageSize:                   12,
		log2MemoryBankInterleavingSize: 12,
		l2CacheSize:                    8 * mem.MB,
		dramSize:                       32 * mem.GB,
	}

    fmt.Printf("freqs : %f\n", b.freq)
	return b
}

// WithEngine sets the engine that the GPU use.
func (b MI100GPUBuilder) WithEngine(engine sim.Engine) MI100GPUBuilder {
	b.engine = engine
	return b
}

// WithFreq sets the frequency that the GPU works at.
func (b MI100GPUBuilder) WithFreq(freq sim.Freq) MI100GPUBuilder {
	b.freq = freq
	return b
}

// WithMemAddrOffset sets the address of the first byte of the GPU to build.
func (b MI100GPUBuilder) WithMemAddrOffset(
	offset uint64,
) MI100GPUBuilder {
	b.memAddrOffset = offset
	return b
}

// WithMMU sets the MMU component that provides the address translation service
// for the GPU.
func (b MI100GPUBuilder) WithMMU(mmu *mmu.MMU) MI100GPUBuilder {
	b.mmu = mmu
    ///////////mmu can also be seen as bottom tlb
    virtualtlbbuilder := virtualtlb.MakeBuilder().
                            WithNumSets(1).
                            WithNumWays(64).
                            WithEngine(b.engine).
                            WithFreq(b.freq).
                            WithLatency(818)
    name_tmp := fmt.Sprintf("%s.Virtual",mmu.Name() )
    b.mmuVirtualMMU = virtualtlbbuilder.BuildMMU(name_tmp)
    b.mmuVirtualMMU.SetRealComponent( mmu )
    
	return b
}

// WithNumMemoryBank sets the number of L2 cache modules and number of memory
// controllers in each GPU.
func (b MI100GPUBuilder) WithNumMemoryBank(n int) MI100GPUBuilder {
	b.numMemoryBank = n
	return b
}

// WithNumShaderArray sets the number of shader arrays in each GPU. Each shader
// array contains a certain number of CUs, a certain number of L1V caches, 1
// L1S cache, and 1 L1V cache.
func (b MI100GPUBuilder) WithNumShaderArray(n int) MI100GPUBuilder {
	b.numShaderArray = n
	return b
}

// WithNumCUPerShaderArray sets the number of CU and number of L1V caches in
// each Shader Array.
func (b MI100GPUBuilder) WithNumCUPerShaderArray(n int) MI100GPUBuilder {
	b.numCUPerShaderArray = n
	return b
}

// WithLog2MemoryBankInterleavingSize sets the number of consecutive bytes that
// are guaranteed to be on a memory bank.
func (b MI100GPUBuilder) WithLog2MemoryBankInterleavingSize(
	n uint64,
) MI100GPUBuilder {
	b.log2MemoryBankInterleavingSize = n
	return b
}

// WithVisTracer applies a tracer to trace all the tasks of all the GPU
// components
func (b MI100GPUBuilder) WithVisTracer(t tracing.Tracer) MI100GPUBuilder {
	b.enableVisTracing = true
	b.visTracer = t
	return b
}

// WithMemTracer applies a tracer to trace the memory transactions.
func (b MI100GPUBuilder) WithMemTracer(t tracing.Tracer) MI100GPUBuilder {
	b.enableMemTracing = true
	b.memTracer = t
	return b
}

// WithISADebugging enables the GPU to dump instruction execution information.
func (b MI100GPUBuilder) WithISADebugging() MI100GPUBuilder {
	b.enableISADebugging = true
	return b
}

// WithLog2CacheLineSize sets the cache line size with the power of 2.
func (b MI100GPUBuilder) WithLog2CacheLineSize(
	log2CacheLine uint64,
) MI100GPUBuilder {
	b.log2CacheLineSize = log2CacheLine
	return b
}

// WithLog2PageSize sets the page size with the power of 2.
func (b MI100GPUBuilder) WithLog2PageSize(log2PageSize uint64) MI100GPUBuilder {
	b.log2PageSize = log2PageSize
	return b
}

// WithMonitor sets the monitor to use.
func (b MI100GPUBuilder) WithMonitor(m *monitoring.Monitor) MI100GPUBuilder {
	b.monitor = m
	return b
}

// WithBufferAnalyzer sets the buffer analyzer to use.
func (b MI100GPUBuilder) WithBufferAnalyzer(
	a *bottleneckanalysis.BufferAnalyzer,
) MI100GPUBuilder {
	b.bufferAnalyzer = a
	return b
}

// WithL2CacheSize set the total L2 cache size. The size of the L2 cache is
// split between memory banks.
func (b MI100GPUBuilder) WithL2CacheSize(size uint64) MI100GPUBuilder {
	b.l2CacheSize = size
	return b
}

// WithDRAMSize sets the size of DRAMs in the GPU.
func (b MI100GPUBuilder) WithDRAMSize(size uint64) MI100GPUBuilder {
	b.dramSize = size
	return b
}

// WithGlobalStorage lets the GPU to build to use the externally provided
// storage.
func (b MI100GPUBuilder) WithGlobalStorage(
	storage *mem.Storage,
) MI100GPUBuilder {
	b.globalStorage = storage
	return b
}

// Build creates a pre-configure GPU similar to the AMD R9 Nano GPU.
func (b MI100GPUBuilder) Build(name string, id uint64) *GPU {
	b.createGPU(name, id)
	b.buildSAs()
	b.buildL2Caches()
	b.buildDRAMControllers()
	b.buildCP()
	b.buildL2TLB()

	b.connectCP()
	b.connectL2AndDRAM()
	b.connectL1ToL2()
	b.connectL1TLBToL2TLB()

	b.populateExternalPorts()

	return b.gpu
}

func (b *MI100GPUBuilder) populateExternalPorts() {
	b.gpu.Domain.AddPort("CommandProcessor", b.cp.ToDriver)
	b.gpu.Domain.AddPort("RDMA", b.rdmaEngine.ToOutside)
	b.gpu.Domain.AddPort("PageMigrationController",
		b.pageMigrationController.GetPortByName("Remote"))

	for i, l2TLB := range b.l2TLBs {
		name := fmt.Sprintf("Translation_%02d", i)
		b.gpu.Domain.AddPort(name, l2TLB.GetPortByName("Bottom"))
	}
}

func (b *MI100GPUBuilder) createGPU(name string, id uint64) {
	b.gpuName = name

	b.gpu = &GPU{}
	b.gpu.Domain = sim.NewDomain(b.gpuName)
	b.gpuID = id
}

func (b *MI100GPUBuilder) connectCP() {
	b.internalConn = sim.NewDirectConnection(
		b.gpuName+"InternalConn", b.engine, b.freq)

	b.internalConn.PlugIn(b.cp.ToDriver, 1)
	b.internalConn.PlugIn(b.cp.ToDMA, 128)
	b.internalConn.PlugIn(b.cp.ToCaches, 128)
	b.internalConn.PlugIn(b.cp.ToCUs, 128)
	b.internalConn.PlugIn(b.cp.ToTLBs, 128)
	b.internalConn.PlugIn(b.cp.ToAddressTranslators, 128)
	b.internalConn.PlugIn(b.cp.ToRDMA, 4)
	b.internalConn.PlugIn(b.cp.ToPMC, 4)

	b.cp.RDMA = b.rdmaEngine.CtrlPort
	b.internalConn.PlugIn(b.cp.RDMA, 1)

	b.cp.DMAEngine = b.dmaEngine.ToCP
	b.internalConn.PlugIn(b.dmaEngine.ToCP, 1)

	pmcControlPort := b.pageMigrationController.GetPortByName("Control")
	b.cp.PMC = pmcControlPort
	b.internalConn.PlugIn(pmcControlPort, 1)

	b.connectCPWithCUs()
	b.connectCPWithAddressTranslators()
	b.connectCPWithTLBs()
	b.connectCPWithCaches()

}

func (b *MI100GPUBuilder) connectL1ToL2() {
	lowModuleFinder := mem.NewInterleavedLowModuleFinder(
		1 << b.log2MemoryBankInterleavingSize)
	lowModuleFinder.ModuleForOtherAddresses = b.rdmaEngine.ToL1
	lowModuleFinder.UseAddressSpaceLimitation = true
	lowModuleFinder.LowAddress = b.memAddrOffset
	lowModuleFinder.HighAddress = b.memAddrOffset + 32*mem.GB

	l1ToL2Conn := sim.NewDirectConnection(b.gpuName+".L1-L2",
		b.engine, b.freq)

	b.rdmaEngine.SetLocalModuleFinder(lowModuleFinder)
	l1ToL2Conn.PlugIn(b.rdmaEngine.ToL1, 64)
	l1ToL2Conn.PlugIn(b.rdmaEngine.ToL2, 64)

	for _, l2 := range b.l2Caches {
		lowModuleFinder.LowModules = append(lowModuleFinder.LowModules,
			l2.GetPortByName("Top"))
		l1ToL2Conn.PlugIn(l2.GetPortByName("Top"), 64)
	}

	for _, l1v := range b.l1vCaches {
		l1v.SetLowModuleFinder(lowModuleFinder)
		l1ToL2Conn.PlugIn(l1v.GetPortByName("Bottom"), 16)
	}

	for _, l1s := range b.l1sCaches {
		l1s.SetLowModuleFinder(lowModuleFinder)
		l1ToL2Conn.PlugIn(l1s.GetPortByName("Bottom"), 16)
	}

	for _, l1iAT := range b.l1iAddrTrans {
		l1iAT.SetLowModuleFinder(lowModuleFinder)
		l1ToL2Conn.PlugIn(l1iAT.GetPortByName("Bottom"), 16)
	}

    if profiler.SampledSimulation {
        for _,l1vvirtual := range b.l1vVirtualCaches {
//            l1vvirtual.SetBottomDevice( b.l2VirtualCaches )
            for _, elem := range(b.l2VirtualCaches) {
                l1vvirtual.AddBottomDevice(elem )
            }
        }
        for _,l1svirtual := range b.l1sVirtualCaches {
//            l1svirtual.SetBottomDevice( b.l2VirtualCaches )
            for _, elem := range(b.l2VirtualCaches) {
                l1svirtual.AddBottomDevice(elem )
            }

        }
        for _,l1ivirtual := range b.l1iVirtualCaches {
//            l1ivirtual.SetBottomDevice( b.l2VirtualCaches )
            for _, elem := range(b.l2VirtualCaches) {
                l1ivirtual.AddBottomDevice(elem )
            }

        }
    }

}

func (b *MI100GPUBuilder) connectL2AndDRAM() {
	b.l2ToDramConnection = sim.NewDirectConnection(
		b.gpuName+"L2-DRAM", b.engine, b.freq)

	lowModuleFinder := mem.NewInterleavedLowModuleFinder(
		1 << b.log2MemoryBankInterleavingSize)

	for i, l2 := range b.l2Caches {
		b.l2ToDramConnection.PlugIn(l2.GetPortByName("Bottom"), 64)
		l2.SetLowModuleFinder(&mem.SingleLowModuleFinder{
			LowModule: b.drams[i].GetPortByName("Top"),
		})
        if profiler.SampledSimulation {
            l2VirtualCache := b.l2VirtualCaches[i]
            bottomdevices := []*virtualcache.VirtualDRAM{ b.dramVirtualDrams[i]}
//            l2VirtualCache.SetBottomDevice( bottomdevices )
            for _, elem := range(bottomdevices) {
                l2VirtualCache.AddBottomDevice(elem )
            }

        }
	}

	for _, dram := range b.drams {
		b.l2ToDramConnection.PlugIn(dram.GetPortByName("Top"), 64)
		lowModuleFinder.LowModules = append(lowModuleFinder.LowModules,
			dram.GetPortByName("Top"))
	}

	b.dmaEngine.SetLocalDataSource(lowModuleFinder)
	b.l2ToDramConnection.PlugIn(b.dmaEngine.ToMem, 64)

	b.pageMigrationController.MemCtrlFinder = lowModuleFinder
	b.l2ToDramConnection.PlugIn(
		b.pageMigrationController.GetPortByName("LocalMem"), 16)
}

func (b *MI100GPUBuilder) connectL1TLBToL2TLB() {
	tlbConn := sim.NewDirectConnection(b.gpuName+"L1TLB-L2TLB",
		b.engine, b.freq)

	tlbConn.PlugIn(b.l2TLBs[0].GetPortByName("Top"), 64)

	for _, l1vTLB := range b.l1vTLBs {
		l1vTLB.LowModule = b.l2TLBs[0].GetPortByName("Top")
		tlbConn.PlugIn(l1vTLB.GetPortByName("Bottom"), 16)
	}

	for _, l1iTLB := range b.l1iTLBs {
		l1iTLB.LowModule = b.l2TLBs[0].GetPortByName("Top")
		tlbConn.PlugIn(l1iTLB.GetPortByName("Bottom"), 16)
	}

	for _, l1sTLB := range b.l1sTLBs {
		l1sTLB.LowModule = b.l2TLBs[0].GetPortByName("Top")
		tlbConn.PlugIn(l1sTLB.GetPortByName("Bottom"), 16)
	}
    if profiler.SampledSimulation {
        for _,l1vvirtualtlb := range b.l1vVirtualTLBs {
//            l1vvirtualtlb.SetBottomDevice( b.l2VirtualTLBs )
            for _, elem := range(b.l2VirtualTLBs) {
                l1vvirtualtlb.AddBottomDevice(elem )
            }

        }
        for _,l1svirtualtlb := range b.l1sVirtualTLBs {
//            l1svirtualtlb.SetBottomDevice( b.l2VirtualTLBs )
            for _, elem := range(b.l2VirtualTLBs) {
                l1svirtualtlb.AddBottomDevice(elem )
            }

        }
        for _,l1ivirtualtlb := range b.l1iVirtualTLBs {
//            l1ivirtualtlb.SetBottomDevice( b.l2VirtualTLBs )
            for _, elem := range(b.l2VirtualTLBs) {
                l1ivirtualtlb.AddBottomDevice(elem )
            }

        }
    }


}

func (b *MI100GPUBuilder) connectCPWithCUs() {
	for _, cu := range b.cus {
		b.cp.RegisterCU(cu)
		b.internalConn.PlugIn(cu.ToACE, 1)
		b.internalConn.PlugIn(cu.ToCP, 1)
        
	}
}

func (b *MI100GPUBuilder) connectCPWithAddressTranslators() {
	for _, at := range b.l1vAddrTrans {
		ctrlPort := at.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, at := range b.l1sAddrTrans {
		ctrlPort := at.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, at := range b.l1iAddrTrans {
		ctrlPort := at.GetPortByName("Control")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, rob := range b.l1vReorderBuffers {
		ctrlPort := rob.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, rob := range b.l1iReorderBuffers {
		ctrlPort := rob.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, rob := range b.l1sReorderBuffers {
		ctrlPort := rob.GetPortByName("Control")
		b.cp.AddressTranslators = append(
			b.cp.AddressTranslators, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}
}

func (b *MI100GPUBuilder) connectCPWithTLBs() {
	for _, tlb := range b.l2TLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, tlb := range b.l1vTLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, tlb := range b.l1sTLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, tlb := range b.l1iTLBs {
		ctrlPort := tlb.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

}

func (b *MI100GPUBuilder) connectCPWithCaches() {
	for _, c := range b.l1iCaches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L1ICaches = append(b.cp.L1ICaches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, c := range b.l1vCaches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L1VCaches = append(b.cp.L1VCaches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, c := range b.l1sCaches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L1SCaches = append(b.cp.L1SCaches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}

	for _, c := range b.l2Caches {
		ctrlPort := c.GetPortByName("Control")
		b.cp.L2Caches = append(b.cp.L2Caches, ctrlPort)
		b.internalConn.PlugIn(ctrlPort, 1)
	}
}

func (b *MI100GPUBuilder) buildSAs() {
	saBuilder := makeShaderArrayBuilder().
		withEngine(b.engine).
		withFreq(b.freq).
		withGPUID(b.gpuID).
		withLog2CachelineSize(b.log2CacheLineSize).
		withLog2PageSize(b.log2PageSize).
		withNumCU(b.numCUPerShaderArray)

	if b.enableISADebugging {
		saBuilder = saBuilder.withIsaDebugging()
	}

	if b.enableVisTracing {
		saBuilder = saBuilder.withVisTracer(b.visTracer)
	}

	if b.enableMemTracing {
		saBuilder = saBuilder.withMemTracer(b.memTracer)
	}
    cunum := 0
	for i := 0; i < b.numShaderArray; i++ {
		saName := fmt.Sprintf("%s.SA_%02d", b.gpuName, i)
		cunum += b.buildSA(saBuilder, saName,i)
	}
    if * profiler.CollectDataApplication {
        profiler.Datafeature.SetCUNum( cunum )
    }
}

func (b *MI100GPUBuilder) buildL2Caches() {
	byteSize := b.l2CacheSize / uint64(b.numMemoryBank)
    const nummshrentry = 64
    const wayassociativity = 16
	l2Builder := writeback.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(wayassociativity).
		WithByteSize(byteSize).
		WithNumMSHREntry(nummshrentry).
		WithNumReqPerCycle(16)

    var virtualcachebuilder virtualcache.Builder
    if profiler.SampledSimulation {

	    blockSize := 1 << b.log2CacheLineSize
	    numSet := int( byteSize / uint64( wayassociativity*blockSize) )
        virtualcachebuilder = virtualcache.MakeBuilder().
                                        WithEngine(b.engine).
                                        WithFreq(b.freq).
                                        WithNumMSHREntry(nummshrentry).
                                        WithNumWays(wayassociativity).
		                                WithLog2BlockSize(b.log2CacheLineSize).
                                        WithNumSets(numSet).
                                        WithLatency( 13).
                                        WithPipelineStageNum( 2).
                                        WithSendPipelineStageNum( 1)
    }


	for i := 0; i < b.numMemoryBank; i++ {
		cacheName := fmt.Sprintf("%s.L2_%d", b.gpuName, i)
		l2 := l2Builder.WithInterleaving(
			1<<(b.log2MemoryBankInterleavingSize-b.log2CacheLineSize),
			b.numMemoryBank,
			i,
		).Build(cacheName)
		b.l2Caches = append(b.l2Caches, l2)
		b.gpu.L2Caches = append(b.gpu.L2Caches, l2)

        if profiler.SampledSimulation {

		    name_tmp := fmt.Sprintf("%s.Virtual", l2.Name() )
            virtualcache := virtualcachebuilder.BuildWriteback( name_tmp) 
            virtualcache.SetRealComponent(l2)

	        b.l2VirtualCaches = append(b.l2VirtualCaches,virtualcache)
        }


		if b.enableVisTracing {
			tracing.CollectTrace(l2, b.visTracer)
		}

		if b.enableMemTracing {
			tracing.CollectTrace(l2, b.memTracer)
		}

		if b.monitor != nil {
			b.monitor.RegisterComponent(l2)
		}
	}
}

func (b *MI100GPUBuilder) buildDRAMControllers() {
	memCtrlBuilder := b.createDramControllerBuilder()

    ///we can see dram as l3cache with long access time
    var virtualcachebuilder virtualcache.Builder
    if profiler.SampledSimulation {
        virtualcachebuilder = virtualcache.MakeBuilder().
                                    WithEngine(b.engine).
		                            WithFreq(500 * sim.MHz).
                                    WithLatency(16).
                                    WithPipelineStageNum(1).
                                    WithSendPipelineStageNum(0)

    }

	for i := 0; i < b.numMemoryBank; i++ {
		dramName := fmt.Sprintf("%s.DRAM_%d", b.gpuName, i)
		dram := memCtrlBuilder.
			Build(dramName)
		// dram := idealmemcontroller.New(
		// 	fmt.Sprintf("%s.DRAM_%d", b.gpuName, i),
		// 	b.engine, 512*mem.MB)
		b.drams = append(b.drams, dram)
        if profiler.SampledSimulation {
            name_tmp := fmt.Sprintf("%s.Virtual",dram.Name() )
            virtualcache := virtualcachebuilder.BuildDRAM( name_tmp )
            
            virtualcache.SetRealComponent(dram)
            virtualcache.SetConnectFreq(b.freq)
//            virtualcache.SetConnectSendPipelineStageNum(b.freq)
//            virtualcache.SetConnectPipelineStageNum(b.freq)

	        b.dramVirtualDrams = append(b.dramVirtualDrams,virtualcache)
        }

		b.gpu.MemControllers = append(b.gpu.MemControllers, dram)

		if b.enableVisTracing {
			tracing.CollectTrace(dram, b.visTracer)
		}

		if b.enableMemTracing {
			tracing.CollectTrace(dram, b.memTracer)
		}

		if b.monitor != nil {
			b.monitor.RegisterComponent(dram)
		}
	}
///for performance, every virtualcu can directly access virtualdram, so that they can flushout
     
    if profiler.SampledSimulation {
    	for _, cu := range b.cus {
            cu.Virtualcu.SetVirtualDramSet(b.dramVirtualDrams )

        }
    }
}

func (b *MI100GPUBuilder) createDramControllerBuilder() dram.Builder {
	memBankSize := 32 * mem.GB / uint64(b.numMemoryBank)
	if 32*mem.GB%uint64(b.numMemoryBank) != 0 {
		panic("GPU memory size is not a multiple of the number of memory banks")
	}

	dramCol := 64
	dramRow := 16384
	dramDeviceWidth := 128
	dramBankSize := dramCol * dramRow * dramDeviceWidth
	dramBank := 4
	dramBankGroup := 4
	dramBusWidth := 256
	dramDevicePerRank := dramBusWidth / dramDeviceWidth
	dramRankSize := dramBankSize * dramDevicePerRank * dramBank
	dramRank := int(memBankSize * 8 / uint64(dramRankSize))

	memCtrlBuilder := dram.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(500 * sim.MHz).
		WithProtocol(dram.HBM).
		WithBurstLength(4).
		WithDeviceWidth(dramDeviceWidth).
		WithBusWidth(dramBusWidth).
		WithNumChannel(1).
		WithNumRank(dramRank).
		WithNumBankGroup(dramBankGroup).
		WithNumBank(dramBank).
		WithNumCol(dramCol).
		WithNumRow(dramRow).
		WithCommandQueueSize(8).
		WithTransactionQueueSize(32).
		WithTCL(7).
		WithTCWL(2).
		WithTRCDRD(7).
		WithTRCDWR(7).
		WithTRP(7).
		WithTRAS(17).
		WithTREFI(1950).
		WithTRRDS(2).
		WithTRRDL(3).
		WithTWTRS(3).
		WithTWTRL(4).
		WithTWR(8).
		WithTCCDS(1).
		WithTCCDL(1).
		WithTRTRS(0).
		WithTRTP(3).
		WithTPPD(2)

	if b.visTracer != nil {
		memCtrlBuilder = memCtrlBuilder.WithAdditionalTracer(b.visTracer)
	}

	if b.globalStorage != nil {
		memCtrlBuilder = memCtrlBuilder.WithGlobalStorage(b.globalStorage)
	}

	return memCtrlBuilder
}

func (b *MI100GPUBuilder) buildSA(
	saBuilder shaderArrayBuilder,
	saName string,
    id int,
) int {
	sa := saBuilder.Build(saName)
    sa.Id = id
	b.populateCUs(&sa)
	b.populateROBs(&sa)
	b.populateTLBs(&sa)
	b.populateL1VAddressTranslators(&sa)
	b.populateL1Vs(&sa)
	b.populateScalerMemoryHierarchy(&sa)
	b.populateInstMemoryHierarchy(&sa)
    return len(sa.cus) 
}

func (b *MI100GPUBuilder) populateCUs(sa *shaderArray) {
	for i, cu := range sa.cus {
        cu.Id = sa.Id * len(sa.cus) + i
        //log.Printf("cuid: %d\n",cu.Id)
		b.cus = append(b.cus, cu)
		b.gpu.CUs = append(b.gpu.CUs, cu)

		if b.monitor != nil {
			b.monitor.RegisterComponent(cu)
		}

		if b.bufferAnalyzer != nil {
			b.bufferAnalyzer.AddComponent(cu)
		}
	}
	for _, cu := range sa.cus {
		for _, simd := range cu.SIMDUnit {
			b.gpu.SIMDs = append(b.gpu.SIMDs, simd.(TraceableComponent))
		}
	}	
}

func (b *MI100GPUBuilder) populateROBs(sa *shaderArray) {
	for _, rob := range sa.l1vROBs {
		b.l1vReorderBuffers = append(b.l1vReorderBuffers, rob)

		if b.monitor != nil {
			b.monitor.RegisterComponent(rob)
		}

		if b.bufferAnalyzer != nil {
			b.bufferAnalyzer.AddComponent(rob)
		}
	}
}

func (b *MI100GPUBuilder) populateTLBs(sa *shaderArray) {
	for idx, tlb := range sa.l1vTLBs {
		b.l1vTLBs = append(b.l1vTLBs, tlb)
		b.gpu.L1VTLBs = append(b.gpu.L1VTLBs, tlb)
        if profiler.SampledSimulation {
    		b.l1vVirtualTLBs = append(b.l1vVirtualTLBs, sa.l1vVirtualTLBs[idx])
        }

		if b.monitor != nil {
			b.monitor.RegisterComponent(tlb)
		}

		if b.bufferAnalyzer != nil {
			b.bufferAnalyzer.AddComponent(tlb)
		}
	}
}

func (b *MI100GPUBuilder) populateL1Vs(sa *shaderArray) {
	for idx, l1v := range sa.l1vCaches {
		b.l1vCaches = append(b.l1vCaches, l1v)
        if profiler.SampledSimulation {
    		b.l1vVirtualCaches = append(b.l1vVirtualCaches, sa.l1vVirtualCaches[idx])
        }
		b.gpu.L1VCaches = append(b.gpu.L1VCaches, l1v)
		if b.monitor != nil {
			b.monitor.RegisterComponent(l1v)
		}
	}
}

func (b *MI100GPUBuilder) populateL1VAddressTranslators(sa *shaderArray) {
	for _, at := range sa.l1vATs {
		b.l1vAddrTrans = append(b.l1vAddrTrans, at)

		if b.monitor != nil {
			b.monitor.RegisterComponent(at)
		}
	}
}

func (b *MI100GPUBuilder) populateScalerMemoryHierarchy(sa *shaderArray) {
	b.l1sAddrTrans = append(b.l1sAddrTrans, sa.l1sAT)
	b.l1sReorderBuffers = append(b.l1sReorderBuffers, sa.l1sROB)
	b.l1sCaches = append(b.l1sCaches, sa.l1sCache)
	b.l1sVirtualCaches = append(b.l1sVirtualCaches, sa.l1sVirtualCache)
	b.gpu.L1SCaches = append(b.gpu.L1SCaches, sa.l1sCache)
	b.l1sTLBs = append(b.l1sTLBs, sa.l1sTLB)
	b.l1sVirtualTLBs = append(b.l1sVirtualTLBs, sa.l1sVirtualTLB)
	b.gpu.L1STLBs = append(b.gpu.L1STLBs, sa.l1sTLB)

	if b.monitor != nil {
		b.monitor.RegisterComponent(sa.l1sAT)
		b.monitor.RegisterComponent(sa.l1sROB)
		b.monitor.RegisterComponent(sa.l1sCache)
		b.monitor.RegisterComponent(sa.l1sTLB)
	}
}

func (b *MI100GPUBuilder) populateInstMemoryHierarchy(sa *shaderArray) {
	b.l1iAddrTrans = append(b.l1iAddrTrans, sa.l1iAT)
	b.l1iReorderBuffers = append(b.l1iReorderBuffers, sa.l1iROB)
	b.l1iCaches = append(b.l1iCaches, sa.l1iCache)
	b.gpu.L1ICaches = append(b.gpu.L1ICaches, sa.l1iCache)
	b.l1iTLBs = append(b.l1iTLBs, sa.l1iTLB)
	b.l1iVirtualCaches = append(b.l1iVirtualCaches, sa.l1iVirtualCache)
	b.l1iVirtualTLBs = append(b.l1iVirtualTLBs, sa.l1iVirtualTLB)
	b.gpu.L1ITLBs = append(b.gpu.L1ITLBs, sa.l1iTLB)

	if b.monitor != nil {
		b.monitor.RegisterComponent(sa.l1iAT)
		b.monitor.RegisterComponent(sa.l1iROB)
		b.monitor.RegisterComponent(sa.l1iCache)
		b.monitor.RegisterComponent(sa.l1iTLB)
	}
}

func (b *MI100GPUBuilder) buildRDMAEngine() {
	b.rdmaEngine = rdma.NewEngine(
		fmt.Sprintf("%s.RDMA", b.gpuName),
		b.engine,
		b.lowModuleFinderForL1,
		nil,
	)
	b.gpu.RDMAEngine = b.rdmaEngine

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.rdmaEngine)
	}
}

func (b *MI100GPUBuilder) buildPageMigrationController() {
	b.pageMigrationController =
		pagemigrationcontroller.NewPageMigrationController(
			fmt.Sprintf("%s.PMC", b.gpuName),
			b.engine,
			b.lowModuleFinderForPMC,
			nil)
	b.gpu.PMC = b.pageMigrationController

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.pageMigrationController)
	}
}

func (b *MI100GPUBuilder) buildDMAEngine() {
	b.dmaEngine = cp.NewDMAEngine(
		fmt.Sprintf("%s.DMA", b.gpuName),
		b.engine,
		nil)

	if b.enableVisTracing {
		tracing.CollectTrace(b.dmaEngine, b.visTracer)
	}

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.dmaEngine)
	}
}

func (b *MI100GPUBuilder) buildCP() {
	builder := cp.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithMonitor(b.monitor).
		WithBufferAnalyzer(b.bufferAnalyzer)

	if b.enableVisTracing {
		builder = builder.WithVisTracer(b.visTracer)
	}

	b.cp = builder.Build(b.gpuName + ".CommandProcessor")
	b.gpu.CommandProcessor = b.cp

	if b.monitor != nil {
		b.monitor.RegisterComponent(b.cp)
	}

	b.buildDMAEngine()
	b.buildRDMAEngine()
	b.buildPageMigrationController()
}

func (b *MI100GPUBuilder) buildL2TLB() {
	numWays := 64
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumWays(numWays).
		WithNumSets(int(b.dramSize / (1 << b.log2PageSize) / uint64(numWays))).
		WithNumMSHREntry(64).
		WithNumReqPerCycle(1024).
		WithPageSize(1 << b.log2PageSize).
		WithLowModule(b.mmu.GetPortByName("Top"))
    
    
	l2TLB := builder.Build(fmt.Sprintf("%s.L2TLB", b.gpuName))

	b.l2TLBs = append(b.l2TLBs, l2TLB)
	b.gpu.L2TLBs = append(b.gpu.L2TLBs, l2TLB)

    if profiler.SampledSimulation {

        numset := int(b.dramSize / (1 << b.log2PageSize) / uint64(numWays))
//        bottomdevices := []*virtualtlb.VirtualTLB{b.mmuVirtualMMU}
        virtualtlbbuilder := virtualtlb.MakeBuilder().
                            WithNumSets(numset).
                            WithNumWays(numWays).
                            WithEngine(b.engine).
                            WithFreq(b.freq).
                            WithLatency(1)
        name_tmp := fmt.Sprintf("%s.Virtual",l2TLB.Name() )
        virtualtlb := virtualtlbbuilder.Build(  name_tmp )
        virtualtlb.SetRealComponent(l2TLB)
        //virtualtlb.SetBottomDevice( bottomdevices )
//        for _, elem := range( bottomdevices ) {
            virtualtlb.AddBottomDevice( b.mmuVirtualMMU )
//        }

        b.l2VirtualTLBs = append(b.l2VirtualTLBs,virtualtlb)
    }

	if b.enableVisTracing {
		tracing.CollectTrace(l2TLB, b.visTracer)
	}

	if b.monitor != nil {
		b.monitor.RegisterComponent(l2TLB)
	}
}

func (b *MI100GPUBuilder) numCU() int {
	return b.numCUPerShaderArray * b.numShaderArray
}

func (b *MI100GPUBuilder) connectWithDirectConnection(
	port1, port2 sim.Port,
	bufferSize int,
) {
	conn := sim.NewDirectConnection(
		port1.Name()+"-"+port2.Name(),
		b.engine, b.freq,
	)
	conn.PlugIn(port1, bufferSize)
	conn.PlugIn(port2, bufferSize)
}
