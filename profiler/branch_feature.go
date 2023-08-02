package profiler
import (
    "encoding/json"
    "os"
    //"log"
    "flag"
    "fmt"
//	"gitlab.com/akita/mem/v3/vm"

	"gitlab.com/akita/akita/v3/sim"
//	"gitlab.com/akita/mgpusim/v3/utils"

//	"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"
	"gitlab.com/akita/mgpusim/v3/insts"
	"gitlab.com/akita/mgpusim/v3/utils"
)
const frequency =1000000000
var BranchSampled = flag.Bool("branch-profiling", false,
	"Branch sampled machanism.")

var branchProfilingThreshold = flag.Uint64("branch-profiling-threshold", 512 ,
	"Branch sampled machanism threshold.")

type BBL struct{
    PC uint64 `json:"pc"`
    InsNum uint64 `json:"insnum"`
}




func (bbv *BBL) Print() {
    fmt.Printf("%d %d",bbv.PC,bbv.InsNum)
}


type BBVDataFeature struct{
    Start_time sim.VTimeInSec  `json:"begin_time"`
    End_time sim.VTimeInSec `json:"end_time"`
    Bbv BBL `json:"bbv"`

}

type WfFeature struct{
    Inscounts [] uint64 `json:"inscounts"`
}
type WfBranchFeature struct{
    startTime sim.VTimeInSec
    startIns uint64
    wffeature *WfFeature
    currentIns uint64
    PC uint64
    StartPC uint64
    flush_data bool
}

type BranchFeature struct {
    Bbv_data_features []BBVDataFeature `json:"branchdatafeature"`
//    Bbv_features map[uint64] uint64 `json:"bbvfeature"`
//    bbv_features map[BBV] uint64
    wfcount_map map[string]*WfBranchFeature
}

var  Branchfeature *BranchFeature
func  InitBranchFeature(  ) {
    if *BranchSampled{
        Branchfeature = &BranchFeature{
            wfcount_map : make(map[string]*WfBranchFeature),
            //bbv_features : make(map[BBV]uint64),
        }
    }
}




func  ReportBranchFeature(  ) {
    if *BranchSampled{
        jsonStr,err := json.MarshalIndent( Branchfeature,""," ")
        if err != nil {
            panic(err)
        }
        file, _ := os.Create("branch_feature.json")
        defer file.Close()
        file.Write(jsonStr)

    }
}
func (df *BranchFeature) FlushData( wf_branch_feature *WfBranchFeature,now sim.VTimeInSec, PC uint64 ) {

        inscount := wf_branch_feature.currentIns - wf_branch_feature.startIns
        bbv := BBL{
            InsNum : inscount,
            PC : wf_branch_feature.PC - wf_branch_feature.StartPC,
        }
///update bbv information
        bbvdatafeature := BBVDataFeature {
            Start_time : wf_branch_feature.startTime,
            End_time : now,
            Bbv : bbv,
        }
/////////update wavefront information
        wf_branch_feature.startIns = wf_branch_feature.currentIns
        wf_branch_feature.PC = PC
        wf_branch_feature.startTime = now
///////////
        df.Bbv_data_features = append( df.Bbv_data_features,bbvdatafeature )
}
func ( df *BranchFeature ) Collect( wfid string, now sim.VTimeInSec,inst *insts.Inst , state utils.InstEmuState )  {
    if !*BranchSampled {
        return
    }
    wf_branch_feature,found := df.wfcount_map[wfid]
    inswidth := uint64( inst.InstWidth() )
    if !found {
        wf_branch_feature = &WfBranchFeature {
            startTime : now,
            startIns : inswidth,
            currentIns : inswidth,
            PC : inst.PC,
            StartPC : inst.PC,
            flush_data : false,
        }
        df.wfcount_map[wfid] =  wf_branch_feature
    } else {
        wf_branch_feature.currentIns += inswidth
    }
    flush_data := false

    if wf_branch_feature.flush_data {
        flush_data = true
        wf_branch_feature.flush_data = false
    }
    if flush_data {
        df.FlushData(wf_branch_feature,now,inst.PC)
        flush_data = false
    }
    if inst.FormatType == insts.SOPP  {
        switch inst.Opcode {
            case 1:
                delete( df.wfcount_map,wfid )
                wf_branch_feature.currentIns += inswidth
                flush_data = true
            case 2,4,5,6,7,8,9,10: // S_CBRANCH_SCC0
                wf_branch_feature.flush_data = true

            default:

        }
    }
    if flush_data {
        df.FlushData(wf_branch_feature,now,inst.PC)
    }

}


