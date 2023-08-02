package virtualdevice
import (
	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/mgpusim/v3/timing/wavefront"
)

type BottomDeviceReadyEvent struct {
	*sim.EventBase
    Req *Request
}

type VirtualComponent interface{
//    NewHandleBottomResponse( resp * Response )
//    NewSendTopResponse( resp * Response )
    IntervalModel( req * Request ) (sim.VTimeInSec,bool)
    GetPipelineStageNum() int
    NewBottomReadyEvent( time sim.VTimeInSec, req *Request ) 
}
type RealComponent interface{
    
    Name() string
    Tick(now sim.VTimeInSec) bool
    //GetEngine() sim.Engine
    Handle(e sim.Event) error
}

type MemType uint8
const constnum=64
type ExecuteStatus struct{
    Inst_idx int
    Issuetime sim.VTimeInSec
    Finishtime sim.VTimeInSec
    Timestamp [constnum]sim.VTimeInSec
    Steps [constnum] uint32
}
func (exestatus*ExecuteStatus) Reset(){
    exestatus.Inst_idx = 0
    exestatus.Issuetime = sim.VTimeInSec(0)
    exestatus.Finishtime = sim.VTimeInSec(0)
    for i := 0 ; i < constnum ; i++ {
        exestatus.Timestamp[i] = sim.VTimeInSec(0)
        exestatus.Steps[i] = 0
    }
//    exestatus.Timestamp = make([16]sim.VTimeInSec,sim.VTimeInSec(0))
//    exestatus.Steps = make([16]uint32,0)
}

const (
    MemRead MemType = iota
    MemWrite
    MemTypeNum
) 

type Request struct {
    Topdevice VirtualComponent
    Bottomdevice VirtualComponent
    Now sim.VTimeInSec
    RecoverTime sim.VTimeInSec
    Pid vm.PID
    Wfid string
    Addr uint64
    Memtype MemType
	AddtionalInfo  []interface{}
    BlockPtr * Block 
    Wf * wavefront.Wavefront
    RecoverMode bool
}
type Response struct {
    Bottomdevice VirtualComponent
    Topdevice VirtualComponent
    Now sim.VTimeInSec
    Pid vm.PID
    Wfid string
    Req *Request
	AddtionalInfo  []interface{}
}
type RecoverWfEvent struct {
	*sim.EventBase
    Req *Request
}


