package virtualcache
import (
    //"gitlab.com/akita/mem/v3/mem"
	//"gitlab.com/akita/mem/v3/vm/tlb"
//    "gitlab.com/akita/mgpusim/v3/profiler"
	//"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/akita/v3/sim"
    "fmt"
//    "gitlab.com/akita/mem/v3/dram"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcache/internal"
	"gitlab.com/akita/mgpusim/v3/virtualdevice"
    //"math"
    "log"
    "runtime"
	//"container/heap"
	"container/list"
)


type VirtualDRAM struct {
    *sim.TickingComponent
    Realdram virtualdevice.RealComponent
    numSets int
    numBanks int
    log2memorybankinterleaving int
    numWays int
    pageSize uint64
	log2PageSize       uint64
    log2BlockSize uint64
    Sets[] internal.Set
    latency int
    req_times [] sim.VTimeInSec
    use_queue *list.List
    refresh_latency int
    pipelinestagenum int

    sendpipelinestagenum int
    connectFreq sim.Freq
    connectsendpipelinestagenum int
    connectpipelinestagenum int
}

type UseRange struct{
    begin sim.VTimeInSec
    end sim.VTimeInSec
}
func (v*VirtualDRAM) FlushoutBuffer( now sim.VTimeInSec ) { // flush useless data in use_queue to improve performance

	var ele *list.Element
	for ele = v.use_queue.Front(); ele != nil; ele = ele.Next() {
        if ele.Value.(*UseRange).end < now {
            v.use_queue.Remove( ele )
        } else {
            break;
        }
    }
}
func (v*VirtualDRAM ) merge_with_next( ele  *list.Element  ) { 

     
    ele_next := ele.Next()
     if ele_next != nil {
         end := ele.Value.(*UseRange).end

         begin_next := ele_next.Value.(*UseRange).begin
         if begin_next <= end { //merge them
             
             end_next := ele_next.Value.(*UseRange).end
             if end_next >= end {
                 ele.Value.(*UseRange).end = end_next
             } else {
                 ele.Value.(*UseRange).end = end
             }
             v.use_queue.Remove( ele_next )
         } 
     }
}

func (v * VirtualDRAM) Add2TimeQueue(  now sim.VTimeInSec  ) sim.VTimeInSec  {
	var ele *list.Element

	for ele = v.use_queue.Front(); ele != nil; ele = ele.Next() {

        begin_queue := ele.Value.(*UseRange).begin
        end_queue := ele.Value.(*UseRange).end
        if now <= end_queue {

		    if now >= begin_queue {
                end_queue = v.Freq.NCyclesLater( v.latency ,end_queue )
                end_push_queue := v.Freq.NCyclesLater( v.refresh_latency,end_queue)

                ele.Value.(*UseRange).end = end_push_queue 
                //check if we require to merge with next elem
                v.merge_with_next( ele )
                return end_queue 
            } else if now < begin_queue {
                end_queue =  v.Freq.NCyclesLater(v.latency,now)
                new_ele_value := &UseRange{
                    begin : now,
                    end : v.Freq.NCyclesLater(  v.refresh_latency,end_queue),
                }
                

                new_ele := v.use_queue.InsertAfter( new_ele_value, ele)
                v.merge_with_next( new_ele )
                return end_queue
            }
        }
    }
    if ele == nil {

        end_queue :=  v.Freq.NCyclesLater(v.latency,now)
        new_ele_value := &UseRange{
            begin : now,
            end : v.Freq.NCyclesLater(  v.refresh_latency,end_queue),
        }

		v.use_queue.PushBack(new_ele_value)
        return end_queue
    }
    log.Panic("Unpredictable issue happen")
    return sim.VTimeInSec(0)
}


func (vt *Builder)  BuildDRAM(name string) *VirtualDRAM {
    vircache := &VirtualDRAM {
        numSets : vt.numSets,
        numBanks : vt.numBanks,
        numWays : vt.numWays,
        log2PageSize : vt.log2PageSize,
        log2BlockSize : vt.log2BlockSize,
        pageSize : 1<<vt.log2PageSize,
        latency : vt.latency,
        refresh_latency : 8,
        pipelinestagenum : vt.pipelinestagenum,
        sendpipelinestagenum : vt.sendpipelinestagenum,

        log2memorybankinterleaving : vt.log2memorybankinterleaving,
        connectsendpipelinestagenum : 5,
        connectpipelinestagenum : 1,

    }
	vircache.TickingComponent = sim.NewTickingComponent(
		name, vt.engine, vt.freq, vircache)
    vircache.use_queue = list.New() 
//    vircache.reset()
    return vircache
}


func (vdram * VirtualDRAM) SetConnectFreq( freq sim.Freq ) {
    vdram.connectFreq = freq
}

func (tlb *VirtualDRAM) SetRealComponent( realdram  virtualdevice.RealComponent) {
    tlb.Realdram = realdram
}
func (vtlb * VirtualDRAM) GetPipelineStageNum() int {
    return vtlb.pipelinestagenum
}

func ( v*VirtualDRAM) Evict( now sim.VTimeInSec ) {
    for _, set := range v.Sets {
        set.Evict(now)
    }
}
func (vtlb *VirtualDRAM) Tick(now sim.VTimeInSec) bool {
    return true
}
func (v *VirtualDRAM) DebugPrint( format string , args ...interface{} ) {
    return;
    _,filename,line, valid := runtime.Caller(1)
    if valid {
        fmt.Printf( "%s:%d  ",filename,line)
    }
    fmt.Printf( " %s " , v.Name() ) 
    fmt.Printf( format ,  args...)
    fmt.Printf( " \n " ) 
}



func (v *VirtualDRAM) IntervalModel( req * virtualdevice.Request) (sim.VTimeInSec , bool) {
const enable_bank_conflict = false

    now := req.Now
//    v.DebugPrint( "%.2f", now*1e9  )
    next_now := v.connectFreq.NCyclesLater( v.connectsendpipelinestagenum,now )
    next_now = v.Freq.NCyclesLater(v.sendpipelinestagenum, next_now )
    var nextTime sim.VTimeInSec
    if enable_bank_conflict {
        nextTime = v.Add2TimeQueue( next_now )
    } else {
        nextTime = v.Freq.NCyclesLater( 40,next_now)
    }


    nextTime = v.Freq.NCyclesLater( v.pipelinestagenum,nextTime )
    
    nextTime = v.connectFreq.NCyclesLater( v.connectpipelinestagenum, nextTime )
    v.DebugPrint( "%.2f %.2f",nextTime*1e9 , (nextTime - now) * 1e9 )
    return nextTime, true
}

func (v *VirtualDRAM) NewBottomReadyEvent( time sim.VTimeInSec, req *virtualdevice.Request ) {

}


