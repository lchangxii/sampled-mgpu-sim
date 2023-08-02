package cu

import (
	"log"

	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mgpusim/v3/insts"
	"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"
    //	"gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mgpusim/v3/timing/wavefront"
)

// Scheduler does its job
type Scheduler interface {
	Run(now sim.VTimeInSec) bool
	Pause()
	Resume()
	Flush()
    StopNewCode()
    StartNewCode()
    removeAllWfFromBuffer(wg *wavefront.WorkGroup) 
}

// SchedulerImpl implements scheduler
// A Scheduler is the controlling unit of a compute unit. It decides which
// wavefront to fetch and to issue.
type SchedulerImpl struct {
	cu                *ComputeUnit
	fetchArbiter      WfArbiter
	issueArbiter      WfArbiter
	internalExecuting []*wavefront.Wavefront

	barrierBuffer     []*wavefront.Wavefront
	barrierBufferSize int

	cyclesNoProgress                  int
	stopTickingAfterNCyclesNoProgress int
    stopNewCode bool
	isPaused bool
}

// NewScheduler returns a newly created scheduler, injecting dependency
// of the compute unit, the fetch arbiter, and the issue arbiter.
func NewScheduler(
	cu *ComputeUnit,
	fetchArbiter WfArbiter,
	issueArbiter WfArbiter,
) *SchedulerImpl {
	s := new(SchedulerImpl)
	s.cu = cu
	s.fetchArbiter = fetchArbiter
	s.issueArbiter = issueArbiter
    s.stopNewCode = false
	s.barrierBufferSize = 16
	s.barrierBuffer = make([]*wavefront.Wavefront, 0, s.barrierBufferSize)

	s.stopTickingAfterNCyclesNoProgress = 4

	return s
}
func (s *SchedulerImpl) releaseWGSampledSim( wg *wavefront.WorkGroup ,now sim.VTimeInSec,interval sim.VTimeInSec)  {
 //   log.Println("release")
    for _,wf := range( wg.Wfs  ){
    	wf.State = wavefront.WfCompleted
        id := wf.UID
        issuetime := s.cu.wftime[id] 
        finishtime := issuetime + interval
        if finishtime <= now {
            finishtime = s.cu.Freq.NextTick(now)
        }
	    wfCompletionEvt := wavefront.NewWfCompletionEvent(finishtime, s.cu, wf)
    	s.cu.Engine.Schedule(wfCompletionEvt)
//	    s.resetRegisterValue(wf)
    } 
}

// Run runs scheduler
func (s *SchedulerImpl) Run(now sim.VTimeInSec) bool {

	madeProgress := false
	if s.isPaused == false {
		madeProgress = s.EvaluateInternalInst(now) || madeProgress
		madeProgress = s.DecodeNextInst(now) || madeProgress
		madeProgress = s.DoIssue(now) || madeProgress
		madeProgress = s.DoFetch(now) || madeProgress
	}
    
	if !madeProgress {
		s.cyclesNoProgress++
	} else {
		s.cyclesNoProgress = 0
	}

	if s.cyclesNoProgress > s.stopTickingAfterNCyclesNoProgress {
		return false
	}
	return true
}

//DecodeNextInst checks
func (s *SchedulerImpl) DecodeNextInst(now sim.VTimeInSec) bool {
	madeProgress := false
	for _, wfPool := range s.cu.WfPools {
		for _, wf := range wfPool.wfs {
			if len(wf.InstBuffer) == 0 {
				wf.InstBufferStartPC = wf.PC & 0xffffffffffffffc0
				continue
			}

			if wf.State != wavefront.WfReady {
				continue
			}

			if wf.InstToIssue != nil {
				continue
			}

			if !s.wfHasAtLeast8BytesInInstBuffer(wf) {
				continue
			}

  //          log.Printf("wfid %s pc %d\n",wf.UID,wf.PC)
			inst, err := s.cu.Decoder.Decode(
				wf.InstBuffer[wf.PC-wf.InstBufferStartPC:])

//            log.Printf("wfid %s pc %d\n",wf.UID,wf.PC)
	        inst.PC = wf.PC
			if err == nil {
				wf.InstToIssue = wavefront.NewInst(inst)
				// s.cu.logInstTask(now, wf, wf.InstToIssue, false)
				madeProgress = true
			}
		}
	}
	return madeProgress
}

func (s *SchedulerImpl) wfHasAtLeast8BytesInInstBuffer(wf *wavefront.Wavefront) bool {
	return len(wf.InstBuffer[wf.PC-wf.InstBufferStartPC:]) >= 8
}

// DoFetch function of the scheduler will fetch instructions from the
// instruction memory
func (s *SchedulerImpl) DoFetch(now sim.VTimeInSec) bool {
	madeProgress := false
	wfs := s.fetchArbiter.Arbitrate(s.cu.WfPools)
	if len(wfs) > 0 {
		wf := wfs[0]

        //log.Printf("wfid %s pc %d\n",wf.UID,wf.PC)
		if len(wf.InstBuffer) == 0 {
			wf.InstBufferStartPC = wf.PC & 0xffffffffffffffc0
		}
		addr := wf.InstBufferStartPC + uint64(len(wf.InstBuffer))
		addr = addr & 0xffffffffffffffc0
		req := mem.ReadReqBuilder{}.
			WithSendTime(now).
			WithSrc(s.cu.ToInstMem).
			WithDst(s.cu.InstMem).
			WithAddress(addr).
			WithPID(wf.PID()).
			WithByteSize(64).
			Build()

		err := s.cu.ToInstMem.Send(req)
		if err == nil {
			info := new(InstFetchReqInfo)
			info.Wavefront = wf
			info.Req = req
			info.Address = addr
			s.cu.InFlightInstFetch = append(s.cu.InFlightInstFetch, info)
			wf.IsFetching = true

			madeProgress = true

			tracing.StartTask(req.ID+"_fetch", wf.UID,
				s.cu, "fetch", "fetch", nil)
			tracing.TraceReqInitiate(req, s.cu, req.ID+"_fetch")
		}
	}

	return madeProgress
}

// DoIssue function of the scheduler issues fetched instruction to the decoding
// units
func (s *SchedulerImpl) DoIssue(now sim.VTimeInSec) bool {
	madeProgress := false

	if s.isPaused == false && s.stopNewCode == false {
		wfs := s.issueArbiter.Arbitrate(s.cu.WfPools)
		for _, wf := range wfs {
			if wf.InstToIssue.ExeUnit == insts.ExeUnitSpecial {
				madeProgress = s.issueToInternal(wf, now) || madeProgress

				continue
			}

			unit := s.getUnitToIssueTo(wf.InstToIssue.ExeUnit)
			if unit.CanAcceptWave() {
				wf.SetDynamicInst(wf.InstToIssue)
				wf.InstToIssue = nil

				s.cu.logInstTask(now, wf, wf.DynamicInst(), false)

				unit.AcceptWave(wf, now)
				wf.State = wavefront.WfRunning
				//s.removeStaleInstBuffer(wf)

				madeProgress = true
			}
		}
	}
	return madeProgress
}

func (s *SchedulerImpl) issueToInternal(wf *wavefront.Wavefront, now sim.VTimeInSec) bool {
	wf.SetDynamicInst(wf.InstToIssue)
	wf.InstToIssue = nil
	s.internalExecuting = append(s.internalExecuting, wf)
	wf.State = wavefront.WfRunning
	//s.removeStaleInstBuffer(wf)

	s.cu.logInstTask(now, wf, wf.DynamicInst(), false)

	return true
}

func (s *SchedulerImpl) getUnitToIssueTo(u insts.ExeUnit) SubComponent {
	switch u {
	case insts.ExeUnitBranch:
		return s.cu.BranchUnit
	case insts.ExeUnitLDS:
		return s.cu.LDSDecoder
	case insts.ExeUnitVALU:
		return s.cu.VectorDecoder
	case insts.ExeUnitVMem:
		return s.cu.VectorMemDecoder
	case insts.ExeUnitScalar:
		return s.cu.ScalarDecoder
	default:
		log.Panic("not sure where to dispatch the instruction")
	}
	return nil
}

// EvaluateInternalInst updates the status of the instruction being executed
// in the scheduler.



func (s *SchedulerImpl) EvaluateInternalInst(now sim.VTimeInSec) bool {
	if s.internalExecuting == nil {
		return false
	}

	madeProgress := false

	newExecuting := make([]*wavefront.Wavefront, 0)
	for _, executing := range s.internalExecuting {
		instProgress := false
		instCompleted := false
		switch executing.Inst().Opcode {
		case 1: // S_ENDPGM
			instProgress, instCompleted = s.evalSEndPgm(executing, now)
		case 10: // S_BARRIER
			instProgress, instCompleted = s.evalSBarrier(executing, now)
		case 12: // S_WAITCNT
			instProgress, instCompleted = s.evalSWaitCnt(executing, now)
		default:
			// The program has to make progress
			executing.State = wavefront.WfReady
			instProgress = true
			instCompleted = true
		}
		madeProgress = instProgress || madeProgress

		if instCompleted { // Global IPC
			s.cu.logInstTask(now, executing, executing.DynamicInst(), true)

//            s.stopTimeModel(executing,now)
		} else {
			newExecuting = append(newExecuting, executing)
		}
	}

	s.internalExecuting = newExecuting

	return madeProgress
}

func (s *SchedulerImpl) evalSEndPgm(
	wf *wavefront.Wavefront,
	now sim.VTimeInSec,
) (madeProgress bool, instCompleted bool) {
	if wf.OutstandingVectorMemAccess > 0 ||
		wf.OutstandingScalarMemAccess > 0 {
		return false, false
	}
    
	wf.State = wavefront.WfCompleted
	wfCompletionEvt := wavefront.NewWfCompletionEvent(s.cu.Freq.NextTick(now), s.cu, wf)
    sampledrunner.Sampledtimeengine.IncreaseIdx()
	s.cu.Engine.Schedule(wfCompletionEvt)
	s.internalExecuting = nil

	s.resetRegisterValue(wf)
	return true, true
}

func (s *SchedulerImpl) resetRegisterValue(wf *wavefront.Wavefront) {
	if wf.CodeObject.WIVgprCount > 0 {
		vRegFile := s.cu.VRegFile[wf.SIMDID].(*SimpleRegisterFile)
		vRegStorage := vRegFile.storage
		data := make([]byte, wf.CodeObject.WIVgprCount*4)
		for i := 0; i < 64; i++ {
			offset := uint64(wf.VRegOffset + vRegFile.ByteSizePerLane*i)
			copy(vRegStorage[offset:], data)
		}
	}

	if wf.CodeObject.WFSgprCount > 0 {
		sRegFile := s.cu.SRegFile.(*SimpleRegisterFile)
		sRegStorage := sRegFile.storage
		data := make([]byte, wf.CodeObject.WFSgprCount*4)
		offset := uint64(wf.SRegOffset)
		copy(sRegStorage[offset:], data)
	}
}

func (s *SchedulerImpl) evalSBarrier(
	wf *wavefront.Wavefront,
	now sim.VTimeInSec,
) (madeProgress bool, instCompleted bool) {
	wf.State = wavefront.WfAtBarrier

	wg := wf.WG
	allAtBarrier := s.areAllWfInWGAtBarrier(wg)

	if allAtBarrier {
		s.passBarrier(wg)
		return true, true
	}

	if len(s.barrierBuffer) < s.barrierBufferSize {
		s.barrierBuffer = append(s.barrierBuffer, wf)
		return true, true
	}

	return false, false
}

func (s *SchedulerImpl) areAllWfInWGAtBarrier(wg *wavefront.WorkGroup) bool {
	for _, wf := range wg.Wfs {
		if wf.State != wavefront.WfAtBarrier {
			return false
		}
	}
	return true
}
func (s *SchedulerImpl) passBarrier(wg *wavefront.WorkGroup) {
	s.removeAllWfFromBarrierBuffer(wg)
	s.setAllWfStateToReady(wg)
}

func (s *SchedulerImpl) setAllWfStateToReady(wg *wavefront.WorkGroup) {
	for _, wf := range wg.Wfs {
		s.cu.UpdatePCAndSetReady(wf)
	}
}

func (s *SchedulerImpl) removeAllWfFromBuffer(wg *wavefront.WorkGroup) {
    s.removeAllWfFromBarrierBuffer(wg)

	newExecuting := make([]*wavefront.Wavefront, 0)
	for _, executing := range s.internalExecuting {
		if executing.WG != wg {
            newExecuting = append(newExecuting,executing)
        }
    }
    s.internalExecuting = newExecuting
    //log.Printf("barrier size%d internal size %d",len(s.barrierBuffer),len(s.internalExecuting))

}
func (s *SchedulerImpl) removeAllWfFromBarrierBuffer(wg *wavefront.WorkGroup) {
	newBarrierBuffer := make([]*wavefront.Wavefront, 0, s.barrierBufferSize)
	for _, wavefront := range s.barrierBuffer {
		if wavefront.WG != wg {
			newBarrierBuffer = append(newBarrierBuffer, wavefront)
		}
	}
	s.barrierBuffer = newBarrierBuffer
}

func (s *SchedulerImpl) evalSWaitCnt(
	wf *wavefront.Wavefront,
	now sim.VTimeInSec,
) (madeProgress bool, instCompleted bool) {
	done := true
	inst := wf.Inst()

	if wf.OutstandingScalarMemAccess > inst.LKGMCNT {
		done = false
	}

	if wf.OutstandingVectorMemAccess > inst.VMCNT {
		done = false
	}

	if done {
		s.cu.UpdatePCAndSetReady(wf)
		return true, true
	}

	return false, false
}

// Pause pauses
func (s *SchedulerImpl) Pause() {
	s.isPaused = true
}
func (s *SchedulerImpl) StopNewCode() {
	s.stopNewCode = true
}
func (s *SchedulerImpl) StartNewCode() {
	s.stopNewCode = false
}

// Resume resumes
func (s *SchedulerImpl) Resume() {
	s.isPaused = false
}

// Flush flushes
func (s *SchedulerImpl) Flush() {
	s.barrierBuffer = nil
	s.internalExecuting = nil
}
