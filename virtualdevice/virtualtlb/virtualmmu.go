package virtualtlb
import (
    //"gitlab.com/akita/mem/v3/mem"
	//"gitlab.com/akita/mem/v3/vm/tlb"
//    "gitlab.com/akita/mgpusim/v3/profiler"
	//"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/akita/v3/sim"
    "fmt"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualtlb/internal"
	"gitlab.com/akita/mgpusim/v3/virtualdevice"
    //"math"
)


type VirtualMMU struct {
    *sim.TickingComponent
    realcomponent virtualdevice.RealComponent 
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
}
func (v *VirtualMMU) DebugPrint(now sim.VTimeInSec) {
    return;
    fmt.Printf( "%s %.18f\n",v.Name(), now )
}
func (vtlb * VirtualMMU) GetPipelineStageNum() int {
    return 0
}

func (vt *Builder)  BuildMMU(name string) *VirtualMMU {
    vircache := &VirtualMMU {
        numSets : vt.numSets,
        numBanks : vt.numBanks,
        numWays : vt.numWays,
        log2PageSize : vt.log2PageSize,
        log2BlockSize : vt.log2BlockSize,
        pageSize : 1<<vt.log2PageSize,
        latency : vt.latency,
        log2memorybankinterleaving : vt.log2memorybankinterleaving,
    }
	vircache.TickingComponent = sim.NewTickingComponent(
		name, vt.engine, vt.freq, vircache)

//    vircache.reset()
    return vircache
}

func (tlb *VirtualMMU) SetRealComponent( realdram virtualdevice.RealComponent) {
    tlb.realcomponent = realdram
}

func (vtlb *VirtualMMU) Tick(now sim.VTimeInSec) bool {
    return true
}


func (v *VirtualMMU) IntervalModel( req * virtualdevice.Request) (sim.VTimeInSec , bool) {
    now := req.Now

    nextTime := v.Freq.NCyclesLater(v.latency,now) 

    return nextTime, true
}
func (v *VirtualMMU) NewBottomReadyEvent( time sim.VTimeInSec, req *virtualdevice.Request ) {

}

