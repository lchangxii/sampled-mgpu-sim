package runner

import (
	"fmt"
	"log"
	"os"

	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/cache/writearound"
	"gitlab.com/akita/mem/v3/cache/writethrough"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm/addresstranslator"
	"gitlab.com/akita/mem/v3/vm/tlb"
	"gitlab.com/akita/mgpusim/v3/timing/cu"
	"gitlab.com/akita/mgpusim/v3/timing/rob"
	"gitlab.com/akita/mgpusim/v3/profiler"
	

	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcu"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcache"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualtlb"
	//"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"
)

type shaderArray struct {
	cus []*cu.ComputeUnit
    Id int
	l1vROBs []*rob.ReorderBuffer
	l1vVirtualRobs [] * virtualcache.VirtualROB
	l1sROB  *rob.ReorderBuffer
	l1sVirtualRob * virtualcache.VirtualROB
	l1iROB  *rob.ReorderBuffer
	l1iVirtualRob * virtualcache.VirtualROB

	l1vATs []*addresstranslator.AddressTranslator
	l1sAT  *addresstranslator.AddressTranslator
	l1iAT  *addresstranslator.AddressTranslator

	l1vCaches []*writearound.Cache
	l1vVirtualCaches [] virtualcache.VirtualCache
	l1sCache  *writethrough.Cache
	l1sVirtualCache  virtualcache.VirtualCache
	l1iCache  *writethrough.Cache
	l1iVirtualCache  virtualcache.VirtualCache

	l1vTLBs []*tlb.TLB
	l1vVirtualTLBs []* virtualtlb.VirtualTLB
	l1sTLB  *tlb.TLB
	l1sVirtualTLB * virtualtlb.VirtualTLB
	l1iTLB  *tlb.TLB
	l1iVirtualTLB * virtualtlb.VirtualTLB
}

type shaderArrayBuilder struct {
	gpuID uint64
	name  string
	numCU int

	engine            sim.Engine
	freq              sim.Freq
	log2CacheLineSize uint64
	log2PageSize      uint64

	isaDebugging bool
	visTracer    tracing.Tracer
	memTracer    tracing.Tracer
}

func makeShaderArrayBuilder() shaderArrayBuilder {
	b := shaderArrayBuilder{
		gpuID:             0,
		name:              "SA",
		numCU:             4,
		freq:              1 * sim.GHz,
		log2CacheLineSize: 6,
		log2PageSize:      12,
	}
	return b
}

func (b shaderArrayBuilder) withEngine(e sim.Engine) shaderArrayBuilder {
	b.engine = e
	return b
}

func (b shaderArrayBuilder) withFreq(f sim.Freq) shaderArrayBuilder {
	b.freq = f
	return b
}

func (b shaderArrayBuilder) withGPUID(id uint64) shaderArrayBuilder {
	b.gpuID = id
	return b
}

func (b shaderArrayBuilder) withNumCU(n int) shaderArrayBuilder {
	b.numCU = n
	return b
}

func (b shaderArrayBuilder) withLog2CachelineSize(
	log2Size uint64,
) shaderArrayBuilder {
	b.log2CacheLineSize = log2Size
	return b
}

func (b shaderArrayBuilder) withLog2PageSize(
	log2Size uint64,
) shaderArrayBuilder {
	b.log2PageSize = log2Size
	return b
}

func (b shaderArrayBuilder) withIsaDebugging() shaderArrayBuilder {
	b.isaDebugging = true
	return b
}

func (b shaderArrayBuilder) withVisTracer(
	visTracer tracing.Tracer,
) shaderArrayBuilder {
	b.visTracer = visTracer
	return b
}

func (b shaderArrayBuilder) withMemTracer(
	memTracer tracing.Tracer,
) shaderArrayBuilder {
	b.memTracer = memTracer
	return b
}

func (b shaderArrayBuilder) Build(name string) shaderArray {
	b.name = name
	sa := shaderArray{}

	b.buildComponents(&sa)
	b.connectComponents(&sa)

	return sa
}

func (b *shaderArrayBuilder) buildComponents(sa *shaderArray) {
	b.buildCUs(sa)

	b.buildL1VTLBs(sa)


	b.buildL1VAddressTranslators(sa)
	b.buildL1VReorderBuffers(sa)
	b.buildL1VCaches(sa)

	b.buildL1STLB(sa)
	b.buildL1SAddressTranslator(sa)
	b.buildL1SReorderBuffer(sa)
	b.buildL1SCache(sa)

	b.buildL1ITLB(sa)
	b.buildL1IAddressTranslator(sa)
	b.buildL1IReorderBuffer(sa)
	b.buildL1ICache(sa)
}

func (b *shaderArrayBuilder) connectComponents(sa *shaderArray) {
	b.connectVectorMem(sa)
	b.connectScalarMem(sa)
	b.connectInstMem(sa)
}

func (b *shaderArrayBuilder) connectVectorMem(sa *shaderArray) {
	for i := 0; i < b.numCU; i++ {
		cu := sa.cus[i]
		rob := sa.l1vROBs[i]
		at := sa.l1vATs[i]
		l1v := sa.l1vCaches[i]
		tlb := sa.l1vTLBs[i]
        if profiler.SampledSimulation {

            cu.Virtualcu.Virtuall1vrob = sa.l1vVirtualRobs[i]
            cu.Virtualcu.Virtuall1vrob.Virtuall1cache = sa.l1vVirtualCaches[i]
            cu.Virtualcu.Virtuall1vrob.Virtuall1tlb = sa.l1vVirtualTLBs[i]
        }

		cu.VectorMemModules = &mem.SingleLowModuleFinder{
			LowModule: rob.GetPortByName("Top"),
		}
		b.connectWithDirectConnection(cu.ToVectorMem,
			rob.GetPortByName("Top"), 8)

		atTopPort := at.GetPortByName("Top")
		rob.BottomUnit = atTopPort
		b.connectWithDirectConnection(
			rob.GetPortByName("Bottom"), atTopPort, 8)

		tlbTopPort := tlb.GetPortByName("Top")
		at.SetTranslationProvider(tlbTopPort)
		b.connectWithDirectConnection(
			at.GetPortByName("Translation"), tlbTopPort, 8)

		at.SetLowModuleFinder(&mem.SingleLowModuleFinder{
			LowModule: l1v.GetPortByName("Top"),
		})
		b.connectWithDirectConnection(l1v.GetPortByName("Top"),
			at.GetPortByName("Bottom"), 8)
	}
}

func (b *shaderArrayBuilder) connectScalarMem(sa *shaderArray) {
	rob := sa.l1sROB
	at := sa.l1sAT
	tlb := sa.l1sTLB
	l1s := sa.l1sCache

	atTopPort := at.GetPortByName("Top")
	rob.BottomUnit = atTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)

	at.SetLowModuleFinder(&mem.SingleLowModuleFinder{
		LowModule: l1s.GetPortByName("Top"),
	})
	b.connectWithDirectConnection(
		l1s.GetPortByName("Top"), at.GetPortByName("Bottom"), 8)

	conn := sim.NewDirectConnection(b.name, b.engine, b.freq)
	conn.PlugIn(rob.GetPortByName("Top"), 8)

    l1sVirtualCache := sa.l1sVirtualCache
    l1sVirtualTLB := sa.l1sVirtualTLB

	for i := 0; i < b.numCU; i++ {
		cu := sa.cus[i]
		cu.ScalarMem = rob.GetPortByName("Top")
		conn.PlugIn(cu.ToScalarMem, 8)
        if profiler.SampledSimulation {

            cu.Virtualcu.Virtuall1srob = sa.l1sVirtualRob
            cu.Virtualcu.Virtuall1srob.Virtuall1cache = l1sVirtualCache
            cu.Virtualcu.Virtuall1srob.Virtuall1tlb = l1sVirtualTLB
        }

	}
}

func (b *shaderArrayBuilder) connectInstMem(sa *shaderArray) {
	rob := sa.l1iROB
	at := sa.l1iAT
	tlb := sa.l1iTLB
	l1i := sa.l1iCache

	l1iTopPort := l1i.GetPortByName("Top")
	rob.BottomUnit = l1iTopPort
	b.connectWithDirectConnection(rob.GetPortByName("Bottom"), l1iTopPort, 8)

	atTopPort := at.GetPortByName("Top")
	l1i.SetLowModuleFinder(&mem.SingleLowModuleFinder{
		LowModule: atTopPort,
	})
	b.connectWithDirectConnection(l1i.GetPortByName("Bottom"), atTopPort, 8)

	tlbTopPort := tlb.GetPortByName("Top")
	at.SetTranslationProvider(tlbTopPort)
	b.connectWithDirectConnection(
		at.GetPortByName("Translation"), tlbTopPort, 8)

	robTopPort := rob.GetPortByName("Top")
	conn := sim.NewDirectConnection(b.name, b.engine, b.freq)
	conn.PlugIn(robTopPort, 8)
    l1iVirtualCache := sa.l1iVirtualCache
    l1iVirtualTLB := sa.l1iVirtualTLB

	for i := 0; i < b.numCU; i++ {
		cu := sa.cus[i]
		cu.InstMem = rob.GetPortByName("Top")
		conn.PlugIn(cu.ToInstMem, 8)
        if profiler.SampledSimulation {
            cu.Virtualcu.Virtuall1irob = sa.l1iVirtualRob
            cu.Virtualcu.Virtuall1irob.Virtuall1cache = l1iVirtualCache
            cu.Virtualcu.Virtuall1irob.Virtuall1tlb = l1iVirtualTLB
        }

	}
}

func (b *shaderArrayBuilder) connectWithDirectConnection(
	port1, port2 sim.Port,
	bufferSize int,
) {
	name := fmt.Sprintf("%s-%s", port1.Name(), port2.Name())
	conn := sim.NewDirectConnection(
		name,
		b.engine, b.freq,
	)
	conn.PlugIn(port1, bufferSize)
	conn.PlugIn(port2, bufferSize)
}

func (b *shaderArrayBuilder) buildCUs(sa *shaderArray) {
	cuBuilder := cu.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithLog2CachelineSize(b.log2CacheLineSize)

    var virtualcubuilder virtualcu.Builder
    if profiler.SampledSimulation {
        virtualcubuilder = virtualcu.MakeBuilder().
                                    WithEngine(b.engine).
                                    WithFreq(b.freq)

    }


	for i := 0; i < b.numCU; i++ {
		cuName := fmt.Sprintf("%s.CU_%02d", b.name, i)
		computeUnit := cuBuilder.Build(cuName)

        if profiler.SampledSimulation {
            //computeUnit.Virtualcu = virtualcubuilder.Build(computeUnit)
            tmp_name := fmt.Sprintf( "%s.Virtual", cuName )
            computeUnit.Virtualcu = virtualcubuilder.Build( tmp_name )
            computeUnit.Virtualcu.SetRealComponent(computeUnit)
        }
		sa.cus = append(sa.cus, computeUnit)
    
		if b.isaDebugging {
			isaDebug, err := os.Create(
				fmt.Sprintf("isa_%s.debug", cuName))
			if err != nil {
				log.Fatal(err.Error())
			}
			isaDebugger := cu.NewISADebugger(
				log.New(isaDebug, "", 0), computeUnit)

			tracing.CollectTrace(computeUnit, isaDebugger)
		}

		if b.visTracer != nil {
			tracing.CollectTrace(computeUnit, b.visTracer)
		}
	}
}

func (b *shaderArrayBuilder) buildL1VReorderBuffers(sa *shaderArray) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)
    
   // if profiler.SampledSimulation {
        virtualtlbbuilder := virtualcache.MakeBuilder().
                            WithEngine(b.engine).
                            WithFreq(b.freq)
//    }

	for i := 0; i < b.numCU; i++ {
		name := fmt.Sprintf("%s.L1VROB_%02d", b.name, i)
		rob := builder.Build(name)
		sa.l1vROBs = append(sa.l1vROBs, rob)

        if profiler.SampledSimulation {
            name_tmp := fmt.Sprintf("%s.Virtual",rob.Name() )
            virtualrob := virtualtlbbuilder.BuildROB(name_tmp  ) 
            //virtualtlb.SetRealComponent(rob)
            sa.l1vVirtualRobs = append( sa.l1vVirtualRobs, virtualrob  )
        }
		if b.visTracer != nil {
			tracing.CollectTrace(rob, b.visTracer)
		}
	}
}

func (b *shaderArrayBuilder) buildL1VAddressTranslators(sa *shaderArray) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	for i := 0; i < b.numCU; i++ {
		name := fmt.Sprintf("%s.L1VAddrTrans_%02d", b.name, i)
		at := builder.Build(name)
		sa.l1vATs = append(sa.l1vATs, at)

		if b.visTracer != nil {
			tracing.CollectTrace(at, b.visTracer)
		}
	}
}

func (b *shaderArrayBuilder) buildL1VTLBs(sa *shaderArray) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(64).
		WithNumReqPerCycle(4)
    var virtualtlbbuilder virtualtlb.Builder
    if profiler.SampledSimulation {
        virtualtlbbuilder = virtualtlb.MakeBuilder().
                            WithNumSets(1).
                            WithNumWays(64).
                            WithLatency(3).
                            WithEngine(b.engine).
                            WithFreq(b.freq)
//                            WithPipelineStageNum(0)
    }
	for i := 0; i < b.numCU; i++ {
		name := fmt.Sprintf("%s.L1VTLB_%02d", b.name, i)
		tlb := builder.Build(name)
		sa.l1vTLBs = append(sa.l1vTLBs, tlb)
        
        if profiler.SampledSimulation {

            name_tmp := fmt.Sprintf("%s.Virtual",tlb.Name() )
            virtualtlb := virtualtlbbuilder.Build(name_tmp  ) 
            virtualtlb.SetRealComponent(tlb)
            sa.l1vVirtualTLBs = append(sa.l1vVirtualTLBs, virtualtlb )
        }
		if b.visTracer != nil {
			tracing.CollectTrace(tlb, b.visTracer)
		}
	}
}

func (b *shaderArrayBuilder) buildL1VCaches(sa *shaderArray) {
    const nummshrentry = 16
    const wayassociativity = 4
    const byteSize = 16 * mem.KB
	builder := writearound.NewBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBankLatency(60).
		WithNumBanks(1).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(wayassociativity).
		WithNumMSHREntry(nummshrentry).
		WithTotalByteSize( byteSize )

	if b.visTracer != nil {
		builder = builder.WithVisTracer(b.visTracer)
	}
    var virtualcachebuilder virtualcache.Builder
    if profiler.SampledSimulation {

	    blockSize := 1 << b.log2CacheLineSize
	    numSet := int( byteSize / uint64( wayassociativity*blockSize) )

        virtualcachebuilder = virtualcache.MakeBuilder().
                                WithNumWays(wayassociativity).
                                WithNumSets( numSet).
                                WithNumBanks(1).
                                WithNumMSHREntry(nummshrentry).
		                        WithLog2BlockSize(b.log2CacheLineSize).
                                WithEngine(b.engine).
                                WithFreq(b.freq).
                                WithPipelineStageNum(5).
                                WithSendPipelineStageNum(3).
                                WithLatency(60).
                                WithPostPipelineStageNum(6)
    }

	for i := 0; i < b.numCU; i++ {
		name := fmt.Sprintf("%s.L1VCache_%02d", b.name, i)
		cache := builder.Build(name)
		sa.l1vCaches = append(sa.l1vCaches, cache)
        if profiler.SampledSimulation {

		    name_tmp := fmt.Sprintf("%s.Virtual", cache.Name() )
            virtualcache := virtualcachebuilder.BuildWritearound( name_tmp )
            virtualcache.SetRealComponent(cache)
            sa.l1vVirtualCaches = append(sa.l1vVirtualCaches, virtualcache )
        }

		if b.memTracer != nil {
			tracing.CollectTrace(cache, b.memTracer)
		}
	}
}

func (b *shaderArrayBuilder) buildL1SReorderBuffer(sa *shaderArray) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1SROB", b.name)
	rob := builder.Build(name)
	sa.l1sROB = rob

    if profiler.SampledSimulation {
        virtualtlbbuilder := virtualcache.MakeBuilder().
                            WithEngine(b.engine).
                            WithFreq(b.freq)

        name_tmp := fmt.Sprintf("%s.Virtual",rob.Name() )
        virtualrob := virtualtlbbuilder.BuildROB(name_tmp  ) 
        //virtualtlb.SetRealComponent(rob)
        sa.l1sVirtualRob = virtualrob 
    }

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1SAddressTranslator(sa *shaderArray) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.L1SAddrTrans", b.name)
	at := builder.Build(name)
	sa.l1sAT = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1STLB(sa *shaderArray) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(64).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1STLB", b.name)
	tlb := builder.Build(name)
	sa.l1sTLB = tlb
    if profiler.SampledSimulation {
        virtualtlbbuilder := virtualtlb.MakeBuilder().
                            WithNumSets(1).
                            WithNumWays(64).
                            WithEngine(b.engine).
                            WithFreq(b.freq)
//		                    WithLatency(1)
//                            WithPipelineStageNum(0)
        name_tmp := fmt.Sprintf("%s.Virtual",tlb.Name() )
        virtualtlb := virtualtlbbuilder.Build( name_tmp  ) 
        virtualtlb.SetRealComponent(tlb)
        virtualtlb.SetLatency( 3)
        sa.l1sVirtualTLB = virtualtlb 
    }

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1SCache(sa *shaderArray) {
    const nummshrentry = 4
    const wayassociativity = 4
    const byteSize = 16 * mem.KB
	builder := writethrough.NewBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBankLatency(1).
		WithNumBanks(1).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(4).
		WithNumMSHREntry(16).
		WithTotalByteSize(16 * mem.KB)
        

	name := fmt.Sprintf("%s.L1SCache", b.name)
	cache := builder.Build(name)
	sa.l1sCache = cache

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
		                                WithLatency(10).
                                        WithPostPipelineStageNum(0)
    
		name_tmp := fmt.Sprintf("%s.Virtual", cache.Name() )
        virtualcache := virtualcachebuilder.Build( name_tmp )
        virtualcache.SetRealComponent(cache)
        sa.l1sVirtualCache = virtualcache

    }

	if b.visTracer != nil {
		tracing.CollectTrace(cache, b.visTracer)
	}

	if b.memTracer != nil {
		tracing.CollectTrace(cache, b.memTracer)
	}
}

func (b *shaderArrayBuilder) buildL1IReorderBuffer(sa *shaderArray) {
	builder := rob.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBufferSize(128).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1IROB", b.name)
	rob := builder.Build(name)
	sa.l1iROB = rob
    if profiler.SampledSimulation {
        virtualtlbbuilder := virtualcache.MakeBuilder().
                            WithEngine(b.engine).
                            WithFreq(b.freq)

        name_tmp := fmt.Sprintf("%s.Virtual",rob.Name() )
        virtualrob := virtualtlbbuilder.BuildROB(name_tmp  ) 
        //virtualtlb.SetRealComponent(rob)
        sa.l1iVirtualRob = virtualrob 
    }

	if b.visTracer != nil {
		tracing.CollectTrace(rob, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1IAddressTranslator(sa *shaderArray) {
	builder := addresstranslator.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithDeviceID(b.gpuID).
		WithLog2PageSize(b.log2PageSize)

	name := fmt.Sprintf("%s.L1IAddrTrans", b.name)
	at := builder.Build(name)
	sa.l1iAT = at

	if b.visTracer != nil {
		tracing.CollectTrace(at, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1ITLB(sa *shaderArray) {
	builder := tlb.MakeBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithNumMSHREntry(4).
		WithNumSets(1).
		WithNumWays(64).
		WithNumReqPerCycle(4)

	name := fmt.Sprintf("%s.L1ITLB", b.name)
	tlb := builder.Build(name)
	sa.l1iTLB = tlb
    if profiler.SampledSimulation {
        virtualtlbbuilder := virtualtlb.MakeBuilder().
                            WithNumSets(1).
                            WithNumWays(64).
                            WithEngine(b.engine).
                            WithFreq(b.freq)
//                            WithPipelineStageNum(60)

        name_tmp := fmt.Sprintf("%s.Virtual", tlb.Name() )
        virtualtlb := virtualtlbbuilder.Build( name_tmp ) 
        virtualtlb.SetRealComponent(tlb)
        sa.l1iVirtualTLB = virtualtlb 
    }

	if b.visTracer != nil {
		tracing.CollectTrace(tlb, b.visTracer)
	}
}

func (b *shaderArrayBuilder) buildL1ICache(sa *shaderArray) {
    const nummshrentry = 16
    const byteSize = 32 * mem.KB
    const wayassociativity = 4
	builder := writethrough.NewBuilder().
		WithEngine(b.engine).
		WithFreq(b.freq).
		WithBankLatency(1).
		WithNumBanks(1).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(wayassociativity).
		WithNumMSHREntry(nummshrentry).
		WithTotalByteSize( byteSize).
		WithNumReqsPerCycle(4)

	name := fmt.Sprintf("%s.L1ICache", b.name)
	cache := builder.Build(name)
	sa.l1iCache = cache
    if profiler.SampledSimulation {
	    blockSize := 1 << b.log2CacheLineSize
	    numSet := int( byteSize / uint64( wayassociativity*blockSize) )

        virtualcachebuilder := virtualcache.MakeBuilder().
                            WithEngine(b.engine).
                            WithFreq(b.freq).
                            WithNumMSHREntry(nummshrentry).
                            WithNumWays(wayassociativity).
		                    WithLog2BlockSize(b.log2CacheLineSize).
                            WithNumSets(numSet)
   
		name_tmp := fmt.Sprintf("%s.Virtual", cache.Name() )
        virtualcache := virtualcachebuilder.Build( name_tmp )
        virtualcache.SetRealComponent(cache)
        sa.l1iVirtualCache = virtualcache 
    }

	if b.visTracer != nil {
		tracing.CollectTrace(cache, b.visTracer)
	}

	if b.memTracer != nil {
		tracing.CollectTrace(cache, b.memTracer)
	}
}
