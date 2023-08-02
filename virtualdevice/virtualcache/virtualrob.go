package virtualcache

import (
    //"gitlab.com/akita/mem/v3/mem"
	//"gitlab.com/akita/mem/v3/vm/tlb"

//	"gitlab.com/akita/mgpusim/v3/timing/cu"
//	"gitlab.com/akita/mgpusim/v3/kernels"
	//"gitlab.com/akita/mgpusim/v3/timing/wavefront"
	//"gitlab.com/akita/mgpusim/v3/utils"
	"gitlab.com/akita/mgpusim/v3/profiler"
//	"gitlab.com/akita/mgpusim/v3/insts"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualtlb"
	"gitlab.com/akita/mgpusim/v3/virtualdevice"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/akita/v3/sim"
    "fmt"
    "log"
    "os"
    "math"
    "reflect"
     "runtime"
)


type MemAccessStatus struct{
    addr uint64
	pid           vm.PID
    status uint16
}

type WavefrontMemAccessStatus struct {
    mem_accesses [] * MemAccessStatus
    wfid string
    fini bool
    topdevice virtualdevice.VirtualComponent
    req *virtualdevice.Request
}
type VirtualROB struct {
    *sim.TickingComponent
    Virtuall1tlb * virtualtlb.VirtualTLB
    Virtuall1cache VirtualCache

    real_device virtualdevice.RealComponent
    count int
    debug_idx int
    pipelinestagenum int
    postpipelinestagenum int
    preparewflatency int
    wavefront_memstatus_queue []* WavefrontMemAccessStatus
}


func (c *VirtualROB) Handle( e sim.Event ) error {
    switch e := e.(type) {
        case *DataReadyEvent:
            c.handleDataReadyEvent(e)
        case sim.TickEvent:
            return c.TickingComponent.Handle(e)
        default:
		    log.Panicf("cannot handle event of %s", reflect.TypeOf(e))
    }
    return nil
}







func (b * Builder)  BuildROB(name string) *VirtualROB {
    vrob := &VirtualROB {
        //vmem_pipelinestagenum : 70-8+2,
        //smem_pipelinestagenum : 0,
        //vmem_postpipelinestagenum : 2,
        //smem_postpipelinestagenum : 0,
        preparewflatency : 12,
    }
	vrob.TickingComponent = sim.NewTickingComponent(
		name, b.engine, b.freq, vrob)
    return vrob;
}
func (vc *VirtualROB) Setl1cache( para VirtualCache ) {
    vc.Virtuall1cache = para;
}
func (vc *VirtualROB) Setl1tlb( para *virtualtlb.VirtualTLB ) {
    vc.Virtuall1tlb = para;
}



func (v * VirtualROB ) findWavefrontMemAccessStatus( wfid string) * WavefrontMemAccessStatus {
    for wfidx := len(v.wavefront_memstatus_queue)-1; wfidx>= 0 ; wfidx-- {
        wavefront_memstatus := v.wavefront_memstatus_queue[wfidx]
        if wavefront_memstatus.wfid == wfid {
            return wavefront_memstatus
        }
    }
    log.Panic( "wavefront %s can not find ", wfid)
    return nil
}

func (v *VirtualROB) updateWavefrontMemAccessStatus( wavefront_memstatus * WavefrontMemAccessStatus) bool {
    
    for _, memaccess := range(wavefront_memstatus.mem_accesses) {
        if memaccess.status < 2 {
            wavefront_memstatus.fini=false
            return false
        }
    }
    wavefront_memstatus.fini=true
    return true
}
func (v *VirtualROB) handleDataReadyEvent(e * DataReadyEvent) {
    req  := e.req
    time := e.Time()

    req.RecoverTime = time
    wfid := req.Wfid
    addr := req.Addr
    wavefront_memstatus := v.findWavefrontMemAccessStatus(wfid)
    mem_accesses := wavefront_memstatus.mem_accesses 
    success := false
    for _,  mem_access := range(mem_accesses) {
        if mem_access.addr == addr {
            _, success = v.IssueMem( time, mem_access )
            break
        }
    }
    if success {
        v.updateWavefrontMemAccessStatus( wavefront_memstatus )
    }

    if wavefront_memstatus.fini && wfid == v.wavefront_memstatus_queue[len(v.wavefront_memstatus_queue)-1].wfid {
        v.TickNow(time)
    }

}

func (v * VirtualROB) Tick(now sim.VTimeInSec) bool {
    queuesize := len(v.wavefront_memstatus_queue) 
    if queuesize == 0 {
        return false
    }
    wavefrontstatus := v.wavefront_memstatus_queue[ queuesize - 1 ]

    if wavefrontstatus.fini {
        wavefrontstatus.topdevice.NewBottomReadyEvent(now, wavefrontstatus.req)
        v.wavefront_memstatus_queue = v.wavefront_memstatus_queue[0:queuesize-1]
        queuesize := len(v.wavefront_memstatus_queue) 
        if queuesize > 0 && v.wavefront_memstatus_queue[queuesize-1].fini {
            return true
        } else {
            return false
        }
    } else {
        return false
    }
}
func (v *VirtualROB) NewBottomReadyEvent( time sim.VTimeInSec, req *virtualdevice.Request ) {
        new_event := &DataReadyEvent {
            sim.NewEventBase( v.Freq.NextTick( time) , v),
            req,
        }
        v.Engine.Schedule(new_event)
}



func (v * VirtualROB) IssueMem( now sim.VTimeInSec , mem_access_status * MemAccessStatus ) ( sim.VTimeInSec, bool) {
    ret_cycle := now
    success := true
    addr := mem_access_status.addr 
    pid := mem_access_status.pid
    if mem_access_status.status == 0 {
         req_mem := & virtualdevice.Request{
            Topdevice : v,
            Bottomdevice : v.Virtuall1tlb,
            Now : ret_cycle,
            Pid : pid,
            Addr : addr,
        }
        ret_cycle, success = v.Virtuall1tlb.IntervalModel( req_mem)
        if success {
            mem_access_status.status++
        } else {
            return ret_cycle,false
        }
    }
    if mem_access_status.status == 1 {
         req_mem := & virtualdevice.Request{
            Topdevice : v,
            Bottomdevice : v.Virtuall1cache,
            Now : ret_cycle,
            Pid : pid,
            Addr : addr,
        }
        ret_cycle, success = v.Virtuall1cache.IntervalModel( req_mem)
        if success {
            mem_access_status.status++
        } else {
            return ret_cycle,false
        }
    }

    return ret_cycle, true
}


func (v * VirtualROB) IntervalModel( req *virtualdevice.Request ) ( sim.VTimeInSec,bool ) {

    inst_feature := req.AddtionalInfo[0].(*profiler.InstFeature)
    

    memfoot := inst_feature

    //memfoot *profiler.InstFeature
    pid := req.Pid
    now := req.Now 
    addrs := memfoot.ADDR
    issuetime := now
    wavefront_memstatus := & WavefrontMemAccessStatus{
        topdevice : req.Topdevice,
        wfid : req.Wfid,
        fini : false,
        req : req,
    }
    for _, addr := range(addrs) {
        mem_access_status := &MemAccessStatus{
                                    addr:addr,
                                    pid: pid,
                                    status : 0,
                                }
        wavefront_memstatus.mem_accesses = append( wavefront_memstatus.mem_accesses, 
                            mem_access_status)
    }

    success_memins := true
    finishtime := sim.VTimeInSec(0)
    for _,  mem_access := range(wavefront_memstatus.mem_accesses) {
        next_now := v.Freq.NCyclesLater( v.pipelinestagenum , issuetime)
        finishtime2 , success := v.IssueMem( next_now , mem_access )
//        finishtime = finishtime.Max( finishtime2 )
        finishtime = sim.VTimeInSec( math.Max( float64( finishtime ) , float64(finishtime2 )))
        if ! success {
            success_memins = false
        }
        issuetime = v.Freq.NextTick(issuetime)
    }
    if success_memins {
        finishtime = v.Freq.NCyclesLater(v.postpipelinestagenum,finishtime)
    }
    if (!success_memins) || ( len(v.wavefront_memstatus_queue) > 0) {
        v.wavefront_memstatus_queue = append(v.wavefront_memstatus_queue, wavefront_memstatus)
    } 

    return finishtime,success_memins
}


func (vc *VirtualROB) DebugExit() {

        os.Exit(0)
}
func (vtlb * VirtualROB) GetPipelineStageNum() int {
    return 0
}

func (v *VirtualROB) DebugPrint( format string , args ...interface{} ) {
//        return;

    _,filename,line, valid := runtime.Caller(1)
    if valid {
        fmt.Printf( "%s:%d  ",filename,line)
    }
    fmt.Printf( " %s " , v.Name() ) 
    fmt.Printf( format ,  args...)
    fmt.Printf( " \n " ) 
}

