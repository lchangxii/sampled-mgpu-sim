package profiler
import (
    "encoding/json"
    "os"
    "log"
    "flag"
    "fmt"
//	"gitlab.com/akita/mem/v3/vm"

	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/mgpusim/v3/utils"

//	"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"
	"gitlab.com/akita/mgpusim/v3/insts"
//	"gitlab.com/akita/mgpusim/v3/utils"
)
var WfProfilingFlag = flag.Bool("wf-profiling", false,
	"wavefront level sampled machanism.")

const RNG_A=0x5DEECE66D
const RNG_C=0xB
const RNG_M=((1<<48)-1)
const BbvDim=16
//((__UINT64_C(1) << 48) - 1)

// Same as drand48, but inlined for efficiency
func rng_next(state uint64) (uint64,uint64) { //nextstate, weight

  state = (RNG_A * state + RNG_C) & RNG_M;
  return state,state >> 16;
}
func rng_seed( seed uint64) uint64 {

  return (seed << 16) + 0x330E;
}
const debug=true
type OnlineBbv struct{
    Bbv_vec [BbvDim]uint64 `json:"Bbv"`
    //private
    inscount uint64
    lastinscount uint64
    inited bool
    startPC uint64
    lastPC uint64
    bbv_count map[BBL]uint64
    //for debugging
    //PC_vec []uint64 `json:"pcs"`
    //InsNum_vec []uint64 `json:"insnums"`
    last_is_branch bool
}
func (onlinebbv *OnlineBbv) Bbv_count() *map[BBL]uint64 {
    return &onlinebbv.bbv_count
}
func InitOnlineBbv () *OnlineBbv {
    ret := &OnlineBbv {
        inited : false,
        last_is_branch : false,
    }
    ret.bbv_count = make(map[BBL]uint64)
    for i := 0; i < BbvDim ; i++ {
        ret.Bbv_vec[i] = 0
    }
    return ret
}

func (Bbv *OnlineBbv) Print() {
    log.Printf("online bbv:")
    for bbl,count := range(Bbv.bbv_count) {
        fmt.Printf("pc %d insnum %d: %d \n",bbl.PC, bbl.InsNum , count)
    }
    fmt.Printf("\n")
}
func (Bbv *OnlineBbv) Count( pc, count uint64) {
    Bb := BBL{
        PC : pc,
        InsNum : count,
    }

    Bbv.bbv_count[Bb]++
  // Perform random projection of basic-block vectors onto NUM_Bbv dimensions
  // As there are too many Bbvs, we cannot store the projection matrix, rather,
  // we re-generate it on request using an RNG seeded with the Bbv address.
  // Since this is a hot loop in FAST_FORWARD mode, use an inlined RNG
  // and four parallel code paths to exploit as much ILP as possible.
  s0 := rng_seed(pc)
  s1 := rng_seed(pc + 1)
  s2 := rng_seed(pc + 2)
  s3 := rng_seed(pc + 3);
  var weight uint64
  for i := 0; i < BbvDim; i += 4 {
    s0,weight = rng_next(s0);
    Bbv.Bbv_vec[i] += (weight & 0xffff) * count;
    s1,weight = rng_next(s1)
    Bbv.Bbv_vec[i + 1] += (weight & 0xffff) * count;
    s2,weight = rng_next(s2);
    Bbv.Bbv_vec[i + 2] += (weight & 0xffff) * count;
    s3,weight = rng_next(s3);
    Bbv.Bbv_vec[i + 3] += (weight & 0xffff) * count;
  }
}

func (Bbv *OnlineBbv) CountInst( inst *insts.Inst )  {
//ret if continue or end
    inswidth := uint64( inst.InstWidth() )
    if !Bbv.inited {
        Bbv.startPC = inst.PC
        Bbv.lastPC = inst.PC
        Bbv.inited = true
        Bbv.last_is_branch = false
        Bbv.inscount = inswidth
        Bbv.lastinscount = inswidth
    } else {
        Bbv.inscount += inswidth
    }

    flush_data := false
    if Bbv.last_is_branch {
        Bbv.last_is_branch = false
        inscount := Bbv.inscount - Bbv.lastinscount
        Bbv.Count( Bbv.lastPC - Bbv.startPC, inscount )
        /////////update bbv info
        Bbv.lastinscount = Bbv.inscount
        Bbv.lastPC = inst.PC
        ///////////
    }
    if inst.FormatType == insts.SOPP  {
        switch inst.Opcode {
            case 2,4,5,6,7,8,9,10: // S_CBRANCH_SCC0
                Bbv.last_is_branch = true
            case 1:
                Bbv.inscount += inswidth
                flush_data = true
            default:
        }
    }
    if flush_data {
        inscount := Bbv.inscount - Bbv.lastinscount
        Bbv.Count( Bbv.lastPC - Bbv.startPC, inscount )
    }

}

var Wffinalfeature *WfFinalFeature
type WfFinalFeature struct{
    Issuetime []sim.VTimeInSec `json:"issuetimes"`
    Finishtime []sim.VTimeInSec `json:"finishtimes"`
    KernelStart sim.VTimeInSec `json:"kernelstarttime"`
    Bbvs [] *OnlineBbv `json:"Bbvs"`
    Sampled bool `json:"sampled"`
    Stable_time sim.VTimeInSec `json:"stable_time"`
    wf2wffeature map[string]*WfFeatureElem
    wfidx uint64
    KernelSampledIdx int `json:"kernelsampledidx"`
}
type WfFeatureElem struct{
    startTime sim.VTimeInSec
    endTime sim.VTimeInSec
    Bbv *OnlineBbv
    wfidx uint64
}

func (wf_final_feature * WfFinalFeature) Reset() {
    wf_final_feature.Issuetime = nil
    wf_final_feature.Finishtime = nil
    wf_final_feature.KernelStart = sim.VTimeInSec(0)
    wf_final_feature.Bbvs = nil
    wf_final_feature.Sampled = false
    wf_final_feature.Stable_time = sim.VTimeInSec(0)
    wf_final_feature.wf2wffeature = make(map[string]*WfFeatureElem)
    wf_final_feature.wfidx = 0
    wf_final_feature.KernelSampledIdx = -1
}

func  InitWfFeature(  ) {
    if *WfProfilingFlag{
        Wffinalfeature = &WfFinalFeature{
        }
        Wffinalfeature.Reset()
    }
}


func  ReportWfFeature(  ) {
    if *WfProfilingFlag{
        jsonStr,err := json.MarshalIndent( Wffinalfeature,""," ")
        if err != nil {
            log.Printf("error happened")
            panic(err)
        }
        file, _ := os.Create("wf_feature.json")
        defer file.Close()
        file.Write(jsonStr)
    }
}
func InitWfFeatureElem(now sim.VTimeInSec) *WfFeatureElem {
    ret := &WfFeatureElem {
            startTime : now,
    }
    ret.Bbv = InitOnlineBbv()
    return ret
}
func ( df * WfFinalFeature ) CollectKernelStart(  now sim.VTimeInSec )  {
    if !*WfProfilingFlag {
        return
    }
    df.KernelStart = now
}

func ( df * WfFinalFeature ) CollectWfStart( wfid string, now sim.VTimeInSec )  {
    if !*WfProfilingFlag {
        return
    }
    wf_feature := InitWfFeatureElem(now)
    wf_feature.wfidx = df.wfidx
    df.wfidx ++

    df.Issuetime = append(df.Issuetime, now)
    df.Finishtime = append(df.Finishtime, sim.VTimeInSec(0))
    df.Bbvs = append(df.Bbvs, nil)

    df.wf2wffeature[wfid] =  wf_feature
}

func ( df * WfFinalFeature ) CollectWfEnd( wfid string, now sim.VTimeInSec, sampled_level utils.SampledLevel)  {
    if !*WfProfilingFlag {
        return
    }
    wf_feature, found := df.wf2wffeature[wfid]
    if found {
        if sampled_level == utils.BBSampled || sampled_level == utils.WfSampled {
            df.Sampled = true
            df.Stable_time = now - df.Issuetime[wf_feature.wfidx]
        }
        df.Bbvs[wf_feature.wfidx] = wf_feature.Bbv
        df.Finishtime[wf_feature.wfidx] = now
        delete(df.wf2wffeature,wfid)
    }
}


func ( df *WfFinalFeature ) Collect( wfid string, now sim.VTimeInSec,inst *insts.Inst )  {
    if !*WfProfilingFlag {
        return
    }
    wf_feature, _ := df.wf2wffeature[wfid]
    wf_feature.Bbv.CountInst( inst )
}


