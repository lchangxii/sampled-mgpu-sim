package cu

import (
	"log"
	"reflect"

	"github.com/rs/xid"
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/mem"
	//"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/mgpusim/v3/emu"
	"gitlab.com/akita/mgpusim/v3/utils"
	"gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mgpusim/v3/insts"
	"gitlab.com/akita/mgpusim/v3/kernels"
	"gitlab.com/akita/mgpusim/v3/protocol"
	"gitlab.com/akita/mgpusim/v3/timing/wavefront"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcu"
	"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"

    "fmt"
)

// A ComputeUnit in the timing package provides a detailed and accurate
// simulation of a GCN3 ComputeUnit
type ComputeUnit struct {
	*sim.TickingComponent
    Id int
    Virtualcu *virtualcu.VirtualCU
	WfDispatcher WfDispatcher
	Decoder      emu.Decoder
	WfPools      []*WavefrontPool
    WfPoolSize   int
    simdCount int

	inflightInst map[string]int

	InFlightInstFetch            []*InstFetchReqInfo
	InFlightScalarMemAccess      []*ScalarMemAccessInfo
	InFlightVectorMemAccess      []VectorMemAccessInfo
	InFlightVectorMemAccessLimit int

	shadowInFlightInstFetch       []*InstFetchReqInfo
	shadowInFlightScalarMemAccess []*ScalarMemAccessInfo
	shadowInFlightVectorMemAccess []VectorMemAccessInfo

	running bool

	Scheduler        Scheduler
	BranchUnit       SubComponent
	VectorMemDecoder SubComponent
	VectorMemUnit    SubComponent
	ScalarDecoder    SubComponent
	VectorDecoder    SubComponent
	LDSDecoder       SubComponent
	ScalarUnit       SubComponent
	SIMDUnit         []SubComponent
	LDSUnit          SubComponent
	SRegFile         RegisterFile
	VRegFile         []RegisterFile

	InstMem          sim.Port
	ScalarMem        sim.Port
	VectorMemModules mem.LowModuleFinder
    

	ToACE       sim.Port
	toACESender sim.BufferedSender
	ToInstMem   sim.Port
	ToScalarMem sim.Port
	ToVectorMem sim.Port
	ToCP        sim.Port

	inCPRequestProcessingStage sim.Msg
	cpRequestHandlingComplete  bool

	isFlushing                   bool
	isPaused                     bool
	isSendingOutShadowBufferReqs bool
	isHandlingWfCompletionEvent  bool
    
	toSendToCP sim.Msg

	currentFlushReq   *protocol.CUPipelineFlushReq
	currentRestartReq *protocol.CUPipelineRestartReq
    wftime map[string] sim.VTimeInSec
    setAllWfsPausedForSampling bool
}

// Handle processes that events that are scheduled on the ComputeUnit
func (cu *ComputeUnit) Handle(evt sim.Event) error {
	ctx := sim.HookCtx{
		Domain: cu,
		Pos:    sim.HookPosBeforeEvent,
		Item:   evt,
	}
	cu.InvokeHook(ctx)

	cu.Lock()
	defer cu.Unlock()

	switch evt := evt.(type) {
	case sim.TickEvent:
		cu.TickingComponent.Handle(evt)
	case *wavefront.WfCompletionEvent:
		cu.handleWfCompletionEvent(evt)
	default:
		log.Panicf("Unable to process evevt of type %s",
			reflect.TypeOf(evt))
	}

	ctx.Pos = sim.HookPosAfterEvent
	cu.InvokeHook(ctx)

	return nil
}

// ControlPort returns the port that can receive controlling messages from the
// Command Processor.
func (cu *ComputeUnit) ControlPort() sim.Port {
	return cu.ToCP
}

// DispatchingPort returns the port that the dispatcher can use to dispatch
// work-groups to the CU.
func (cu *ComputeUnit) DispatchingPort() sim.Port {
	return cu.ToACE
}

// WfPoolSizes returns an array of the numbers of wavefronts that each SIMD unit
// can execute.
func (cu *ComputeUnit) WfPoolSizes() []int {
    wfpoolsize := cu.WfPoolSize
    
    ret := make([]int,cu.simdCount)
    for i:= 0; i<cu.simdCount ; i++ {
        ret[i] = wfpoolsize
    }
    return ret
//	return []int{wfpoolsize, wfpoolsize, wfpoolsize, wfpoolsize}
}

// VRegCounts returns an array of the numbers of vector regsiters in each SIMD
// unit.
func (cu *ComputeUnit) VRegCounts() []int {
	return []int{16384, 16384, 16384, 16384}
}

// SRegCount returns the number of scalar register in the Compute Unit.
func (cu *ComputeUnit) SRegCount() int {
	return 3200
}

// LDSBytes returns the number of bytes in the LDS of the CU.
func (cu *ComputeUnit) LDSBytes() int {
	return 64 * 1024
}

func (cu *ComputeUnit) checkAllDevicesStatus() {
    if cu.BranchUnit.IsIdle() {
        log.Printf("branch unit is idle")
    } else {
        log.Printf("branch unit is busy")
    }
    if cu.VectorMemDecoder.IsIdle() {
        log.Printf("vector mem unit is idle")
    } else {
        log.Printf("vector mem unit is busy")
    }
    if cu.VectorMemUnit.IsIdle() {
        log.Printf("vector mem is idle")
    } else {
        log.Printf("vector mem is busy")
    }
    if cu.ScalarDecoder.IsIdle() {
        log.Printf("scalar decoder is idle")
    } else {
        log.Printf("scalar decoder is busy")
    }

}
func (cu *ComputeUnit) checkInflightInst(wf *wavefront.Wavefront ) bool{
    for _,elem   := range(cu.InFlightInstFetch) {
        if elem.Wavefront  == wf {
            return false
        }
    }
    for _,elem   := range(cu.InFlightScalarMemAccess) {
        if elem.Wavefront  == wf {
            return false
        }
    }
    for _,elem   := range(cu.InFlightVectorMemAccess) {
        if elem.Wavefront  == wf {
            return false
        }
    }


    return true
}
func (cu *ComputeUnit) ResetForSampling() {
    cu.setAllWfsPausedForSampling = false
    cu.Scheduler.StartNewCode()
}
func (cu *ComputeUnit) stopTimeModel(
	now sim.VTimeInSec,
)  {
    if cu.setAllWfsPausedForSampling {
//        cu.checkAllDevicesStatus()
        return
    }
    if *sampledrunner.SampledRunnerFlag {
        predtime,enableSampled := sampledrunner.Sampledengine.Predict()
//        _,enableSampled := sampledrunner.Sampledengine.Predict()
        if enableSampled{
            
            cu.Scheduler.StopNewCode()
            global_finished := true
            for _, wfPool := range cu.WfPools {
		        for _, wf_out := range wfPool.wfs {
                    if wf_out == nil {
                        continue
                    }
//                    if wf_out.State == wavefront.WfCompleted {//do nothing
//                    } else 
                    if wf_out.State == wavefront.WfRunning { //wait until finished
                        //log.Printf("%s is running\n",wf_out.UID)
                        global_finished = false
                        continue
                    }  else if wf_out.State == wavefront.WfReady || wf_out.State==wavefront.WfAtBarrier || wf_out.State==wavefront.WfCompleted {
//                        if cu.checkInflightInst(wf) {
                            wg := wf_out.WG
                            if wf_out.State == wavefront.WfCompleted {
                                allwfscompleted := true
                                for _,wf_it := range wg.Wfs{
                                    if wf_it.State == wavefront.WfCompleted || wf_it.State == wavefront.WfSampledCompleted {
                                        continue
                                    } else {
                                        allwfscompleted = false
                                        break
                                    }
                                }
                                if allwfscompleted {
                                    continue
                                }
                            }


                            finished := true
                            for _,wf_it := range wg.Wfs{
                                if wf_it.State == wavefront.WfRunning {
                                    finished = false
                                    break
                                    //return
                                }
                            }
//                        if !finished {
//                            log.Printf("unfinished") 
//                            for _,wf_it := range wg.Wfs{
//                                log.Printf("wfid %s %d\n",wf_it.UID,wf_it.State)
//                            }
//                        } else {
//                             log.Printf("finished") 
//                            for _,wf_it := range wg.Wfs{
//                                log.Printf("wfid %s %d\n",wf_it.UID,wf_it.State)
//                            }
//                       
//                        }


                            if !finished {
                                global_finished = false
                            } else {
                                cu.Scheduler.removeAllWfFromBuffer(wg)
//                                req := wg.MapReq
                                for _,wf_it:=range(wg.Wfs) {
                                    if wf_it.State != wavefront.WfCompleted && wf_it.State!= wavefront.WfSampledCompleted {
                                    predicted_time := predtime + cu.wftime[wf_it.UID]
                                    nexttick := cu.Freq.NextTick(now) 
                                    if predicted_time < nexttick{
    
                                        predicted_time = nexttick
                                    }
                                    wf_it.State = wavefront.WfSampledCompleted
                                    wf_it.Sampled_level = utils.WfSampled
//                                    newEvent := wavefront.NewSampledWfCompletionEvent(predicted_time  , cu, wf_it)
//                                    cu.Engine.Schedule(newEvent)

                                    sampledrunner.Sampledtimeengine.NewRawSampledWfCompletionEvent(now,predicted_time,cu,wf_it)



                                }
                                }
                            }
//                        } else {
  //                          finished = false
    //                    }
                    } //else if wf.State == wavefront.WfAtBarrier {
//                        panic("barrier")
  //                  }

                }
            }
            if global_finished {
  //              log.Printf("finished\n")
                cu.setAllWfsPausedForSampling = true
            }
        return
        }
    } 
    if *sampledrunner.BranchSampledFlag  {
        enableSampled := sampledrunner.Branchsampledengine.EnableSampled()
//        _,enableSampled := sampledrunner.Sampledengine.Predict()
        if ! enableSampled{
            return
        }
        cu.Scheduler.StopNewCode()
        global_finished := true
        for _, wfPool := range cu.WfPools {
		    for _, wf_out := range wfPool.wfs {
                if wf_out == nil {
                    continue
                }
//                if wf_out.State == wavefront.WfCompleted {//do nothing
//                } else 
                if wf_out.State == wavefront.WfRunning { //wait until finished
//                    log.Printf("%s is running\n",wf.UID)
                    global_finished = false

                }  else if wf_out.State == wavefront.WfReady || wf_out.State==wavefront.WfAtBarrier || wf_out.State == wavefront.WfCompleted {
//                    if cu.checkInflightInst(wf) {
                        wg := wf_out.WG
                        if wf_out.State == wavefront.WfCompleted {
                            allwfscompleted := true
                            for _,wf_it := range wg.Wfs{
                                if wf_it.State == wavefront.WfCompleted || wf_it.State == wavefront.WfSampledCompleted {
                                    continue
                                } else {
                                    allwfscompleted = false
                                    break
                                }
                            }
                            if allwfscompleted {
                                continue
                            }
                        }
                        finished := true
                        for _,wf_it := range wg.Wfs{
                            if wf_it.State == wavefront.WfRunning {
                                finished = false
                                break
                                //return
                            }
                        }
                        
//                        if !finished {
//                            log.Printf("unfinished") 
//                            for _,wf_it := range wg.Wfs{
//                                log.Printf("wfid %s %d\n",wf_it.UID,wf_it.State)
//                            }
//                        } else {
//                             log.Printf("finished") 
//                            for _,wf_it := range wg.Wfs{
//                                log.Printf("wfid %s %d\n",wf_it.UID,wf_it.State)
//                            }
//                       
//                        }
                        if !finished{
                            global_finished = false
                            continue
                        }
                        cu.Scheduler.removeAllWfFromBuffer(wg)
                        req := wg.MapReq
                        predicttimes := emu.Sampledcomputeunit.RunWG(req,now)
	                    for idx, wf_it := range wg.Wfs {
                            if wf_it.State != wavefront.WfCompleted && wf_it.State != wavefront.WfSampledCompleted {
                                predicted_time :=  cu.wftime[wf_it.UID] + predicttimes[idx]
//                fmt.Printf("predict %s %10.20f\n", wf.UID, predicted_time)
                                nexttick := cu.Freq.NextTick(now) 
                                if predicted_time < nexttick{

                                    predicted_time = nexttick
                                }
                                wf_it.State = wavefront.WfSampledCompleted

                                wf_it.Sampled_level = utils.BBSampled
//                             	newEvent := wavefront.NewSampledWfCompletionEvent(
//		    	                    predicted_time  , cu, wf_it)

//                        		cu.Engine.Schedule(newEvent)

                                sampledrunner.Sampledtimeengine.NewRawSampledWfCompletionEvent(now,predicted_time,cu,wf_it)



                            }
                        }


//                    } else {
  //                      finished = false
    //                }
                } //else if wf.State == wavefront.WfAtBarrier {
//                    panic("barrier")
  //              }

            }
        }
        if global_finished {
  //          log.Printf("finished\n")
            cu.setAllWfsPausedForSampling = true
        }

    }


}



// Tick ticks
func (cu *ComputeUnit) Tick(now sim.VTimeInSec) bool {
	cu.Lock()
	defer cu.Unlock()
	madeProgress := false
    if ( *sampledrunner.SampledRunnerFlag || *sampledrunner.BranchSampledFlag) {
            cu.stopTimeModel(now)
    }

	madeProgress = cu.runPipeline(now) || madeProgress
	madeProgress = cu.sendToACE(now) || madeProgress
	madeProgress = cu.sendToCP(now) || madeProgress
	madeProgress = cu.processInput(now) || madeProgress
	madeProgress = cu.doFlush(now) || madeProgress

	return madeProgress
}

//nolint:gocyclo
func (cu *ComputeUnit) runPipeline(now sim.VTimeInSec) bool {
	madeProgress := false

	if !cu.isPaused {
		madeProgress = cu.BranchUnit.Run(now) || madeProgress
		madeProgress = cu.ScalarUnit.Run(now) || madeProgress
		madeProgress = cu.ScalarDecoder.Run(now) || madeProgress
		for _, simdUnit := range cu.SIMDUnit {
			madeProgress = simdUnit.Run(now) || madeProgress
		}
		madeProgress = cu.VectorDecoder.Run(now) || madeProgress
		madeProgress = cu.LDSUnit.Run(now) || madeProgress
		madeProgress = cu.LDSDecoder.Run(now) || madeProgress
		madeProgress = cu.VectorMemUnit.Run(now) || madeProgress
		madeProgress = cu.VectorMemDecoder.Run(now) || madeProgress
		madeProgress = cu.Scheduler.Run(now) || madeProgress
	}

	return madeProgress
}

func (cu *ComputeUnit) doFlush(now sim.VTimeInSec) bool {
	madeProgress := false
	if cu.isFlushing {
		//If a flush request arrives before the shadow buffer requests have been sent out
		if cu.isSendingOutShadowBufferReqs {
			madeProgress = cu.reInsertShadowBufferReqsToOriginalBuffers() || madeProgress
		}
		madeProgress = cu.flushPipeline(now) || madeProgress
	}

	if cu.isSendingOutShadowBufferReqs {
		madeProgress = cu.checkShadowBuffers(now) || madeProgress
	}

	return madeProgress
}

func (cu *ComputeUnit) processInput(now sim.VTimeInSec) bool {
	madeProgress := false

	if !cu.isPaused || cu.isSendingOutShadowBufferReqs {
		madeProgress = cu.processInputFromACE(now) || madeProgress
		madeProgress = cu.processInputFromInstMem(now) || madeProgress
		madeProgress = cu.processInputFromScalarMem(now) || madeProgress
		madeProgress = cu.processInputFromVectorMem(now) || madeProgress
	}

	madeProgress = cu.processInputFromCP(now) || madeProgress

	return madeProgress
}

func (cu *ComputeUnit) processInputFromCP(now sim.VTimeInSec) bool {
	req := cu.ToCP.Retrieve(now)
	if req == nil {
		return false
	}

	cu.inCPRequestProcessingStage = req
	switch req := req.(type) {
	case *protocol.CUPipelineRestartReq:
		cu.handlePipelineResume(now, req)
	case *protocol.CUPipelineFlushReq:
		cu.handlePipelineFlushReq(now, req)
	default:
		panic("unknown msg type")
	}

	return true
}

func (cu *ComputeUnit) handlePipelineFlushReq(
	now sim.VTimeInSec,
	req *protocol.CUPipelineFlushReq,
) error {
	cu.isFlushing = true
	cu.currentFlushReq = req

	return nil
}

func (cu *ComputeUnit) handlePipelineResume(
	now sim.VTimeInSec,
	req *protocol.CUPipelineRestartReq,
) error {
	cu.isSendingOutShadowBufferReqs = true
	cu.currentRestartReq = req

	rsp := protocol.CUPipelineRestartRspBuilder{}.
		WithSrc(cu.ToCP).
		WithDst(cu.currentRestartReq.Src).
		WithSendTime(now).
		Build()
	err := cu.ToCP.Send(rsp)

	if err != nil {
		cu.currentRestartReq = nil
		log.Panicf("Unable to send restart rsp to CP")
	}
	return nil
}

func (cu *ComputeUnit) sendToCP(now sim.VTimeInSec) bool {
	if cu.toSendToCP == nil {
		return false
	}

	cu.toSendToCP.Meta().SendTime = now
	sendErr := cu.ToCP.Send(cu.toSendToCP)
	if sendErr == nil {
		cu.toSendToCP = nil
		return true
	}

	return false
}

func (cu *ComputeUnit) sendToACE(now sim.VTimeInSec) bool {
	return cu.toACESender.Tick(now)
}

func (cu *ComputeUnit) flushPipeline(now sim.VTimeInSec) bool {
	if cu.currentFlushReq == nil {
		return false
	}

	if cu.isHandlingWfCompletionEvent == true {
		return false
	}

	cu.shadowInFlightInstFetch = nil
	cu.shadowInFlightScalarMemAccess = nil
	cu.shadowInFlightVectorMemAccess = nil

	cu.populateShadowBuffers()
	cu.setWavesToReady()
	cu.Scheduler.Flush()
	cu.flushInternalComponents()
	cu.Scheduler.Pause()
	cu.isPaused = true

	respondToCP := protocol.CUPipelineFlushRspBuilder{}.
		WithSendTime(now).
		WithSrc(cu.ToCP).
		WithDst(cu.currentFlushReq.Src).
		Build()
	cu.toSendToCP = respondToCP
	cu.currentFlushReq = nil
	cu.isFlushing = false

	return true
}

func (cu *ComputeUnit) flushInternalComponents() {
	cu.BranchUnit.Flush()

	cu.ScalarUnit.Flush()
	cu.ScalarDecoder.Flush()

	for _, simdUnit := range cu.SIMDUnit {
		simdUnit.Flush()
	}

	cu.VectorDecoder.Flush()
	cu.LDSUnit.Flush()
	cu.LDSDecoder.Flush()
	cu.VectorMemDecoder.Flush()
	cu.VectorMemUnit.Flush()
}

func (cu *ComputeUnit) processInputFromACE(now sim.VTimeInSec) bool {
	req := cu.ToACE.Retrieve(now)
	if req == nil {
		return false
	}

	switch req := req.(type) {
	case *protocol.MapWGReq:
		return cu.handleMapWGReq(now, req)
	default:
		panic("unknown req type")
	}
}
func (v *ComputeUnit) DebugPrint(now sim.VTimeInSec) {
    return;
    fmt.Printf( "%s %.18f\n",v.Name(), now )
}


func (cu *ComputeUnit) handleMapWGReq(
	now sim.VTimeInSec,
	req *protocol.MapWGReq,
) bool {
	wg := cu.wrapWG(req.WorkGroup, req)

	tracing.TraceReqReceive(req, cu)

	cu.logWGFeatureTask(now, wg, 0)
    skip_simulate := false

    if * profiler.CollectDataApplication || *sampledrunner.SampledRunnerFlag || *sampledrunner.BranchSampledFlag || *profiler.WfProfilingFlag || *sampledrunner.KernelSampledFlag{
    	for _, wf := range wg.Wfs {

            cu.wftime[ wf.UID ] = now
        }
    }
    if (*sampledrunner.KernelSampledFlag ) || ( *sampledrunner.SampledRunnerFlag ){

        //to process some special cases; sometimes wavefront in detailed mode send to this cu too late;
        wfsampled := false
        var wfpredicttime sim.VTimeInSec
        if *sampledrunner.SampledRunnerFlag {
            wfpredicttime, wfsampled = sampledrunner.Sampledengine.Predict()
        }

	    for _, wf := range wg.Wfs {

            predtime := wf.Predtime
//            if skip_simulate && (!wf.Skip) {
//                panic("wavefront in one group should be the same")
//            }
            skip_simulate = wf.Skip
            if wfsampled && (!skip_simulate) {
                skip_simulate = true
                predtime = wfpredicttime
            }

            if  skip_simulate {
                predicted_time := predtime + now
                wf.State = wavefront.WfSampledCompleted
                wf.Sampled_level = utils.WfSampled
//                newEvent := wavefront.NewSampledWfCompletionEvent(predicted_time  , cu, wf)
//            	cu.Engine.Schedule(newEvent)
                sampledrunner.Sampledtimeengine.NewRawSampledWfCompletionEvent(now,predicted_time,cu,wf)
                if *profiler.WfProfilingFlag {
                    profiler.Wffinalfeature.CollectWfStart(wf.UID,now)
                }

	    	    tracing.StartTask(wf.UID,
	    	    	tracing.MsgIDAtReceiver(req, cu),
	    	    	cu,
	    	    	"wavefront",
	    	    	"wavefront",
	    	    	nil,
	    	    )
            }
	    }
        if skip_simulate {
            return true
        }

    }







    if *sampledrunner.BranchSampledFlag  {
        enable_samped := sampledrunner.Branchsampledengine.EnableSampled()
        if enable_samped {
            predicttimes := emu.Sampledcomputeunit.RunWG(req,now)
        
	        for idx, wf := range wg.Wfs {

                predicted_time :=  now + predicttimes[idx] 
//                fmt.Printf("predict %s %10.20f\n", wf.UID, predicted_time)
                wf.State = wavefront.WfSampledCompleted
                wf.Sampled_level = utils.BBSampled
//             	newEvent := wavefront.NewSampledWfCompletionEvent(
//			    predicted_time  , cu, wf)
        		//cu.Engine.Schedule(newEvent)
                sampledrunner.Sampledtimeengine.NewRawSampledWfCompletionEvent(now,predicted_time,cu,wf)
                if *profiler.WfProfilingFlag {
                    profiler.Wffinalfeature.CollectWfStart(wf.UID,now)
                }

    	    	tracing.StartTask(wf.UID,
	    	    	tracing.MsgIDAtReceiver(req, cu),
		    	    cu,
    			    "wavefront",
        			"wavefront",
	    	    	nil,
        		)

            }
            return true
        }
    }

	for i, wf := range wg.Wfs {

        
    	location := req.Wavefronts[i]
	    cu.WfPools[location.SIMDID].AddWf(wf)

		cu.WfDispatcher.DispatchWf(now, wf, req.Wavefronts[i])
		wf.State = wavefront.WfReady
        if *sampledrunner.BranchSampledFlag  {
            sampledrunner.Branchsampledengine.CollectWfStart(wf.UID,now)
        }
        if *profiler.WfProfilingFlag {
            profiler.Wffinalfeature.CollectWfStart(wf.UID,now)
        }


		tracing.StartTask(wf.UID,
			tracing.MsgIDAtReceiver(req, cu),
			cu,
			"wavefront",
			"wavefront",
			nil,
		)
	}
    cu.running = true
	cu.TickLater(now)
	return true
}

func (cu *ComputeUnit) handleWfCompletionEvent(evt *wavefront.WfCompletionEvent) error {
	wf := evt.Wf
	wg := wf.WG

	tracing.EndTask(wf.UID, cu)
    
    wf.State = wavefront.WfCompleted
    issuetime , found := cu.wftime [wf.UID]
    if found {
        finishtime := evt.Time()
        if *profiler.CollectDataApplication { 
            profiler.Datafeature.AddData(cu.Id , issuetime, finishtime)
        }
        if *sampledrunner.BranchSampledFlag {
            sampledrunner.Branchsampledengine.CollectWfEnd(wf.UID,finishtime)
        }
        if *profiler.WfProfilingFlag {
            profiler.Wffinalfeature.CollectWfEnd(wf.UID,finishtime,wf.Sampled_level)
        }

        wf.Finishtime = finishtime
        wf.Issuetime = issuetime
//        if *sampledrunner.SampledRunnerFlag {
//            sampledrunner.Sampledengine.Collect(issuetime,finishtime)
//        }
        delete( cu.wftime, wf.UID )
    }

	if cu.isAllWfInWGCompleted(wg) {

		cu.isHandlingWfCompletionEvent = true

		ok := cu.sendWGCompletionMessage(evt, wg)
		if ok {
			cu.clearWGResource(wg)
			tracing.TraceReqComplete(wg.MapReq, cu)
		}else {
        
//            fmt.Printf( "wf %s end time:  %.18f\n",  wf.UID,evt.Time() )
        }
//        _,enable := sampledrunner.Sampledengine.Predict()
//        if  enable {
//            fmt.Printf( "wf %s end time:  %.18f\n",  wf.UID,evt.Time() )
//        }

		if !cu.hasMoreWfsToRun() {
			cu.running = false
		}
	}

	cu.TickLater(evt.Time())

	return nil
}

func (cu *ComputeUnit) clearWGResource(wg *wavefront.WorkGroup) {
	for _, wf := range wg.Wfs {
		wfPool := cu.WfPools[wf.SIMDID]
		wfPool.RemoveWf(wf)
	}
}

func (cu *ComputeUnit) isAllWfInWGCompleted(wg *wavefront.WorkGroup) bool {
	for _, wf := range wg.Wfs {
		if wf.State != wavefront.WfCompleted {
			return false
		}
	}
	return true
}

func (cu *ComputeUnit) sendWGCompletionMessage(
	evt *wavefront.WfCompletionEvent,
	wg *wavefront.WorkGroup,
) bool {
	now := evt.Time()
	mapReq := wg.MapReq
	dispatcher := mapReq.Src

	msg := protocol.WGCompletionMsgBuilder{}.
		WithSendTime(now).
		WithSrc(cu.ToACE).
		WithDst(dispatcher).
		WithRspTo(mapReq.ID).
		Build()

	err := cu.ToACE.Send(msg)
	if err != nil {
		newEvent := wavefront.NewWfCompletionEvent(
			cu.Freq.NextTick(now), cu, evt.Wf)
		cu.Engine.Schedule(newEvent)
		return false
	}

	tracing.TraceReqComplete(mapReq, cu)

	cu.logWGFeatureTask(now, wg, 2)
	cu.isHandlingWfCompletionEvent = false
	return true
}

func (cu *ComputeUnit) hasMoreWfsToRun() bool {
	for _, wfpool := range cu.WfPools {
		if len(wfpool.wfs) > 0 {
			return true
		}
	}
	return false
}

func (cu *ComputeUnit) wrapWG(
	raw *kernels.WorkGroup,
	req *protocol.MapWGReq,
) *wavefront.WorkGroup {
	wg := wavefront.NewWorkGroup(raw, req)

	lds := make([]byte, req.WorkGroup.Packet.GroupSegmentSize)
	wg.LDS = lds

	for _, rawWf := range req.WorkGroup.Wavefronts {
		wf := wavefront.NewWavefront(rawWf)
		wg.Wfs = append(wg.Wfs, wf)
		wf.WG = wg
		wf.SetPID(req.PID)
	}

	return wg
}

func (cu *ComputeUnit) processInputFromInstMem(now sim.VTimeInSec) bool {
	rsp := cu.ToInstMem.Retrieve(now)
	if rsp == nil {
		return false
	}

	switch rsp := rsp.(type) {
	case *mem.DataReadyRsp:
		cu.handleFetchReturn(now, rsp)
	default:
		log.Panicf("cannot handle request of type %s from ToInstMem port",
			reflect.TypeOf(rsp))
	}
	return true
}

func (cu *ComputeUnit) handleFetchReturn(
	now sim.VTimeInSec,
	rsp *mem.DataReadyRsp,
) bool {
	if len(cu.InFlightInstFetch) == 0 {
		return false
	}

	info := cu.InFlightInstFetch[0]
	if info.Req.ID != rsp.RespondTo {
		return false
	}

	wf := info.Wavefront
	addr := info.Address
	cu.InFlightInstFetch = cu.InFlightInstFetch[1:]

	if addr == wf.InstBufferStartPC+uint64(len(wf.InstBuffer)) {
		wf.InstBuffer = append(wf.InstBuffer, rsp.Data...)
	}

	wf.IsFetching = false
	wf.LastFetchTime = now

	tracing.TraceReqFinalize(info.Req, cu)
	tracing.EndTask(info.Req.ID+"_fetch", cu)
	return true
}

func (cu *ComputeUnit) processInputFromScalarMem(now sim.VTimeInSec) bool {
	rsp := cu.ToScalarMem.Retrieve(now)
	if rsp == nil {
		return false
	}

	switch rsp := rsp.(type) {
	case *mem.DataReadyRsp:
		cu.handleScalarDataLoadReturn(now, rsp)
	default:
		log.Panicf("cannot handle request of type %s from ToInstMem port",
			reflect.TypeOf(rsp))
	}
	return true
}

func (cu *ComputeUnit) handleScalarDataLoadReturn(
	now sim.VTimeInSec,
	rsp *mem.DataReadyRsp,
) {
	if len(cu.InFlightScalarMemAccess) == 0 {
		return
	}

	info := cu.InFlightScalarMemAccess[0]
	req := info.Req
	if req.ID != rsp.RespondTo {
		return
	}

	wf := info.Wavefront
	access := RegisterAccess{
		WaveOffset: wf.SRegOffset,
		Reg:        info.DstSGPR,
		RegCount:   len(rsp.Data) / 4,
		Data:       rsp.Data,
	}
	cu.SRegFile.Write(access)

	cu.InFlightScalarMemAccess = cu.InFlightScalarMemAccess[1:]

	cu.logInstTask(now, wf, info.Inst, true)
	tracing.TraceReqFinalize(req, cu)

	if cu.isLastRead(req) {
		wf.OutstandingScalarMemAccess--
	}
}

func (cu *ComputeUnit) isLastRead(req *mem.ReadReq) bool {
	return !req.CanWaitForCoalesce
}

func (cu *ComputeUnit) processInputFromVectorMem(now sim.VTimeInSec) bool {
	rsp := cu.ToVectorMem.Retrieve(now)
	if rsp == nil {
		return false
	}

	switch rsp := rsp.(type) {
	case *mem.DataReadyRsp:
		cu.handleVectorDataLoadReturn(now, rsp)
	case *mem.WriteDoneRsp:
		cu.handleVectorDataStoreRsp(now, rsp)
	default:
		log.Panicf("cannot handle request of type %s from ToInstMem port",
			reflect.TypeOf(rsp))
	}

	return true
}

//nolint:gocyclo
func (cu *ComputeUnit) handleVectorDataLoadReturn(
	now sim.VTimeInSec,
	rsp *mem.DataReadyRsp,
) {
	if len(cu.InFlightVectorMemAccess) == 0 {
		return
	}

	info := cu.InFlightVectorMemAccess[0]

	if info.Read == nil {
		return
	}

	if info.Read.ID != rsp.RespondTo {
		return
	}

	cu.InFlightVectorMemAccess = cu.InFlightVectorMemAccess[1:]
	tracing.TraceReqFinalize(info.Read, cu)

	wf := info.Wavefront
	inst := info.Inst

	for _, laneInfo := range info.laneInfo {
		offset := laneInfo.addrOffsetInCacheLine
		access := RegisterAccess{}
		access.WaveOffset = wf.VRegOffset
		access.Reg = laneInfo.reg
		access.RegCount = laneInfo.regCount
		access.LaneID = laneInfo.laneID
		if inst.FormatType == insts.FLAT && inst.Opcode == 16 { // FLAT_LOAD_UBYTE
			access.Data = insts.Uint32ToBytes(uint32(rsp.Data[offset]))
		} else if inst.FormatType == insts.FLAT && inst.Opcode == 18 {
			access.Data = insts.Uint32ToBytes(uint32(rsp.Data[offset]))
		} else {
			access.Data = rsp.Data[offset : offset+uint64(4*laneInfo.regCount)]
		}
		cu.VRegFile[wf.SIMDID].Write(access)
	}

	if !info.Read.CanWaitForCoalesce {
		wf.OutstandingVectorMemAccess--
		if info.Inst.FormatType == insts.FLAT {
			wf.OutstandingScalarMemAccess--
		}

		cu.logInstTask(now, wf, info.Inst, true)
	}
}

func (cu *ComputeUnit) handleVectorDataStoreRsp(
	now sim.VTimeInSec,
	rsp *mem.WriteDoneRsp,
) {
	if len(cu.InFlightVectorMemAccess) == 0 {
		return
	}

	info := cu.InFlightVectorMemAccess[0]

	if info.Write == nil {
		return
	}

	if info.Write.ID != rsp.RespondTo {
		return
	}

	cu.InFlightVectorMemAccess = cu.InFlightVectorMemAccess[1:]
	tracing.TraceReqFinalize(info.Write, cu)

	wf := info.Wavefront
	if !info.Write.CanWaitForCoalesce {
		wf.OutstandingVectorMemAccess--
		if info.Inst.FormatType == insts.FLAT {
			wf.OutstandingScalarMemAccess--
		}
		cu.logInstTask(now, wf, info.Inst, true)
	}
}

// UpdatePCAndSetReady is self explained
func (cu *ComputeUnit) UpdatePCAndSetReady(wf *wavefront.Wavefront) {
	wf.State = wavefront.WfReady
	wf.PC += uint64(wf.Inst().ByteSize)
	cu.removeStaleInstBuffer(wf)
}

func (cu *ComputeUnit) removeStaleInstBuffer(wf *wavefront.Wavefront) {
	if len(wf.InstBuffer) != 0 {
		for wf.PC >= wf.InstBufferStartPC+64 {
			wf.InstBuffer = wf.InstBuffer[64:]
			wf.InstBufferStartPC += 64
		}
	}
}

func (cu *ComputeUnit) flushCUBuffers() {
	cu.InFlightInstFetch = nil
	cu.InFlightScalarMemAccess = nil
	cu.InFlightVectorMemAccess = nil
}
func (cu *ComputeUnit) logWGFeatureTask(
	now sim.VTimeInSec,
    wg *wavefront.WorkGroup,
	types uint32,
) {
	if types == 2 { //2 means finish
		tracing.EndTask2(
            wg.UID,
			cu,
            map[string]interface{}{
                "now": now,
            },
		)
		return
	} else if types == 1 { // 1 means step
        tracing.AddTaskStep(
            wg.UID,
            cu,
            "no thing",
        )
        return
    } else if types == 0 { // 0 means start
	tracing.StartTask(
		wg.UID,
		"0",
		cu,
		"wg_feature",
		"wavegroup",
		// inst.InstName,
		map[string]interface{}{
            "now":now,
			"wg":   wg,
            "cu":   cu,
		},
	)
    }
}


func (cu *ComputeUnit) logInstTask(
	now sim.VTimeInSec,
	wf *wavefront.Wavefront,
	inst *wavefront.Inst,
	completed bool,
) {
	if completed {
		tracing.EndTask(inst.ID, cu)
        //sampling
        if *sampledrunner.IPCSampledRunnerFlag || *profiler.BranchSampled || *sampledrunner.BranchSampledFlag || *profiler.ReportIPCFlag {
    	    _, found := cu.inflightInst[inst.ID] //to avoid count one instruction multi-times
    	    if !found {
	    	    return
    	    }
        
	        delete(cu.inflightInst, inst.ID)
            insts_num := inst.InstWidth()
            if *profiler.ReportIPCFlag {
                profiler.Instcount.Count( now, insts_num )
            }
            if *sampledrunner.IPCSampledRunnerFlag {
                sampledrunner.IPC_sampled_engine.Collect( now, insts_num, )
            }
            
            if *sampledrunner.BranchSampledFlag {
                profiler.Global_inst_feature.InstRetired( wf.UID, now,inst.Inst )
            }


            if (*sampledrunner.BranchSampledFlag) &&(inst.FormatType == insts.SOPP && inst.Opcode == 1) { // endpgm
                sampledrunner.Branchsampledengine.Collect( wf.UID,now,inst.Inst,wf )
            }
            if *profiler.BranchSampled {
                if (inst.FormatType == insts.SOPP && inst.Opcode == 1) { //not endpgm
                    profiler.Branchfeature.Collect( wf.UID, now, inst.Inst,wf )
                }
            }
        }
        
		return
	}
    if *profiler.ReportIPCFlag {
        insts_num := inst.InstWidth()
        profiler.Instcount.CountIssue( now, insts_num )
    }

    if  *sampledrunner.BranchSampledFlag {
        profiler.Global_inst_feature.InstIssue( wf.UID, now,inst.Inst )
    }
    if *sampledrunner.IPCSampledRunnerFlag || *profiler.BranchSampled || *sampledrunner.BranchSampledFlag || *profiler.ReportIPCFlag {
   // if *sampledrunner.IPCSampledRunnerFlag || *profiler.BranchSampled {
        cu.inflightInst[inst.ID] = 1
    }
    if *sampledrunner.BranchSampledFlag {
        if !(inst.FormatType == insts.SOPP && inst.Opcode == 1) { //not endpgm
            sampledrunner.Branchsampledengine.Collect( wf.UID,now,inst.Inst,wf )
        }
    }
    if *profiler.BranchSampled {
        if !(inst.FormatType == insts.SOPP && inst.Opcode == 1) { //not endpgm
            profiler.Branchfeature.Collect( wf.UID, now, inst.Inst,wf )
        }
    }
    if *profiler.WfProfilingFlag {
        profiler.Wffinalfeature.Collect( wf.UID, now, inst.Inst )
    }

	tracing.StartTask(
		inst.ID,
		wf.UID,
		cu,
		"inst",
		cu.execUnitToString(inst.ExeUnit),
		// inst.InstName,
		map[string]interface{}{
			"inst": inst,
			"wf":   wf,
		},
	)
}

func (cu *ComputeUnit) execUnitToString(u insts.ExeUnit) string {
	switch u {
	case insts.ExeUnitVALU:
		return "VALU"
	case insts.ExeUnitScalar:
		return "Scalar"
	case insts.ExeUnitVMem:
		return "VMem"
	case insts.ExeUnitBranch:
		return "Branch"
	case insts.ExeUnitLDS:
		return "LDS"
	case insts.ExeUnitGDS:
		return "GDS"
	case insts.ExeUnitSpecial:
		return "Special"
	}
	panic("unknown exec unit")
}

func (cu *ComputeUnit) reInsertShadowBufferReqsToOriginalBuffers() bool {
	cu.isSendingOutShadowBufferReqs = false
	for i := 0; i < len(cu.shadowInFlightVectorMemAccess); i++ {
		cu.InFlightVectorMemAccess = append(cu.InFlightVectorMemAccess, cu.shadowInFlightVectorMemAccess[i])
	}

	for i := 0; i < len(cu.shadowInFlightScalarMemAccess); i++ {
		cu.InFlightScalarMemAccess = append(cu.InFlightScalarMemAccess, cu.shadowInFlightScalarMemAccess[i])
	}

	for i := 0; i < len(cu.shadowInFlightInstFetch); i++ {
		cu.InFlightInstFetch = append(cu.InFlightInstFetch, cu.shadowInFlightInstFetch[i])
	}

	return true
}

func (cu *ComputeUnit) checkShadowBuffers(now sim.VTimeInSec) bool {
	numReqsPendingToSend :=
		len(cu.shadowInFlightScalarMemAccess) +
			len(cu.shadowInFlightVectorMemAccess) +
			len(cu.shadowInFlightInstFetch)

	if numReqsPendingToSend == 0 {
		cu.isSendingOutShadowBufferReqs = false
		cu.Scheduler.Resume()
		cu.isPaused = false
		return true
	}

	return cu.sendOutShadowBufferReqs(now)
}

func (cu *ComputeUnit) sendOutShadowBufferReqs(now sim.VTimeInSec) bool {
	madeProgress := false

	madeProgress = cu.sendScalarShadowBufferAccesses(now) || madeProgress
	madeProgress = cu.sendVectorShadowBufferAccesses(now) || madeProgress
	madeProgress = cu.sendInstFetchShadowBufferAccesses(now) || madeProgress

	return madeProgress
}

func (cu *ComputeUnit) sendScalarShadowBufferAccesses(
	now sim.VTimeInSec,
) bool {
	if len(cu.shadowInFlightScalarMemAccess) > 0 {
		info := cu.shadowInFlightScalarMemAccess[0]

		req := info.Req
		req.ID = xid.New().String()
		req.SendTime = now
		err := cu.ToScalarMem.Send(req)
		if err == nil {
			cu.InFlightScalarMemAccess =
				append(cu.InFlightScalarMemAccess, info)
			cu.shadowInFlightScalarMemAccess =
				cu.shadowInFlightScalarMemAccess[1:]
			return true
		}
	}

	return false
}

func (cu *ComputeUnit) sendVectorShadowBufferAccesses(
	now sim.VTimeInSec,
) bool {
	if len(cu.shadowInFlightVectorMemAccess) > 0 {
		info := cu.shadowInFlightVectorMemAccess[0]
		if info.Read != nil {
			req := info.Read
			req.ID = sim.GetIDGenerator().Generate()
			req.SendTime = now
			err := cu.ToVectorMem.Send(req)
			if err == nil {
				cu.InFlightVectorMemAccess = append(
					cu.InFlightVectorMemAccess, info)
				cu.shadowInFlightVectorMemAccess = cu.shadowInFlightVectorMemAccess[1:]
				return true
			}
		} else if info.Write != nil {
			req := info.Write
			req.ID = sim.GetIDGenerator().Generate()
			req.SendTime = now
			err := cu.ToVectorMem.Send(req)
			if err == nil {
				cu.InFlightVectorMemAccess = append(cu.InFlightVectorMemAccess, info)
				cu.shadowInFlightVectorMemAccess = cu.shadowInFlightVectorMemAccess[1:]
				return true
			}
		}
	}
	return false
}

func (cu *ComputeUnit) sendInstFetchShadowBufferAccesses(
	now sim.VTimeInSec,
) bool {
	if len(cu.shadowInFlightInstFetch) > 0 {
		info := cu.shadowInFlightInstFetch[0]
		req := info.Req
		req.ID = xid.New().String()
		req.SendTime = now
		err := cu.ToInstMem.Send(req)
		if err == nil {
			cu.InFlightInstFetch = append(cu.InFlightInstFetch, info)
			cu.shadowInFlightInstFetch = cu.shadowInFlightInstFetch[1:]
			return true
		}
	}
	return false
}
func (cu *ComputeUnit) populateShadowBuffers() {
	for i := 0; i < len(cu.InFlightInstFetch); i++ {
		cu.shadowInFlightInstFetch = append(cu.shadowInFlightInstFetch, cu.InFlightInstFetch[i])
	}

	for i := 0; i < len(cu.InFlightScalarMemAccess); i++ {
		cu.shadowInFlightScalarMemAccess = append(cu.shadowInFlightScalarMemAccess, cu.InFlightScalarMemAccess[i])
	}

	for i := 0; i < len(cu.InFlightVectorMemAccess); i++ {
		cu.shadowInFlightVectorMemAccess = append(cu.shadowInFlightVectorMemAccess, cu.InFlightVectorMemAccess[i])
	}

	cu.InFlightScalarMemAccess = nil
	cu.InFlightInstFetch = nil
	cu.InFlightVectorMemAccess = nil
}

func (cu *ComputeUnit) GetEngine() sim.Engine{
    return cu.Engine
}
func (cu *ComputeUnit) setWavesToReady() {
	for _, wfPool := range cu.WfPools {
		for _, wf := range wfPool.wfs {
			if wf.State != wavefront.WfCompleted {
				wf.State = wavefront.WfReady
				wf.IsFetching = false
			}
		}
	}
}

// NewComputeUnit returns a newly constructed compute unit
func NewComputeUnit(
	name string,
	engine sim.Engine,
) *ComputeUnit {
	cu := new(ComputeUnit)
    cu.inflightInst = make(map[string]int) //for inst sampling
    cu.wftime = make(map[string]sim.VTimeInSec)
	cu.TickingComponent = sim.NewTickingComponent(
		name, engine, 1*sim.GHz, cu)

	cu.ToACE = sim.NewLimitNumMsgPort(cu, 4, name+".ToACE")
	cu.toACESender = sim.NewBufferedSender(
		cu.ToACE, sim.NewBuffer(cu.Name()+".ToACESenderBuffer", 40960000))
	cu.ToInstMem = sim.NewLimitNumMsgPort(cu, 4, name+".ToInstMem")
	cu.ToScalarMem = sim.NewLimitNumMsgPort(cu, 4, name+".ToScalarMem")
	cu.ToVectorMem = sim.NewLimitNumMsgPort(cu, 4, name+".ToVectorMem")
	cu.ToCP = sim.NewLimitNumMsgPort(cu, 4, name+".ToCP")

	return cu
}
