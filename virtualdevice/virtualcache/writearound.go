package virtualcache

import (
    //"gitlab.com/akita/mem/v3/mem"
	//"gitlab.com/akita/mem/v3/vm/tlb"
//    "gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/akita/v3/sim"
    "fmt"
    "log"
    "reflect"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcache/internal"
	"gitlab.com/akita/mem/v3/cache"
	"gitlab.com/akita/mgpusim/v3/virtualdevice"
    "runtime"
  //  "math"
)



type Writearound struct {
    *sim.TickingComponent
    realcomponent virtualdevice.RealComponent
    numSets int
    pipelinestagenum int
    postpipelinestagenum int
    sendpipelinestagenum int
    numMSHREntry int
    numBanks int
    log2memorybankinterleaving int
    numWays int
    pageSize uint64
	log2PageSize       uint64
    log2BlockSize uint64
    Sets[] internal.Set
    directory virtualdevice.Directory
    mshr virtualdevice.MSHR
    bottomdevices [] virtualdevice.VirtualComponent
//    topdevices [] VirtualComponent
    requestBuffer [] * virtualdevice.Request
    latency int
}
func (vtlb *Writearound) SetRealComponent(  realcomp virtualdevice.RealComponent ) {
    vtlb.realcomponent = realcomp
}


func (v *Writearound) Tick(now sim.VTimeInSec) bool {
    queuesize := len(v.requestBuffer)
    if queuesize <= 0 {
        return false
    }

    req := v.requestBuffer[ queuesize - 1 ]
    finishtime, success := v.IntervalModel(req)

    if success {
        req.RecoverTime = finishtime
        v.requestBuffer = v.requestBuffer[1:]
        req.Topdevice.NewBottomReadyEvent(finishtime, req)
        return true
    }
    return false
}

func (v *Writearound) AddBottomDevice( bottom virtualdevice.VirtualComponent ) {
    v.bottomdevices = append( v.bottomdevices,bottom )
    v.numBanks = len(v.bottomdevices)
}

//func (v *Writearound) SetBottomDevice( bottom [] virtualdevice.VirtualComponent ) {
//    v.bottomdevices = append( v.bottomdevices,bottom... )
//    v.numBanks = len(v.bottomdevices)
//}


func (tlb *Writearound) vAddrToSetID(vAddr uint64) (setID int) {
	return int(vAddr / tlb.pageSize % uint64(tlb.numSets))
}
func (tlb *Writearound) vAddrToPageID(addr uint64) uint64 {
	return (addr >> tlb.log2PageSize) << tlb.log2PageSize
}
func (tlb *Writearound) vAddrToCacheID(addr uint64) uint64 {
	return (addr >> tlb.log2BlockSize) << tlb.log2BlockSize
}

func ( v *Writearound ) getBank( addr uint64  )  virtualdevice.VirtualComponent {
    var whichbank uint64
    if v.numBanks == 1 {
       whichbank = 0
    } else {
       whichbank = ( addr >> v.log2memorybankinterleaving ) % uint64(v.numBanks)
    }

    v.DebugPrint( "bank %d numbanks %d addr %x\n", whichbank,v.numBanks,addr )
    ret := v.bottomdevices[ whichbank ]
    return ret
}
func (v*Writearound) printName() {
    fmt.Printf( "%s",v.Name() )
}


func (v *Writearound) handleReadMSHRHit( now sim.VTimeInSec, mshrEntry * virtualdevice.MSHREntry  ) sim.VTimeInSec { // return expected latency
//	mshrEntry.Requests = append(mshrEntry.Requests, req )
    if mshrEntry.Expectedtime <= now {
        return v.Freq.NCyclesLater( v.latency,now )
    }
    nexttime := mshrEntry.Expectedtime
    return nexttime
}
func (v *Writearound) handleWriteMSHRHit( now sim.VTimeInSec, mshrEntry * virtualdevice.MSHREntry  ) sim.VTimeInSec { // return expected latency
//	mshrEntry.Requests = append(mshrEntry.Requests, req )
    if mshrEntry.Expectedtime <= now {
        return v.Freq.NCyclesLater( v.latency,now )
    }
    nexttime := mshrEntry.Expectedtime
    return nexttime
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
func (v *Writearound) handleReadHit( now sim.VTimeInSec, block * virtualdevice.Block, ) ( sim.VTimeInSec , bool) {
	v.directory.Visit(block)
    nexttime := v.Freq.NCyclesLater( v.latency ,now )
    return nexttime,true
}

func (v *Writearound) handleWriteHit( now sim.VTimeInSec, block * virtualdevice.Block, ) ( sim.VTimeInSec , bool) {
	v.directory.Visit(block)
    nexttime := v.Freq.NCyclesLater( v.latency ,now )
    return nexttime,true
}



func (v * Writearound ) needEviction(victim *cache.Block) bool {
	return victim.IsValid && victim.IsDirty
}
func (v *Writearound) updateVictimBlockMetaData(victim *virtualdevice.Block, cacheLineID uint64, pid vm.PID) {
	victim.Tag = cacheLineID
	victim.PID = pid
	victim.IsLocked = true
	victim.IsValid = true
	v.directory.Visit(victim)
}
func (v *Writearound) evict(
	victim * virtualdevice.Block,
    cacheLineID uint64,
    pid vm.PID,
) bool {

	v.updateVictimBlockMetaData(victim, cacheLineID, pid)

	return true
}
func (v*Writearound) findVictim( cachelineid uint64, pid vm.PID) *virtualdevice.Block {
    victim := v.directory.FindVictim(cachelineid)
    return victim
//    mshrentry.Block = victim
}



func (vt *Builder)  BuildWritearound(name string) VirtualCache {
    vircache := &Writearound {
        numSets : vt.numSets,
        numBanks : vt.numBanks,
        numMSHREntry : vt.numMSHREntry,
        numWays : vt.numWays,
        log2PageSize : vt.log2PageSize,
        log2BlockSize : vt.log2BlockSize,
        pageSize : 1<<vt.log2PageSize,
        latency : vt.latency,
        pipelinestagenum : vt.pipelinestagenum,
        postpipelinestagenum : vt.postpipelinestagenum,
        sendpipelinestagenum : vt.sendpipelinestagenum,
        log2memorybankinterleaving : vt.log2memorybankinterleaving,
    }
    vircache.directory = virtualdevice.NewDirectory(
		vt.numSets, vt.numWays, 1<<vt.log2BlockSize,
		virtualdevice.NewLRUVictimFinder())
	vircache.TickingComponent = sim.NewTickingComponent(
		name, vt.engine, vt.freq, vircache)

	vircache.mshr = virtualdevice.NewMSHR(vt.numMSHREntry)
    return vircache
}

func (v *Writearound) handleReadFromBottomCompletionEvent(e *ReadFromBottomCompletionEvent) {
   req:=e.req
   block := req.BlockPtr
   pid := block.PID
   cachelineid := block.Tag
   block.IsLocked = false 
   // c.mshr.Query( pid, cachelineid )
    //mshrentry = v.mshr.Remove( pid, cachelineid)
    v.DebugPrint("rm %d\n",cachelineid)
    v.mshr.Remove( pid, cachelineid)

}

func (v *Writearound) handleReleaseBlockEvent(e *ReleaseBlockLockEvent) {
    req := e.req
    block := req.BlockPtr
    block.IsLocked = false 
}
func (v *Writearound) handleReleaseMSHREvent(e *ReleaseMSHREvent) {

    req := e.req
    block := req.BlockPtr
    pid := block.PID
    cachelineid := block.Tag
    v.mshr.Remove( pid, cachelineid)

    ///tick to handle buffer
    v.TickLater(e.Time()) 
}




func (c *Writearound) Handle( e sim.Event ) error {
    switch e := e.(type) {
        case *ReleaseMSHREvent:
            c.handleReleaseMSHREvent(e)
        case *ReleaseBlockLockEvent:
            c.handleReleaseBlockEvent(e)

        case *ReadFromBottomCompletionEvent:
            c.handleReadFromBottomCompletionEvent(e)
        default:
		    log.Panicf("cannot handle event of %s", reflect.TypeOf(e))
    }
    return nil
}



func (v *Writearound) fetch(
    cacheLineID uint64,
    pid vm.PID,
    now sim.VTimeInSec,
    memtype virtualdevice.MemType,

) (sim.VTimeInSec,bool) {
    
    ///which bottom we required
    component := v.getBank( cacheLineID )
    req_time := v.Freq.NCyclesLater(v.sendpipelinestagenum,now)
    req_mem := & virtualdevice.Request{
        Topdevice : v,
        Bottomdevice : component,
        Now : req_time,
        Pid : pid,
        Addr : cacheLineID,
        Memtype : memtype,
    }

    nextEventTime , success := component.IntervalModel( req_mem )
    
	return nextEventTime,success
}

func (v *Writearound) handleviaMshr( now sim.VTimeInSec, cachelineID uint64 , pid vm.PID, memtype virtualdevice.MemType ) (sim.VTimeInSec,bool) {
    block := v.findVictim( cachelineID, pid)
    if block.IsLocked{

        return block.ExpectedTime,false
    }
    
    expectedtime,success := v.fetch( cachelineID,pid, now,memtype)
    if success {
        v.updateVictimBlockMetaData(block,cachelineID,pid)
        v.DebugPrint("add %d\n",cachelineID)


        write_from_bottom_time := v.Freq.NCyclesLater( v.latency , expectedtime )

        expectedtime = v.Freq.NCyclesLater( v.pipelinestagenum,expectedtime )
        v.DebugPrint("add event%d\n",block.Tag)
        block.ExpectedTime = write_from_bottom_time
        mshrentry := v.mshr.Add( pid, cachelineID, now, write_from_bottom_time )
        mshrentry.Block = block
        req_send := &virtualdevice.Request {
            BlockPtr : block,
        }
        nextEvent := & ReadFromBottomCompletionEvent{
        //sim.NewSampledEventBase( nextEventTime, v),
            sim.NewEventBase( write_from_bottom_time  , v),
            req_send,
        }
        v.Engine.Schedule( nextEvent )

        return expectedtime, success
    } else {
        return expectedtime,false
    }
}

func (v *Writearound) NewBottomReadyEvent( time sim.VTimeInSec, req *virtualdevice.Request ) {
    nextEvent := & ReadFromBottomCompletionEvent{
        //sim.NewSampledEventBase( nextEventTime, v),
        sim.NewEventBase( time  , v),
            req,
    }
    v.Engine.Schedule( nextEvent )
}

func (v *Writearound) handleReadMiss( now sim.VTimeInSec, cacheLineID uint64, pid vm.PID) ( sim.VTimeInSec, bool ) { /// return a latency
    ////rank event to top hardware
    

    const memtype = virtualdevice.MemRead
    if !v.mshr.IsFull() {
        expectedtime, success := v.handleviaMshr( now, cacheLineID, pid,memtype )
        return expectedtime,success
    } else {
        return v.mshr.AvailTime(),false
    }
}

func (v *Writearound) handleWriteMiss( now sim.VTimeInSec, cacheLineID uint64, pid vm.PID  ) (sim.VTimeInSec,bool) { /// return a latency
    ////rank event to top hardware
    

    const memtype = virtualdevice.MemWrite
    if !v.mshr.IsFull() {

        v.DebugPrint("%.2f\n", now*1e9)
        expectedtime,success := v.handleviaMshr( now, cacheLineID, pid,memtype )
        return expectedtime,success
    } else {
        return v.mshr.AvailTime(),false
    }
}




func (v*Writearound) GetPipelineStageNum() int {
    return v.pipelinestagenum
}
func (v *Writearound) DebugPrint( format string , args ...interface{} ) {
    return;
    _,filename,line, valid := runtime.Caller(1)
    if valid {
        fmt.Printf( "%s:%d  ",filename,line)
    }
    fmt.Printf( " %s " , v.Name() ) 
    fmt.Printf( format ,  args...)
    fmt.Printf( " \n " ) 
}

func (v *Writearound) handleRead( req * virtualdevice.Request ) (sim.VTimeInSec,bool ) {
    addr:= req.Addr
    pid := req.Pid 
    now := req.Now 

    cachelineID := v.vAddrToCacheID(addr)

    mshrEntry := v.mshr.Query( pid, cachelineID )
    if mshrEntry != nil {
        v.DebugPrint("step2 %d",cachelineID)
        time := v.handleReadMSHRHit( now, mshrEntry )
        return time,true
    }
    block := v.directory.Lookup(pid,cachelineID)
    if block != nil {

        latency, _ :=  v.handleReadHit( now, block )
        latency = v.Freq.NCyclesLater(v.postpipelinestagenum,latency)
        return latency,true
    }

        v.DebugPrint("step4 %d",cachelineID)
    expectedtime, success := v.handleReadMiss( now, cachelineID, pid ) 
    return expectedtime, success

} 

func (v *Writearound) handleWrite( req * virtualdevice.Request ) (sim.VTimeInSec,bool ) {
    addr:= req.Addr
    pid := req.Pid 
    now := req.Now 

    cachelineID := v.vAddrToCacheID(addr)

//    fmt.Printf("cacheline : %d \n", cachelineID)
    ////mshr
    mshrEntry := v.mshr.Query( pid, cachelineID )
    if mshrEntry != nil {

        v.DebugPrint("step1 %d",cachelineID)
       time := v.handleWriteMSHRHit( now, mshrEntry )
       return time,true
    }
    block := v.directory.Lookup(pid,cachelineID)
    if block != nil {
        latency, success :=  v.handleWriteHit( now, block )
        return latency,success
    }

    expectedtime,success := v.handleWriteMiss( now, cachelineID, pid ) 
    v.DebugPrint("step3 %d %t",cachelineID,success)
    return expectedtime, success



}
func (v *Writearound) IntervalModel( req * virtualdevice.Request ) (sim.VTimeInSec,bool ) {
    memtype := req.Memtype
    success := true
    time := req.Now
    v.DebugPrint("%.2f\n",req.Now*1e9)
    if memtype == virtualdevice.MemRead {
        time,success  = v.handleRead(req)
    } else {
        time,success  = v.handleWrite(req)
    }
    if ! success {
        v.requestBuffer = append(  v.requestBuffer, req )
    }

    return time,success
}


