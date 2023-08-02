package dispatching

import (
	"fmt"
	"log"
	"flag"
	"gitlab.com/akita/akita/v3/monitoring"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mgpusim/v3/kernels"
	"gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"
	"gitlab.com/akita/mgpusim/v3/emu"
	"gitlab.com/akita/mgpusim/v3/protocol"
	"gitlab.com/akita/mgpusim/v3/timing/cp/internal/resource"
)

// A Dispatcher is a sub-component of a command processor that can dispatch
// work-groups to compute units.
type Dispatcher interface {
	tracing.NamedHookable
	RegisterCU(cu resource.DispatchableCU)
	IsDispatching() bool
	StartDispatching(req *protocol.LaunchKernelReq, now sim.VTimeInSec)
	Tick(now sim.VTimeInSec) (madeProgress bool)
}

// A DispatcherImpl is a ticking component that can dispatch work-groups.
type DispatcherImpl struct {
	sim.HookableBase

	cp                     tracing.NamedHookable
	name                   string
	respondingPort         sim.Port
	dispatchingPort        sim.Port
	alg                    algorithm
	dispatching            *protocol.LaunchKernelReq
	currWG                 dispatchLocation
	cycleLeft              int
	numDispatchedWGs       int
	numDispatchedWFs       uint64
	numCompletedWGs        int
	numCompletedWFs        uint64
	inflightWGs            map[string]dispatchLocation
	originalReqs           map[string]*protocol.MapWGReq
	latencyTable           []int
	constantKernelOverhead int
    kernel_issuetime sim.VTimeInSec
	monitor     *monitoring.Monitor
	progressBar *monitoring.ProgressBar
}

// Name returns the name of the dispatcher
func (d *DispatcherImpl) Name() string {
	return d.name
}

// RegisterCU allows the dispatcher to dispatch work-groups to the CU.
func (d *DispatcherImpl) RegisterCU(cu resource.DispatchableCU) {
	d.alg.RegisterCU(cu)
}

// IsDispatching checks if the dispatcher is dispatching another kernel.
func (d *DispatcherImpl) IsDispatching() bool {
	return d.dispatching != nil
}
var OnlyAnalysisFlag = flag.Bool("only-analysis", false,
	"only analysis gpu workloads; won't do detailed sim.")

func (d DispatcherImpl) StaticAnalysisKernelSampled( )  {
    currWG := d.alg.Next()
	if !currWG.valid {
        return
    }

        wg := currWG.wg
        reqBuilder := protocol.NewMapWGReqBuilder().
		    WithWG( wg).
		    WithPID(d.dispatching.PID)
	    for _, l := range currWG.locations {
		    reqBuilder = reqBuilder.AddWf(l)
        }
	    req := reqBuilder.Build()

        emu.Static_compute_unit.AnalysisKernel(req)
        emu.Static_compute_unit.PrintData()
    d.alg.FreeResources(currWG)
}
func (d DispatcherImpl) AnalysisKernelSampled( ) ( uint64,uint64) {

	numwg := uint64( d.alg.NumWG() )
	next_wg_exit := d.alg.HasNext()
    wfidx := uint64(0)
    dispatchLocations := make( []dispatchLocation , 0)
    var wfperwg uint64
    log.Printf("the wg number : %d\n",numwg)
    for next_wg_exit {
        currWG := d.alg.Next()
		if !currWG.valid {
            break
        }
        locations := currWG.locations
        dispatchLocations = append(dispatchLocations,currWG)

	    d.alg.FreeResources(currWG)

        wfperwg = uint64( len( locations ) )
        wfidx += wfperwg
	    next_wg_exit = d.alg.HasNext()
    }
    emu.Bbvcomputeunit.Wfnum = wfidx
	for i, location := range dispatchLocations {
        if i % 100 != 0 {
            continue
        }
        wg := location.wg
        reqBuilder := protocol.NewMapWGReqBuilder().
		    WithWG( wg).
		    WithPID(d.dispatching.PID)
	    for _, l := range location.locations {
		    reqBuilder = reqBuilder.AddWf(l)
        }
	    req := reqBuilder.Build()
        emu.Bbvcomputeunit.RunWG(req)

        if * sampledrunner.BranchSampledFlag || *sampledrunner.SampledRunnerFlag{
            if uint64(i) > *sampledrunner.KernelSampledThreshold {
                break
            }
        }
	}

    bbvs := emu.Bbvcomputeunit.GetAllonlineBBVs()
    if * sampledrunner.BranchSampledFlag {
        sampledrunner.Branchsampledengine.Analysis( bbvs)
    }
    var wfnumskip uint64
    var wgnumskip uint64
    if wfperwg == 0 {
//        fmt.Printf("disable sampled simulation for wf0")
//        if *sampledrunner.BranchSampledFlag {
//            sampledrunner.Branchsampledengine.Disabled()
//        }
//        if *sampledrunner.SampledRunnerFlag {
//            sampledrunner.Sampledengine.Disabled()
//        }
//        wgnumskip = 0
    } else {
        if *sampledrunner.KernelSampledFlag {
            wfnumskip=sampledrunner.Kernelsampledengine.Analysis( bbvs,numwg * wfperwg )
        
            wgnumskip = wfnumskip / wfperwg
        }
    }
//    if ( wfidx< *sampledrunner.KernelSampledThreshold) {
//        fmt.Printf("disable sampled simulation for wf0")
//        if *sampledrunner.BranchSampledFlag {
//            sampledrunner.Branchsampledengine.Disabled()
//        }
//
//        if *sampledrunner.SampledRunnerFlag {
//            sampledrunner.Sampledengine.Disabled()
//        }
//        return 0,0
//    }
    return wgnumskip,wfperwg

}
// StartDispatching lets the dispatcher to start dispatch another kernel.
func (d *DispatcherImpl) StartDispatching(req *protocol.LaunchKernelReq,now sim.VTimeInSec) {
    profiler.Walltime.InitStartWallTime( "dispatcher" )
    d.kernel_issuetime = now
	d.mustNotBeDispatchingAnotherKernel()
    info := kernels.KernelLaunchInfo{
		CodeObject: req.HsaCo,
		Packet:     req.Packet,
		PacketAddr: req.PacketAddress,
		WGFilter:   req.WGFilter,
	}
    if profiler.Wffinalfeature != nil {
        profiler.Wffinalfeature.CollectKernelStart(now)
    }
    profiler.Walltime.InitStartWallTime( "analysis" )
	d.dispatching = req
	d.alg.StartNewKernel(info)
    fmt.Printf(" set kernel; wg %d to finished\n",d.alg.NumWG())
    packet := info.Packet
    workgroupsizex := int(packet.WorkgroupSizeX)
    workgroupsizey := int(packet.WorkgroupSizeY)
    workgroupsizez := int(packet.WorkgroupSizeZ)
    wfnums := (d.alg.NumWG())* workgroupsizex*workgroupsizey*workgroupsizez/64
    fmt.Printf(" wf %d to finished\n",wfnums)

    disabledanalysis := false
    if *sampledrunner.BranchSampledFlag{
        if !disabledanalysis {
            d.StaticAnalysisKernelSampled()
            d.alg.StartNewKernel(info)
        }
    }

    if ( *sampledrunner.BranchSampledFlag || (*sampledrunner.KernelSampledFlag && sampledrunner.Kernelsampledengine.HistorySize() > 0)) && uint64( d.alg.NumWG()) >  *sampledrunner.KernelSampledThreshold || *OnlyAnalysisFlag {
        
        sampledrunner.Sampledtimeengine.SetTargetCompletedWfs(uint64(wfnums))

        if !disabledanalysis {
            wgnum2skip,wfperwg := d.AnalysisKernelSampled()
            fmt.Printf("wgnum2skip %d wfperwg %d\n",wgnum2skip,wfperwg)
            d.alg.StartNewKernel(info)
        }
//        if false&&wgnum2skip != 0 {
//            ///skip wg 
//            for idx:=0 ; idx<int(wgnum2skip) ;idx++ {
//	    	    d.currWG = d.alg.Next()
//    	        d.alg.FreeResources(d.currWG)
//            //d.currWG = nil
//		        d.currWG.valid = false
//            }
////	    	d.currWG = d.alg.Next()
////            d.kernel_sampled_issuetime = sampledrunner.Kernelsampledengine.Issuetime(wgnum2skip*wfperwg) + now
//            sampledrunner.Kernelsampledengine.SetOffset(now)
//            fmt.Printf("kernel launch %.2f issue ",now*1e9)
//	        d.numDispatchedWGs = int( wgnum2skip)
//        	d.numDispatchedWFs = wgnum2skip * wfperwg
//    	    d.numCompletedWGs = int( wgnum2skip)
//        } else {

//        }
    }
	d.numDispatchedWGs = 0
    d.numDispatchedWFs = 0
    d.numCompletedWFs = 0
	d.numCompletedWGs = 0

    walltimeinterval := profiler.Walltime.GetIntervalWallTime( "analysis" )
    //    profiler.Fullsim.FFlush( float64(now-d.kernel_issuetime), walltimeinterval )
    profiler.Fullsim.Analysistime = walltimeinterval
    emu.Bbvcomputeunit.Analysistime = walltimeinterval
	d.initializeProgressBar(req.ID)
}

func (d *DispatcherImpl) initializeProgressBar(kernelID string) {
	if d.monitor != nil {
		d.progressBar = d.monitor.CreateProgressBar(
			fmt.Sprintf("At %s, Kernel: %s, ", d.Name(), kernelID),
			uint64(d.alg.NumWG()),
		)
	}
}

func (d *DispatcherImpl) mustNotBeDispatchingAnotherKernel() {
	if d.IsDispatching() {
		panic("dispatcher is dispatching another request")
	}
}

// Tick updates the state of the dispatcher.
func (d *DispatcherImpl) Tick(now sim.VTimeInSec) (madeProgress bool) {
	if d.cycleLeft > 0 {
		d.cycleLeft--
		return true
	}
//    if now < d.kernel_sampled_issuetime {
//        return true
//    }

	if d.dispatching != nil {
		if d.kernelCompleted() {
			madeProgress = d.completeKernel(now) || madeProgress
		} else {
			madeProgress = d.dispatchNextWG(now) || madeProgress
		}
	}

	madeProgress = d.processMessagesFromCU(now) || madeProgress

	return madeProgress
}

func (d *DispatcherImpl) processMessagesFromCU(now sim.VTimeInSec) bool {
	msg := d.dispatchingPort.Peek()
	if msg == nil {
		return false
	}

	switch msg := msg.(type) {
	case *protocol.WGCompletionMsg:
		location, ok := d.inflightWGs[msg.RspTo]
        //_,enable := sampledrunner.Sampledengine.Predict()
//        if enable {
//
//                fmt.Printf( " %t\n",  ok )
//        }

//	    for _, l := range location.locations {
//            wavefront := l.Wavefront
//            if enable {
//
//                fmt.Printf( "dispatch136wf %s end time:  %.18f %t\n",  wavefront.UID,now,ok )
//            }
//        }
		if !ok {
			return false
		}
//        wg := location.wg
//        if wg.CollectWfMem {
//            wgfeature := sampledrunner.WGFeatureMemTracer{
//                IDX:wg.IDX,
//                IDY:wg.IDY,
//                IDZ:wg.IDZ,
//                WGFeatureMem: &kernels.WGFeatureMem {
//                    WFMemFootprint : make(map[string][]*kernels.MemFootprint),
//                },
//            }
//            for _, wf := range wg.Wavefronts  {
//                wgfeature.WFMemFootprint[wf.UID] = wf.CollectMemFootprints
//                wgfeature.WfIDs = append(wgfeature.WfIDs, wf.UID)
//            }
//            sampledrunner.WgFeatureVector = append(sampledrunner.WgFeatureVector,wgfeature)
//        }

        if *sampledrunner.SampledRunnerFlag {

//            wg := location.wg

	        for _, l := range location.locations {
                wavefront := l.Wavefront
                sampledrunner.Sampledengine.Collect( wavefront.Issuetime,wavefront.Finishtime )
//                _,enable := sampledrunner.Sampledengine.Predict()
//                if enable {
//                fmt.Printf( "dispatch168wf %s end time:  %.18f\n",  wavefront.UID,now )
//                }
            }
        }
		d.alg.FreeResources(location)
		delete(d.inflightWGs, msg.RspTo)
		d.numCompletedWFs += uint64(len( location.locations))
		d.numCompletedWGs ++
		if d.numCompletedWGs == d.alg.NumWG() {
			d.cycleLeft = d.constantKernelOverhead
		}

		d.dispatchingPort.Retrieve(now)

		originalReq := d.originalReqs[msg.RspTo]
		delete(d.originalReqs, msg.RspTo)
		tracing.TraceReqFinalize(originalReq, d)

		if d.progressBar != nil {
			d.progressBar.MoveInProgressToFinished(1)
		}

		return true
	}

	return false
}

func (d *DispatcherImpl) kernelCompleted() bool {
    if *OnlyAnalysisFlag {
        return true
    }
	if d.currWG.valid {
		return false
	}

	if d.alg.HasNext() {
		return false
	}

	if d.numCompletedWGs < d.numDispatchedWGs {
		return false
	}
    fmt.Printf("complete wg %d\n",d.numCompletedWGs)
	return true
}

func (d *DispatcherImpl) completeKernel(now sim.VTimeInSec) (
	madeProgress bool,
) {
	req := d.dispatching

	rsp := protocol.NewLaunchKernelRsp(now, req.Dst, req.Src, req.ID)

	err := d.respondingPort.Send(rsp)
	if err == nil {
		d.dispatching = nil

		if d.monitor != nil {
			d.monitor.CompleteProgressBar(d.progressBar)
		}

		tracing.TraceReqComplete(req, d.cp)

        walltimeinterval := profiler.Walltime.GetIntervalWallTime( "dispatcher" )
    
        profiler.Fullsim.FFlush( float64(now-d.kernel_issuetime), walltimeinterval )
        emu.Bbvcomputeunit.FFlush()
		return true
	}

	return false
}

func (d *DispatcherImpl) dispatchNextWG(
	now sim.VTimeInSec,
) (madeProgress bool) {
	if !d.currWG.valid {
        haswg := true
		if !d.alg.HasNext() {
            haswg = false
		}
        if haswg {
    		d.currWG = d.alg.Next()
		    if !d.currWG.valid { //this means that we do not have resources we enable sampling at this point
	    		haswg = false
		    }
        }
            if *sampledrunner.SampledRunnerFlag {
                if sampledrunner.Sampledengine.IfDisable() {
                    log.Printf("enabled warp sampling")
                    sampledrunner.Sampledengine.Enable()
                }
            }
            if *sampledrunner.BranchSampledFlag {
                if sampledrunner.Branchsampledengine.IfDisable() {
                    log.Printf("enabled branch sampling")
                    sampledrunner.Branchsampledengine.Enable()
                }
            }

        if !haswg {

            return false
        }
	}
    reqBuilder := protocol.NewMapWGReqBuilder().
		WithSrc(d.dispatchingPort).
		WithDst(d.currWG.cu).
		WithSendTime(now).
		WithPID(d.dispatching.PID).
		WithWG(d.currWG.wg)
    wfskip := false
    wf_interval_time := sim.VTimeInSec(0)
    if  *sampledrunner.SampledRunnerFlag {
       wf_interval_time,wfskip = sampledrunner.Sampledengine.Predict()
    }
    kernelskip := false
    if *sampledrunner.KernelSampledFlag {
        kernelskip = sampledrunner.Kernelsampledengine.EnableSampled()
    }
    skip:=false
    if kernelskip{
        skip = true
    }
    interval_time := sim.VTimeInSec(0)
	for idx, l := range d.currWG.locations {
        if *sampledrunner.KernelSampledFlag {
            wfidx := uint64(idx) + d.numDispatchedWFs
            if kernelskip {
                interval_time, _ = sampledrunner.Kernelsampledengine.Predict(wfidx)
//                var wait_issue bool
//                interval_time, wait_issue = sampledrunner.Kernelsampledengine.Predict(wfidx)
//                if wait_issue {
//                    issuetime := sampledrunner.Kernelsampledengine.Issuetime(wfidx)
//                    if now <= issuetime - 2*1e-8{
//                        return true
//                    }
//                }
            }
//            interval_time = interval_time + sim.VTimeInSec(1e-9*float64(wfidx))
        }
        if (!skip ) && *sampledrunner.SampledRunnerFlag {
//           interval_time,skip = sampledrunner.Sampledengine.Predict()
           interval_time = wf_interval_time
           skip = wfskip
        }
        l.Wavefront.Skip = skip
        l.Wavefront.Predtime = interval_time

		reqBuilder = reqBuilder.AddWf(l)
	}
	req := reqBuilder.Build()
	err := d.dispatchingPort.Send(req)

	// fmt.Printf("%.10f, %d, %d\n", now, d.currWG.wg.IDX, d.currWG.cuID)

	if err == nil {
		d.currWG.valid = false
		d.numDispatchedWGs++
		d.numDispatchedWFs += uint64(len(d.currWG.locations))
		d.inflightWGs[req.ID] = d.currWG
		d.originalReqs[req.ID] = req
		d.cycleLeft = d.latencyTable[len(d.currWG.locations)]
        //fmt.Printf("dispatch %d completed %d\n",d.numDispatchedWFs,d.numCompletedWFs)
        sampledrunner.Sampledtimeengine.UpdateMaxWFS( d.numDispatchedWFs - d.numCompletedWFs )
		if d.progressBar != nil {
			d.progressBar.IncrementInProgress(1)
		}

		tracing.TraceReqInitiate(req, d,
			tracing.MsgIDAtReceiver(d.dispatching, d.cp))

		return true
	}

	return false
}
