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
	//"gitlab.com/akita/mem/v3/cache"
	"gitlab.com/akita/mgpusim/v3/virtualdevice"
    "runtime"
  //  "math"
)



type Writeback struct {
    *sim.TickingComponent
    realcomponent virtualdevice.RealComponent
    numSets int
    pipelinestagenum int
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

func (vtlb *Writeback) SetRealComponent(  realcomp virtualdevice.RealComponent ) {
    vtlb.realcomponent = realcomp
}



func (v *Writeback) AddBottomDevice( bottom virtualdevice.VirtualComponent ) {
    v.bottomdevices = append( v.bottomdevices,bottom )
    v.numBanks = len(v.bottomdevices)
}

//func (v *Writeback) SetBottomDevice( bottom [] virtualdevice.VirtualComponent ) {
//    v.bottomdevices = append( v.bottomdevices,bottom... )
//    v.numBanks = len(v.bottomdevices)
//}


func (tlb *Writeback) vAddrToSetID(vAddr uint64) (setID int) {
	return int(vAddr / tlb.pageSize % uint64(tlb.numSets))
}
func (tlb *Writeback) vAddrToPageID(addr uint64) uint64 {
	return (addr >> tlb.log2PageSize) << tlb.log2PageSize
}
func (tlb *Writeback) vAddrToCacheID(addr uint64) uint64 {
	return (addr >> tlb.log2BlockSize) << tlb.log2BlockSize
}

func ( v *Writeback ) getBank( addr uint64  )  virtualdevice.VirtualComponent {
    var whichbank uint64
    if v.numBanks == 1 {
       whichbank = 0
    } else {
       whichbank = ( addr >> v.log2memorybankinterleaving ) % uint64(v.numBanks)
    }
    v.DebugPrint( "bank: %d numbanks %d \n", whichbank,v.numBanks )
    ret := v.bottomdevices[ whichbank ]
    return ret
}
func (v*Writeback) printName() {
    fmt.Printf( "%s",v.Name() )
}


func (v *Writeback) handleReadMSHRHit( now sim.VTimeInSec, mshrEntry * virtualdevice.MSHREntry  ) sim.VTimeInSec { // return expected latency
//	mshrEntry.Requests = append(mshrEntry.Requests, req )
    if mshrEntry.Expectedtime <= now {
        return v.Freq.NCyclesLater( v.latency,now )
    }
    nexttime := mshrEntry.Expectedtime
    return nexttime
}
func (v *Writeback) handleWriteMSHRHit( now sim.VTimeInSec, mshrEntry * virtualdevice.MSHREntry  ) sim.VTimeInSec { // return expected latency
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
func (v *Writeback) handleReadHit( now sim.VTimeInSec, block * virtualdevice.Block, ) ( sim.VTimeInSec , bool) {
	v.directory.Visit(block)
    nexttime := v.Freq.NCyclesLater( v.latency ,now )
    return nexttime,true
}

func (v *Writeback) handleWriteHit( now sim.VTimeInSec, block * virtualdevice.Block, ) ( sim.VTimeInSec , bool) {
	v.directory.Visit(block)
    nexttime := v.Freq.NCyclesLater( v.latency ,now )
    return nexttime,true
}



func (v * Writeback ) needEviction(victim *virtualdevice.Block) bool {
	return victim.IsValid && victim.IsDirty
}
func (v *Writeback) updateVictimBlockMetaData(victim *virtualdevice.Block, cacheLineID uint64, pid vm.PID) {
	victim.Tag = cacheLineID
	victim.PID = pid
	victim.IsLocked = true
	victim.IsValid = false
    
	v.directory.Visit(victim)
}




func (v *Writeback) evict(
	victim * virtualdevice.Block,
    cacheLineID uint64,
    pid vm.PID,
    now sim.VTimeInSec,
) (sim.VTimeInSec,bool) {
    
    expectedtime,success := v.fetch(  victim.Tag,victim.PID,now, virtualdevice.MemWrite )

    v.updateVictimBlockMetaData(victim, cacheLineID, pid)   
    victim.Blockstatus = virtualdevice.Evict
    if success {
         
    } else {

    }
	return expectedtime,success
}
func (v*Writeback) findVictim( cachelineid uint64, pid vm.PID ) (*virtualdevice.Block, bool ) {
    victim := v.directory.FindVictim(cachelineid)
    if victim.IsLocked || victim.ReadCount > 0 {
        return nil,false
    }
    return victim,true
}



func (vt *Builder)  BuildWriteback(name string) VirtualCache {
    vircache := &Writeback {
        numSets : vt.numSets,
        numBanks : vt.numBanks,
        numMSHREntry : vt.numMSHREntry,
        numWays : vt.numWays,
        log2PageSize : vt.log2PageSize,
        log2BlockSize : vt.log2BlockSize,
        pageSize : 1<<vt.log2PageSize,
        latency : vt.latency,
        pipelinestagenum : vt.pipelinestagenum,
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

func (v *Writeback) handleEvictCompletion( req * virtualdevice.Request) {
    block := req.BlockPtr
    block.Blockstatus = virtualdevice.Waitbottom
//    v.handleReq(req)
}

func (v *Writeback) handleEvictCompletionEvent(e *EvictCompletionEvent) {
    v.handleEvictCompletion(e.req)
}
func (v*Writeback) handleReleaseMSHR( req * virtualdevice.Request ) {
    block := req.BlockPtr
    pid := block.PID
    cachelineid := block.Tag
    v.mshr.Remove( pid, cachelineid)

    ///tick to handle buffer
//    v.TickLater(e.Time()) 
    v.TickLater(req.Now) 
}


func (v *Writeback) handleReleaseMSHREvent(e *ReleaseMSHREvent) {
    v.handleReleaseMSHR(e.req)
}

func (v *Writeback) Tick(now sim.VTimeInSec) bool {
    queuesize := len(v.requestBuffer) 
    if queuesize == 0 {
        return false
    }
    req := v.requestBuffer[ queuesize - 1 ]
    finishtime, success := v.IntervalModel(req)
    if success {
        req.RecoverTime = finishtime
        v.requestBuffer = v.requestBuffer[1:queuesize]
        req.Topdevice.NewBottomReadyEvent(now, req)
        return true
    } else {
        return false
    }
}



func (v *Writeback) handleReleaseBlockEvent(e *ReleaseBlockLockEvent) {
    req := e.req
    block := req.BlockPtr
    for _, req := range(block.RequestBuffer) {
        req.RecoverTime = e.Time()
        req.Topdevice.NewBottomReadyEvent( e.Time(),req )
    }
    block.IsLocked = false 
}


func (v *Writeback) handleWriteCompletionEvent(e *WriteCompletionEvent) {
	e.req.BlockPtr.IsLocked = false
}


func (v *Writeback) handleReadFromBottomCompletionEvent(e *ReadFromBottomCompletionEvent) {

    req := e.req
    block := req.BlockPtr
    pid := block.PID
    cachelineid := block.Tag
    block.IsLocked = false 
    v.mshr.Remove( pid, cachelineid)
}
func (v*Writeback) handleBottomDeviceReadyEvent(e*virtualdevice.BottomDeviceReadyEvent) {
    req := e.Req
    now := req.RecoverTime
    memtype := req.Memtype
    if memtype == virtualdevice.MemRead {
        block := req.BlockPtr
        pid := block.PID
        cachelineid := block.Tag

        v.mshr.Remove( pid, cachelineid)
        
        ////release block lock
        releaselocktime := v.Freq.NCyclesLater( v.latency , now )
        nextWriteToCacheEvent := & ReleaseBlockLockEvent{
                sim.NewEventBase( releaselocktime  , v),
                req,
            }
        v.Engine.Schedule( nextWriteToCacheEvent )
        v.TickLater( e.Time()) //solve MSHR full
        
        /////////////notify top device the data is ready
        expectedtime := v.Freq.NCyclesLater( v.pipelinestagenum , releaselocktime )
        req.RecoverTime = expectedtime
        req.Topdevice.NewBottomReadyEvent(expectedtime,req)
    } else {
	    req.BlockPtr.IsLocked = false
    }

}
func (c *Writeback) Handle( e sim.Event ) error {
    switch e := e.(type) {
        case *ReleaseMSHREvent:
            c.handleReleaseMSHREvent(e)
        case *ReleaseBlockLockEvent:
            c.handleReleaseBlockEvent(e)
        case *ReadFromBottomCompletionEvent:
            c.handleReadFromBottomCompletionEvent(e)
        case *WriteCompletionEvent:
            c.handleWriteCompletionEvent(e)
        case *virtualdevice.BottomDeviceReadyEvent:
            c.handleBottomDeviceReadyEvent(e)

        default:
		    log.Panicf("cannot handle event of %s", reflect.TypeOf(e))
    }
    return nil
}



func (v *Writeback) fetch(
    cacheLineID uint64,
    pid vm.PID,
    now sim.VTimeInSec,
    memtype virtualdevice.MemType,
) (sim.VTimeInSec,bool) {
    
    ///which bottom we required
    component := v.getBank( cacheLineID )
    req_mem := & virtualdevice.Request{
        Topdevice : v,
        Bottomdevice : component,
        Now : now,
        Pid : pid,
        Addr : cacheLineID,
    }

    nextEventTime , success := component.IntervalModel( req_mem )
    
	return nextEventTime,success
}
func (v*Writeback) handleReadReqAfterBottomDataReady( req * virtualdevice.Request ) (sim.VTimeInSec, bool ) {
    expectedtime := req.Now
    req2 := &virtualdevice.Request{
        BlockPtr : req.BlockPtr,
    }
    nextEvent := & ReleaseMSHREvent{
        sim.NewEventBase( expectedtime  , v),
        req2,
    }
    v.Engine.Schedule( nextEvent )



    releaselocktime := v.Freq.NCyclesLater( v.latency , expectedtime )
    nextWriteToCacheEvent := & ReleaseBlockLockEvent{
        sim.NewEventBase( releaselocktime  , v),
        req,
    }
    v.Engine.Schedule( nextWriteToCacheEvent )

    expectedtime = v.Freq.NCyclesLater( v.pipelinestagenum , releaselocktime )
    return expectedtime,true
}

func (v*Writeback) handleReadReqAfterEvict( req * virtualdevice.Request ) (sim.VTimeInSec,bool) {
    pid := req.Pid
    cachelineID :=  req.Addr
    now := req.Now
    block := req.BlockPtr
    memtype := req.Memtype
    mshrentry := v.mshr.Add( pid, cachelineID, now,now)
    mshrentry.Block=block
    expectedtime,success := v.fetch( cachelineID,pid,now,memtype)
    if success {
        req.Now = expectedtime
        //        handleReleaseMSHREvent
    }
    return expectedtime,success
}

func (v*Writeback) handleReqAfterEvict( req * virtualdevice.Request ) (sim.VTimeInSec,bool) {
    memtype := req.Memtype
    if memtype == virtualdevice.MemRead {
        return v.handleReadReqAfterEvict(req)
    } else {
//        return v.handleWriteReqAfterEvict(req)
        return v.handleReadReqAfterEvict(req)
    }
}


func (v *Writeback) handleReadviaMshr( now sim.VTimeInSec, cachelineID uint64 , pid vm.PID, req * virtualdevice.Request) (sim.VTimeInSec,bool) {
    const memtype = virtualdevice.MemRead
    
    block,success := v.findVictim( cachelineID, pid )
    if !success {
        v.requestBuffer = append( v.requestBuffer , req)
        return  sim.VTimeInSec(0),false
    }

    var next_now sim.VTimeInSec
    success = true
    if v.needEviction(block) {
        next_now,success = v.evict( block, cachelineID , pid, now )
        if !success {
            return next_now,false
        }
    } else {
        next_now = now
    }


    return next_now,success
 
}
func (v *Writeback) NewBottomReadyEvent( time sim.VTimeInSec, req *virtualdevice.Request ) {
    nextEvent := & ReleaseMSHREvent{
        //sim.NewSampledEventBase( nextEventTime, v),
        sim.NewEventBase( time  , v),
            req,
    }
    v.Engine.Schedule( nextEvent )
}


func (v *Writeback) handleWriteviaMshr( now sim.VTimeInSec, cachelineID uint64 , pid vm.PID,req *virtualdevice.Request) (sim.VTimeInSec,bool) {

    const memtype = virtualdevice.MemWrite
    block,_ := v.findVictim( cachelineID, pid )
    if block.IsLocked {
        v.DebugPrint("locked\n")
        block.RequestBuffer = append(block.RequestBuffer,req)
        return block.ExpectedTime,false
    }
    var next_now sim.VTimeInSec
    success := true
    if v.needEviction(block) {
        next_now,success = v.evict( block,cachelineID , pid, now )
        if !success {
            return next_now,false
        }
    } else {
        next_now = now
    } 
    v.DebugPrint("%.2f\n",next_now*1e9)
    if (success) {

        v.updateVictimBlockMetaData( block,cachelineID,pid )
        req := &virtualdevice.Request{
            BlockPtr : block,
        }
        expectedtime := v.Freq.NCyclesLater( v.latency + v.pipelinestagenum, next_now )
        new_event := &WriteCompletionEvent {
            sim.NewEventBase( expectedtime , v),
            req,
        }
        v.Engine.Schedule(new_event)
        return expectedtime, success
    } else {
        return next_now, success
    }
 
}


func (v *Writeback) handleReadMiss( now sim.VTimeInSec, cacheLineID uint64, pid vm.PID, req *virtualdevice.Request) (sim.VTimeInSec,bool) { /// return a latency
    ////rank event to top hardware
    
    if !v.mshr.IsFull() {

        expectedtime,success := v.handleReadviaMshr( now, cacheLineID, pid,req)
        return expectedtime,success
    } else {
        v.requestBuffer = append( v.requestBuffer , req  )
        return v.mshr.AvailTime(),false
    }
}

func (v *Writeback) handleWriteMiss( now sim.VTimeInSec, cacheLineID uint64, pid vm.PID ,req * virtualdevice.Request ) ( sim.VTimeInSec , bool ) { /// return a latency
    ////rank event to top hardware

    if !v.mshr.IsFull() {
        expectedtime,success := v.handleWriteviaMshr( now, cacheLineID, pid,req )
        return expectedtime,success
    } else {
        v.requestBuffer = append(v.requestBuffer,req)
        return v.mshr.AvailTime(),false
    }
}




func (v*Writeback) GetPipelineStageNum() int {
    return v.pipelinestagenum
}
func (v *Writeback) DebugPrint( format string , args ...interface{} ) {
    return
    _,filename,line, valid := runtime.Caller(1)
    if valid {
        fmt.Printf( "%s:%d  ",filename,line)
    }
    fmt.Printf( " %s " , v.Name() ) 
    fmt.Printf( format ,  args...)
    fmt.Printf( " \n " ) 
}
func (v*Writeback) handleRead(req*virtualdevice.Request) (sim.VTimeInSec,bool) {
    addr:= req.Addr
    pid := req.Pid 
    now := req.Now 

    req_time := v.Freq.NCyclesLater(v.sendpipelinestagenum,now)


    cachelineID := v.vAddrToCacheID(addr)
//    fmt.Printf("cacheline : %d \n", cachelineID)
    ////mshr
    mshrEntry := v.mshr.Query( pid, cachelineID )
    if mshrEntry != nil {
        time := v.handleReadMSHRHit( req_time, mshrEntry )
        return time,true
    }
    block := v.directory.Lookup(pid,cachelineID)
    if block != nil {


            latency, success :=  v.handleReadHit( req_time, block )
            return latency,success
    }


    expectedtime,success := v.handleReadMiss( req_time, cachelineID, pid,req ) 
    return expectedtime, success

}

func (v*Writeback) handleWrite(req*virtualdevice.Request) (sim.VTimeInSec,bool) {
    addr:= req.Addr
    pid := req.Pid 
    now := req.Now 

    req_time := v.Freq.NCyclesLater(v.sendpipelinestagenum,now)


    cachelineID := v.vAddrToCacheID(addr)
    mshrEntry := v.mshr.Query( pid, cachelineID )
    if mshrEntry != nil {

       time := v.handleWriteMSHRHit( req_time, mshrEntry )
       return time,true
    }
    block := v.directory.Lookup(pid,cachelineID)
    if block != nil {

        latency, success :=  v.handleWriteHit( req_time, block )
        return latency,success
    }


    expectedtime,success := v.handleWriteMiss( req_time, cachelineID, pid ,req) 
    return expectedtime, success
}
func (v *Writeback) IntervalModel( req * virtualdevice.Request ) (sim.VTimeInSec,bool ) {
    memtype := req.Memtype
    time := req.Now
    success := true
    v.DebugPrint("%.2f\n",req.Now*1e9)
    if memtype == virtualdevice.MemRead {
        time,success  = v.handleRead(req)
    } else {
        time,success  = v.handleWrite(req)
    }
    
    return time,success
    

}


