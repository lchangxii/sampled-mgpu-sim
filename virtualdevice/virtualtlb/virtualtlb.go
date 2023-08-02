package virtualtlb

import (
    //"gitlab.com/akita/mem/v3/mem"
	//"gitlab.com/akita/mem/v3/vm/tlb"
//    "gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mem/v3/cache/writearound"
	"gitlab.com/akita/mem/v3/cache/writethrough"
	"gitlab.com/akita/mem/v3/cache/writeback"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/akita/v3/sim"
    "fmt"
    "log"
    "reflect"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualtlb/internal"
	"gitlab.com/akita/mem/v3/cache"
	"gitlab.com/akita/mgpusim/v3/virtualdevice"
    "runtime"
  //  "math"
)

type Builder struct {

    numSets        int
	numWays        int
	numBanks        int
    numMSHREntry    int
	pageSize       uint64
	log2PageSize       uint64
    log2memorybankinterleaving int
    log2BlockSize  uint64
    latency int
    mshrfixedlatency int
    sendpipelinestagenum int
    pipelinestagenum int
    freq sim.Freq
    engine sim.Engine
}
func MakeBuilder(  ) Builder {
    return Builder{
		numSets:        1,
		numWays:        32,
		numBanks:        1,
        numMSHREntry:    4,
        log2PageSize :  12, 
        log2BlockSize : 6,
        log2memorybankinterleaving : 7,
        latency : 0,
        mshrfixedlatency : 8,
        sendpipelinestagenum : 1,
        pipelinestagenum : 1,
    }
}
// WithNumSets sets the number of sets in a TLB. Use 1 for fully associated
// TLBs.
func (b Builder) WithEngine( engine sim.Engine) Builder {
	b.engine = engine
	return b
}
func (b Builder) WithNumSets( numset int) Builder {
	b.numSets = numset
	return b
}
func (b Builder) WithPipelineStageNum( n int) Builder {
	b.pipelinestagenum = n
	return b
}
func (b Builder) WithSendPipelineStageNum(n int) Builder {
	b.sendpipelinestagenum = n
	return b
}



func (b Builder) WithFreq( freq sim.Freq) Builder {
	b.freq = freq
	return b
}

func (b Builder) WithLatency( para int ) Builder {
	b.latency = para
	return b
}
// WithNumWays sets the number of ways in a TLB. Set this field to the number
// of TLB entries for all the functions.
func (b Builder) WithNumWays(n int) Builder {
	b.numWays = n
	return b
}
func (b Builder) WithNumBanks(n int) Builder {
	b.numBanks = n
	return b
}
// A WfCompletionEvent marks the completion of a wavefront
type ReadCompletionEvent struct {
	*sim.EventBase
    req *virtualdevice.Request
}
type ReadFromBottomCompletionEvent struct {
	*sim.EventBase
    req *virtualdevice.Request
}

type VirtualStorage interface {
    
    Evict( now sim.VTimeInSec ) 
    IntervalModel2( addr uint64, pid vm.PID , now sim.VTimeInSec) sim.VTimeInSec 
}

type VirtualTLB struct {
    *sim.TickingComponent
    Realcache * writearound.Cache 
    Realwritethroughcache * writethrough.Cache 
    Realwritebackcache * writeback.Cache 
    numSets int
    numMSHREntry int
    numBanks int
    pipelinestagenum int
    sendpipelinestagenum int
    log2memorybankinterleaving int
    numWays int
    pageSize uint64
	log2PageSize       uint64
    log2BlockSize uint64
    Sets[] internal.Set
    directory cache.Directory
    mshr virtualdevice.MSHR
    bottomdevices [] virtualdevice.VirtualComponent
//    topdevices [] VirtualComponent
///requestBuffer waits for mshr to be avaiable
    requestBuffer [] * virtualdevice.Request

    realcomponent virtualdevice.RealComponent
    latency int
}



func (vt *Builder)  Build(name string) *VirtualTLB {
    vircache := &VirtualTLB {
        numSets : vt.numSets,
        numBanks : vt.numBanks,
        numMSHREntry : vt.numMSHREntry,
        numWays : vt.numWays,
        log2PageSize : vt.log2PageSize,
        log2BlockSize : vt.log2BlockSize,
        pageSize : 1<<vt.log2PageSize,
        latency : vt.latency,
        log2memorybankinterleaving : vt.log2memorybankinterleaving,
        pipelinestagenum : vt.pipelinestagenum,
        sendpipelinestagenum : vt.sendpipelinestagenum,
    }
    vircache.directory = cache.NewDirectory(
		vt.numSets, vt.numWays, 1<<vt.log2BlockSize,
		cache.NewLRUVictimFinder())
	vircache.TickingComponent = sim.NewTickingComponent(
		name, vt.engine, vt.freq, vircache)

	vircache.mshr = virtualdevice.NewMSHR(vt.numMSHREntry)
    vircache.reset()
    return vircache
}
func (tlb *VirtualTLB) reset() {
	tlb.Sets = make([]internal.Set, tlb.numSets)
	for i := 0; i < tlb.numSets; i++ {
		set := internal.NewSet(tlb.numWays)
		tlb.Sets[i] = set
	}
}


func (vtlb * VirtualTLB) GetPipelineStageNum() int {
    return vtlb.pipelinestagenum
}
func (v *VirtualTLB) Tick(now sim.VTimeInSec) bool {
    queuesize := len(v.requestBuffer)
    if queuesize <=0 {
        return false
    }
    req := v.requestBuffer[queuesize-1]

    finishtime , success := v.IntervalModel(req)
    if success {
        req.RecoverTime = finishtime
        v.requestBuffer = v.requestBuffer[1:queuesize]
        req.Topdevice.NewBottomReadyEvent( finishtime,req )
        return true
    } 
    return false
}

func (vtlb *VirtualTLB) SetRealComponent(  realcomp virtualdevice.RealComponent ) {
    vtlb.realcomponent = realcomp
}
func (cache *VirtualTLB) SetLatency( para int) {
    cache.latency = para
}
func (v *VirtualTLB) AddBottomDevice( bottom virtualdevice.VirtualComponent ) {
    v.bottomdevices = append( v.bottomdevices,bottom )
    v.numBanks = len(v.bottomdevices)
}

func (v *VirtualTLB) SetBottomDevice( bottom [] virtualdevice.VirtualComponent ) {
    v.bottomdevices = append( v.bottomdevices,bottom... )
    v.numBanks = len(v.bottomdevices)
}
func (tlb *VirtualTLB) SetRealCache( realcache *writearound.Cache) {
    tlb.Realcache = realcache
}
func (tlb *VirtualTLB) SetRealWritebackCache( realcache *writeback.Cache) {
    tlb.Realwritebackcache = realcache
}
func (tlb *VirtualTLB) SetRealWritethroughCache( realcache *writethrough.Cache) {
    tlb.Realwritethroughcache = realcache
}


func (tlb *VirtualTLB) vAddrToSetID(vAddr uint64) (setID int) {
	return int(vAddr / tlb.pageSize % uint64(tlb.numSets))
}
func (tlb *VirtualTLB) vAddrToPageID(addr uint64) uint64 {
	return (addr >> tlb.log2PageSize) << tlb.log2PageSize
}
func (tlb *VirtualTLB) vAddrToCacheID(addr uint64) uint64 {
	return (addr >> tlb.log2BlockSize) << tlb.log2BlockSize
}

func ( v *VirtualTLB ) getBank( addr uint64  )  virtualdevice.VirtualComponent {
    var whichbank uint64
    if v.numBanks == 1 {
       whichbank = 0
    } else {
       whichbank = ( addr >> v.log2memorybankinterleaving ) % uint64(v.numBanks)
    }
//    fmt.Printf( "%d %d %s\n", whichbank , len(v.bottomdevices), v.realcomponent.Name() )

    ret := v.bottomdevices[ whichbank ]
    return ret
}


func (v *VirtualTLB) handleReadMSHRHit( now sim.VTimeInSec, mshrEntry *virtualdevice.MSHREntry  ) sim.VTimeInSec { // return expected latency
//	mshrEntry.Requests = append(mshrEntry.Requests, req )
    if now >= mshrEntry.Expectedtime { //this tlb table has been inserted in table
        return v.Freq.NCyclesLater( v.latency,now )
    }
    nexttime := mshrEntry.Expectedtime
    return nexttime
}

func (v*VirtualTLB) printName() {
    fmt.Printf( "%s",v.Name() )
}
//
//
//func NewReadCompleteEvent(
//	time sim.VTimeInSec,
//	handler sim.Handler,
//	wf *wavefront.Wavefront,
//) *ReadCompletionEvent {
//	evt := new(ReadCompletionEvent)
//	evt.EventBase = sim.NewEventBase(time, handler)
////	evt.Wf = wf
//	return evt
//}
//
//func handlReadCompleteEvent( evt sim.Event ) {
//    top.RespondReadMSHRHit(  )
//}
//
func (v *VirtualTLB) handleReadHit( now sim.VTimeInSec, wayID int  ) ( sim.VTimeInSec , bool) {
    if v.latency == 0 {
        return now,true
    } else {
    
        nexttime := v.Freq.NCyclesLater( v.latency ,now )
    
        return nexttime,true
    }
}
func (v * VirtualTLB ) needEviction(victim *cache.Block) bool {
	return victim.IsValid && victim.IsDirty
}
func (v *VirtualTLB) updateTLBTable( pageid uint64, pid vm.PID) {
	setID := v.vAddrToSetID( pageid)
	set := v.Sets[setID]
	wayID, ok := v.Sets[setID].Evict()
	if !ok {
		panic("failed to evict")
	}
    page := vm.Page{
    	PID   : pid,
        VAddr : pageid,
        Valid : true,
    }
	set.Update(wayID, page)
	set.Visit(wayID)
}

func (v *VirtualTLB) handleBottomDeviceReadyEvent( e *virtualdevice.BottomDeviceReadyEvent ) {
    resp := e.Req
    pid := resp.Pid
    pageid := resp.Addr
    v.mshr.Remove( pid, pageid)
    v.updateTLBTable( pageid, pid)
    time := resp.RecoverTime
        ////return to top device
        time = v.Freq.NCyclesLater( v.pipelinestagenum , time )
        resp.RecoverTime = time
        resp.Topdevice.NewBottomReadyEvent( time, resp )

    ///tick later to handle mhsr issue
    v.TickLater(e.Time())

}


func (v *VirtualTLB) handleReadFromBottomCompletionEvent(e *ReadFromBottomCompletionEvent) {
    resp := e.req
    pid := resp.Pid
    pageid := resp.Addr
//    mshrentry := v.mshr.Query( pid, pageid )
//    v.mshr.Query( pid, pageid )

   // c.mshr.Query( pid, cachelineid )
   //mshrentry := v.mshr.Remove( pid, pageid)
   v.mshr.Remove( pid, pageid)
   

//    for mshrentry != nil {
//         
//        pageid = mshrentry.Address 
//        pid = mshrentry.PID
//        if(mshrentry.Expectedtime < resp.Now ) {
   v.updateTLBTable( pageid, pid)
//            break
//        } else {
//
//            mshrentry = v.mshr.Remove( pid, pageid)
//        }
//    }
}



func (c *VirtualTLB) Handle( e sim.Event ) error {
    switch e := e.(type) {
        case *virtualdevice.BottomDeviceReadyEvent:
            c.handleBottomDeviceReadyEvent(e)
        case *ReadFromBottomCompletionEvent:
            c.handleReadFromBottomCompletionEvent(e)
        default:
		    log.Panicf("cannot handle event of %s", reflect.TypeOf(e))
    }
    return nil
}



func (v *VirtualTLB) fetch(
    pageid uint64,
    pid vm.PID,
    now sim.VTimeInSec,

) (sim.VTimeInSec,bool) {
    
    ///which bottom we required
    component := v.getBank( pageid )

    req_mem := & virtualdevice.Request{
        Topdevice : v,
        Bottomdevice : component,
        Now : now,
        Pid : pid,
        Addr : pageid,
    }
    
    nextEventTime , bottom_success := component.IntervalModel( req_mem )
    if bottom_success {
        nextEvent := & ReadFromBottomCompletionEvent{
            sim.NewEventBase( nextEventTime, v),
            req_mem,
        }
        v.Engine.Schedule( nextEvent )
    } 
    
    return nextEventTime,bottom_success
//	return v.Freq.NCyclesLater( v.latency, nextEventTime)
}
func (v *VirtualTLB) NewBottomReadyEvent( time sim.VTimeInSec, req *virtualdevice.Request ) {
        nextEvent := & virtualdevice.BottomDeviceReadyEvent {
    //    sim.NewSampledEventBase( nextEventTime, v),
            sim.NewEventBase( time, v),
            req,
        }
        v.Engine.Schedule( nextEvent )


}


func (v *VirtualTLB) handleviaMshr( now sim.VTimeInSec, pageid uint64 , pid vm.PID) (sim.VTimeInSec,bool) {
    v.mshr.Add( pid, pageid, now, sim.VTimeInSec(0 ))

    expectedtime,bottom_success := v.fetch( pageid,pid,now)
    return expectedtime,bottom_success 
}
func (v *VirtualTLB) handleReadMiss( now sim.VTimeInSec, pageid uint64, pid vm.PID , req *virtualdevice.Request) (sim.VTimeInSec, bool) { /// return a latency
    ////rank event to top hardware
    
    var expectedtime sim.VTimeInSec
    var success bool
    if !v.mshr.IsFull() {
        
        expectedtime,success = v.handleviaMshr( now, pageid, pid )
        return expectedtime,success
    } else {

        v.requestBuffer = append( v.requestBuffer, req )

        return v.mshr.AvailTime(), false
    }
}
func (v *VirtualTLB) DebugPrint( format string , args ...interface{} ) {
    return;
    _,filename,line, valid := runtime.Caller(1)
    if valid {
        fmt.Printf( "%s:%d  ",filename,line)
    }
    fmt.Printf( " %s " , v.Name() ) 
    fmt.Printf( format ,  args...)
    fmt.Printf( " \n " ) 
}


func (v *VirtualTLB) IntervalModel( req * virtualdevice.Request ) (sim.VTimeInSec,bool ) {
    addr:= req.Addr
    pid := req.Pid 
    now := req.Now 
    now_recv := v.Freq.NCyclesLater( v.sendpipelinestagenum , now )
    
    pageid := v.vAddrToPageID(addr)

    var success bool
    ////mshr
    mshrEntry := v.mshr.Query( pid, pageid )
    if mshrEntry != nil {
        time := v.handleReadMSHRHit( now_recv, mshrEntry )
        time = v.Freq.NCyclesLater( v.pipelinestagenum , time )
        return time,true
    }
	setID := v.vAddrToSetID( pageid)
	set := v.Sets[setID]

    wayid,_,found := set.Lookup(pid,pageid)
    var expectedtime sim.VTimeInSec
    if found {
        expectedtime,success =  v.handleReadHit( now_recv, wayid )
	    set.Visit(wayid)
    } else {
        expectedtime,success = v.handleReadMiss( now_recv, pageid, pid,req ) 
    }
    expectedtime = v.Freq.NCyclesLater( v.pipelinestagenum , expectedtime )
 //   v.DebugPrint( "%.2f %d %d \n",( expectedtime - now ) *1e9, v.pipelinestagenum,v.sendpipelinestagenum )
    return expectedtime, success
}


