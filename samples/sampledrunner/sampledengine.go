package sampledrunner
import (
    "container/heap"
	"gitlab.com/akita/mgpusim/v3/timing/wavefront"
//	"gitlab.com/akita/mgpusim/v3/kernels"
	"gitlab.com/akita/akita/v3/sim"
    "log"
    "reflect"
)

type Wavefront struct {
	*wavefront.Wavefront
//    bbls []profiler.BBL
//    bbltimes []sim.VTimeInSec
    starttime sim.VTimeInSec
    endtime sim.VTimeInSec
    submitted bool
    cu sim.Handler
}

type wfHeap []*Wavefront

// Len returns the length of the event queue
func (h wfHeap) Len() int {
	return len(h)
}

func (h wfHeap) Less(i, j int) bool {
	return h[i].endtime < h[j].endtime
}

// Swap changes the position of two events in the event queue
func (h wfHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push adds an event into the event queue
func (h *wfHeap) Push(x interface{}) {
	wf := x.(*Wavefront)
	*h = append(*h, wf)
}

// Pop removes and returns the next event to happen
func (h *wfHeap) Pop() interface{} {
	old := *h
	n := len(old)
	wf := old[n-1]
	*h = old[0 : n-1]
	return wf
}



type SampledTimeEngine struct {
    *sim.TickingComponent
    wfs  wfHeap
    targetcompltedwfs uint64
    submitedwfs uint64
    finishedbytimemodel uint64
    activate_wfs uint64
}
var Sampledtimeengine *SampledTimeEngine

func (cu *SampledTimeEngine) SetTargetCompletedWfs( target uint64) {
    cu.targetcompltedwfs = target
}

func (cu *SampledTimeEngine) Tick(now sim.VTimeInSec) bool {
    return false
}
func InitSampledTimeEngine( engine sim.Engine,freq sim.Freq ) {
    Sampledtimeengine = &SampledTimeEngine{
    }
	Sampledtimeengine.TickingComponent = sim.NewTickingComponent(
		"sampledtimeengine", engine, freq, Sampledtimeengine)

    Sampledtimeengine.wfs = make([]*Wavefront,0)
    Sampledtimeengine.submitedwfs = 0
    heap.Init(&Sampledtimeengine.wfs)
}
///two types of events
///the time of the sampled wf completion comes; call corresponding cu that the wf completed
///the time of the
type SampledWfCompletedEvent struct {
	*sim.EventBase

}
func (c *SampledTimeEngine) Handle( e sim.Event ) error {
    switch e := e.(type) {
        case *SampledWfCompletedEvent:
            c.handleSampledWfCompletedEvent(e)
        default:
		    log.Panicf("cannot handle event of %s", reflect.TypeOf(e))
    }
    return nil
}

func (engine *SampledTimeEngine) handleSampledWfCompletedEvent( e sim.Event ) {
    now := e.Time()
    complete_time := engine.Freq.NextTick(now)
    if engine.wfs.Len() == 0 {
        return
    }
    new_wf := engine.wfs[0]
    //log.Printf("wf queue size %d %.2f %.2f %b\n",engine.wfs.Len(),new_wf.endtime*1e9,now*1e9,new_wf.endtime == now)
    for new_wf.endtime == now { ///submit all completed wfs
        
        evt := wavefront.NewWfCompletionEvent( complete_time, new_wf.cu,new_wf.Wavefront)
        
        engine.Engine.Schedule( evt )
        heap.Pop(&engine.wfs)
        if engine.wfs.Len() == 0 {
            break
        }
        new_wf = engine.wfs[0]
    }
    if engine.wfs.Len()> 0 && (!new_wf.submitted ){
        engine.NewSampledWfCompleteEvent(new_wf)
    }
}
func (engine *SampledTimeEngine) NewSampledWfCompleteEvent( new_wf *Wavefront ) {
        evt := new(SampledWfCompletedEvent)
        new_wf.submitted = true //marked as submitted
        time := new_wf.endtime
        //new event 
        //log.Printf("new event %.2f\n",time*1e9)
        evt.EventBase = sim.NewSampledEventBase( time , engine )
        engine.Engine.Schedule( evt )
}

func (engine *SampledTimeEngine) IncreaseIdx(  ) {
    engine.submitedwfs++
    engine.finishedbytimemodel++
//    log.Printf("start %d\n",engine.finishedbytimemodel )
}

func (engine *SampledTimeEngine) UpdateMaxWFS( wfnum uint64 ) {
    if engine.activate_wfs < wfnum {
        engine.activate_wfs = wfnum
    }
}
func (engine *SampledTimeEngine) finetuneTime(now sim.VTimeInSec) {
    wfnums := engine.wfs.Len()
    wfs := make([]*Wavefront,wfnums)
    i := 0
    for engine.wfs.Len() != 0 {
        wfs[i] = engine.wfs[0]
        heap.Pop(&engine.wfs)
        i++
    }
    activate_wfs := engine.activate_wfs
//    endtime := wfs[0].endtime
//    for _,wf := range wfs{
//        if wf.starttime < endtime {
//            activate_wfs++
//        }
//    }
    log.Printf("%d %d\n",activate_wfs,wfnums)
    log.Printf("submitted wf complete %d ",engine.submitedwfs)
    if activate_wfs == 1 {
        return
    }
    idx := 1
    endidx := len(wfs)
    left_times := make([]sim.VTimeInSec,wfnums-1)
    for idx < endidx {
        left_time := wfs[idx].endtime - wfs[idx-1].endtime
        current_activate_wfs := wfnums - idx
        left_times[idx-1] = left_time * sim.VTimeInSec(current_activate_wfs) / sim.VTimeInSec(activate_wfs)

        //current_activate_wfs--
        idx++
    }
    idx = 1
    for idx < endidx {
        wfs[idx].endtime = wfs[idx-1].endtime + left_times[idx-1]
        idx++

    }
    engine.Engine.DisabledSampled()
    nexttick := engine.Freq.NextTick(now)
    for _,wf := range wfs{
        if wf.endtime < nexttick {
            wf.endtime = nexttick
        }
        evt := wavefront.NewWfCompletionEvent( wf.endtime, wf.cu, wf.Wavefront)
        engine.Engine.Schedule( evt )
    }
}

func (engine *SampledTimeEngine) NewRawSampledWfCompletionEvent(  now sim.VTimeInSec,time sim.VTimeInSec, handler sim.Handler , wf  * wavefront.Wavefront ) {
    new_wf := &Wavefront{
        Wavefront:wf,
        submitted : false,
        endtime : time,
        starttime : now,
        cu:handler,
    }
    heap.Push( &engine.wfs,  new_wf)
    engine.submitedwfs++
    //log.Printf("start %.2f %.2f %d target %d\n",now*1e9, time*1e9, engine.submitedwfs,engine.targetcompltedwfs )
    if (engine.submitedwfs==engine.targetcompltedwfs) { 
        engine.finetuneTime(now)
    } else {
        if new_wf == engine.wfs[0] {
            engine.NewSampledWfCompleteEvent( new_wf)
        }
    }
}

