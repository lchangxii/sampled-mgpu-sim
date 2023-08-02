package sampledrunner
import (
//    "encoding/json"
//    "os"
    "log"
    "flag"
    "fmt"
//	"gitlab.com/akita/mem/v3/vm"

	"gitlab.com/akita/akita/v3/sim"
//	"gitlab.com/akita/mgpusim/v3/utils"

//	"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"
	"gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mgpusim/v3/insts"
	"gitlab.com/akita/mgpusim/v3/utils"
)
var BranchSampledFlag = flag.Bool("branch-sampled", false,
	"Branch sampled machanism.")
var BranchSampledThresholdFlag = flag.Float64("branch-sampled-coverage-threshold", 0.95,
	"Branch sampled machanism coverage threshold.")
var BranchSampledLeastSqureFlag = flag.Float64("branch-sampled-threshold", 0.03,
	"Branch sampled machanism threshold.")

type StaticComputeUnit interface {
    GetBBLInsts( bbl profiler.BBL)[]*insts.Inst
}

type WfBranchFeature struct{
    wfStartTime sim.VTimeInSec
    wfStartInterval sim.VTimeInSec
    startTime sim.VTimeInSec
    startIns uint64
    currentIns uint64
    PC uint64
    StartPC uint64
    last_inst_is_branch bool
    bbl_seq []profiler.BBL
    lastBBLFinishTime sim.VTimeInSec
    predict_bb_idx int
}



type BranchSampledEngine struct {
    Freq sim.Freq
    static_compute_unit StaticComputeUnit
    bbv2sampledengine_map map[profiler.BBL]*SampledEngine
    bbv2bbmodeltime_map map[profiler.BBL]sim.VTimeInSec
    bbl2rate map[profiler.BBL]float64
    wfcount_map map[string]*WfBranchFeature
    bbv_counts map[profiler.BBL] uint64
    insnums uint64
    insnums_enablesampled uint64
    enableSampled bool
    disableEngine bool
    //endtimesum sim.VTimeInSec
  //  begintimesum sim.VTimeInSec
//    begintimenum uint64
    //endtimenum uint64
    finish_rate float64
    last_inst_is_branch bool
}
func (sampled_engine *BranchSampledEngine) SetStaticComputeUnit( staticcomputeunit StaticComputeUnit ) {
    if sampled_engine == nil {
        return
    }
    sampled_engine.static_compute_unit = staticcomputeunit
}

func (sampled_engine *BranchSampledEngine) Reset() {
    if !*BranchSampledFlag {
        return
    }
    fmt.Printf("branch engine reset\n")
    sampled_engine.bbv2sampledengine_map = make(map[profiler.BBL]*SampledEngine)
    sampled_engine.bbv2bbmodeltime_map = make(map[profiler.BBL]sim.VTimeInSec)
    sampled_engine.wfcount_map = make(map[string]*WfBranchFeature)
    sampled_engine.bbv_counts = make(map[profiler.BBL]uint64)
    sampled_engine.bbl2rate = make(map[profiler.BBL]float64)
    sampled_engine.enableSampled = false
//  sampled_engine.  enableSampled : true,
    sampled_engine.insnums = 0
    sampled_engine.insnums_enablesampled = 0
    //sampled_engine.endtimesum = sim.VTimeInSec(0)
//    sampled_engine.begintimesum = sim.VTimeInSec(0)
  //  sampled_engine.begintimenum = 0
//    sampled_engine.endtimenum = 0
    sampled_engine.finish_rate = 0
    sampled_engine.last_inst_is_branch = false
    sampled_engine.disableEngine = true
}


var  Branchsampledengine *BranchSampledEngine
func  InitBranchSampledFeature( freq sim.Freq ) {
    if *BranchSampledFlag {
        Branchsampledengine = &BranchSampledEngine{
            Freq :freq,
        }
        Branchsampledengine.Reset()
        InitUniqBBModel(freq)
    }
}

func ( br_engine *BranchSampledEngine ) EnableSampled(  )  bool {
    return br_engine.enableSampled
}
func ( br_engine *BranchSampledEngine ) Predict( wfid string,bbv profiler.BBL )  sim.VTimeInSec  {
    if br_engine.enableSampled {

        wffeature,found := br_engine.wfcount_map[wfid]
        if found {
            wffeature.predict_bb_idx++
//            fmt.Printf("  %d\n", )
            if wffeature.predict_bb_idx == len(wffeature.bbl_seq) {

                interval := wffeature.lastBBLFinishTime - wffeature.wfStartTime 
                interval -= br_engine.StartTime()
                delete( br_engine.wfcount_map,wfid)
                return interval

            } else if wffeature.predict_bb_idx < len(wffeature.bbl_seq) {
                return sim.VTimeInSec(0)
            }
        }
//        for idx, elem := range br_engine.bbv2sampledengine_map{
//            predicttime,_ := elem.Predict()
//            fmt.Printf("%d %d %.2f\n",idx.PC,idx.InsNum,predicttime*1e9)
//        }
        sampledmap,found := br_engine.bbv2sampledengine_map[bbv]
        if found {
            predicttime , predfound := sampledmap.Predict()
            if predfound {
  //          fmt.Printf("solved %d %d\n",bbv.PC,bbv.InsNum)
                return predicttime
            }
        }
//            fmt.Printf("unsolved ")

            intervaltime,found2 := br_engine.bbv2bbmodeltime_map[bbv]
            if found2 {
                return intervaltime
            } else {
                intervaltime = Global_bbmode.IntervalModel( br_engine.static_compute_unit.GetBBLInsts( bbv ) )
                br_engine.bbv2bbmodeltime_map[bbv] = intervaltime
            }

  //          fmt.Printf("unsolved %d %d %.2f\n",bbv.PC,bbv.InsNum,intervaltime)

            return intervaltime
    } else {
        return sim.VTimeInSec(0)
    }
}
func(br_engine*BranchSampledEngine) Print() {
    fmt.Printf("Hello")
    log.Printf("Hello")
}

func ( br_engine *BranchSampledEngine ) CollectWfStart( wfid string, now sim.VTimeInSec )  {
    if !*BranchSampledFlag {
        return
    }
    if br_engine.enableSampled {
        return
    }
    wf_branch_feature := &WfBranchFeature {
        wfStartTime : now,
        startTime : now,
        startIns : 0,
        currentIns : 0,
        last_inst_is_branch : false,
        predict_bb_idx : 0,
    }
    //panic(inst.PC)
 //   fmt.Printf("collect begin wfid %s\n",wfid)
    br_engine.wfcount_map[wfid] =  wf_branch_feature
}

func ( br_engine *BranchSampledEngine ) CollectWfEnd( wfid string, now sim.VTimeInSec )  {
    if !*BranchSampledFlag {
        return
    }
    if br_engine.enableSampled {
        return
    }
//    wf_branch_feature,_ := br_engine.wfcount_map[wfid]
//    wf_branch_feature, found := br_engine.wfcount_map[wfid]
    _, found := br_engine.wfcount_map[wfid]
    if found {
//        endtime := now - wf_branch_feature.wfStartTime
    //    fmt.Printf("wfid %s endtime %f %f %f\n",wfid,endtime * 1e9, now*1e9,wf_branch_feature.startTime*1e9)
//        br_engine.endtimesum += endtime 
//        br_engine.endtimenum++ 
        delete( br_engine.wfcount_map,wfid )
    }
}
func ( br_engine *BranchSampledEngine ) EndTime( ) sim.VTimeInSec {

    return br_engine.Freq.NextTick(sim.VTimeInSec(0))
//    return sim.VTimeInSec(0))
    //if br_engine.endtimenum == 0 {
    //    return sim.VTimeInSec(0)
    //} else {
    //    return br_engine.endtimesum / sim.VTimeInSec(br_engine.endtimenum)
    //}
}


func ( br_engine *BranchSampledEngine ) StartTime( ) sim.VTimeInSec {
    return br_engine.Freq.NextTick(sim.VTimeInSec(0))
    //br_engine.begintimesum / sim.VTimeInSec(br_engine.begintimenum)
}

func ( br_engine *BranchSampledEngine ) Analysis( bbls []*profiler.OnlineBbv )  {
    var sum uint64
    sum = 0
    bbl2count := make( map[profiler.BBL]uint64 )
    for _,onlinebbl := range(bbls) {
        for bbl,count := range( (*onlinebbl.Bbv_count())) {
            num := count * bbl.InsNum
            bbl2count[bbl] += num
            sum += num
        }
    }
    for bbl,count := range(bbl2count) {
        br_engine.bbl2rate[bbl] = float64(count) / float64(sum)
    }
    for bbl,rate := range(br_engine.bbl2rate){
        bbl.Print()
        fmt.Printf(" %.2f\n",rate)
    }
}

func (br_engine *BranchSampledEngine) Flush(wf_branch_feature * WfBranchFeature, now sim.VTimeInSec, PC uint64 ) {
        inscount := wf_branch_feature.currentIns - wf_branch_feature.startIns
        bbl := profiler.BBL{
            PC:wf_branch_feature.PC - wf_branch_feature.StartPC,
            InsNum : inscount,
        }
        branchsampled_engine, found := br_engine.bbv2sampledengine_map[ bbl ]
        issuetime := wf_branch_feature.startTime
        finishtime := now

/////////update wavefront information
        wf_branch_feature.startIns = wf_branch_feature.currentIns
        wf_branch_feature.PC = PC
        wf_branch_feature.startTime = now
///////////
        wf_branch_feature.lastBBLFinishTime = finishtime
        wf_branch_feature.bbl_seq = append(wf_branch_feature.bbl_seq,bbl)
        if found {
            if branchsampled_engine.enableSampled {
                //updated := 
                branchsampled_engine.Update( issuetime,finishtime  )
//                if updated{
//                    bbl.Print()
//                    fmt.Printf("updated pred time %.2f\n", branchsampled_engine.predTime*1e9)
//                }
                return
            } else {

                branchsampled_engine.Collect( issuetime,finishtime )
            }

        } else {
            branchsampled_engine = NewSampledEngine(4096,*BranchSampledLeastSqureFlag,true)
//            branchsampled_engine.Reset()
            branchsampled_engine.Collect( issuetime , finishtime)
            br_engine.bbv2sampledengine_map[bbl] = branchsampled_engine
        }
        current_enable_sampled := branchsampled_engine.enableSampled
        if current_enable_sampled {
            br_engine.Update( bbl )
            bbl.Print()
            fmt.Printf(" pred time %.2f\n", branchsampled_engine.predTime*1e9)

        }

}
func (sampled_engine *BranchSampledEngine) Disabled(  ) {
    sampled_engine.disableEngine = true 
}
func (sampled_engine *BranchSampledEngine) Enable(  ) {
    sampled_engine.disableEngine = false
}
func (sampled_engine *BranchSampledEngine) IfDisable(  ) bool {
    return sampled_engine.disableEngine 
}

func ( br_engine *BranchSampledEngine ) Collect( wfid string, now sim.VTimeInSec,inst *insts.Inst , state utils.InstEmuState )  {
    if !*BranchSampledFlag  {
        return
    }
    if br_engine.enableSampled || br_engine.disableEngine {
        return
    }
    if len(br_engine.bbl2rate) == 0{
        return
    }

    wf_branch_feature,_ := br_engine.wfcount_map[wfid]
    inswidth := uint64( inst.InstWidth() )
    if wf_branch_feature.currentIns == 0 {
        wf_branch_feature.PC = inst.PC
        wf_branch_feature.StartPC = inst.PC
        wf_branch_feature.wfStartInterval = now - wf_branch_feature.wfStartTime
        //br_engine.begintimesum += wf_branch_feature.wfStartTime
//        br_engine.begintimenum++
        wf_branch_feature.currentIns = inswidth
        wf_branch_feature.startIns = inswidth

    } else {
        wf_branch_feature.currentIns += inswidth
    }

    if wf_branch_feature.last_inst_is_branch {
        br_engine.Flush(wf_branch_feature,now,inst.PC)
        wf_branch_feature.last_inst_is_branch = false
    }
    flush_data := false
    if inst.FormatType == insts.SOPP  {
        switch inst.Opcode {
            case 2,4,5,6,7,8,9,10: // S_CBRANCH_SCC0
                wf_branch_feature.last_inst_is_branch = true
            case 1:
                flush_data = true
                wf_branch_feature.currentIns+=inswidth
            default:
        }
    }
    if flush_data {

        br_engine.Flush(wf_branch_feature,now,inst.PC)
    }
}

func ( br_engine *BranchSampledEngine ) Update( bbl profiler.BBL )  {
    rate,found := br_engine.bbl2rate[bbl]
    if found {
        br_engine.finish_rate += rate
        if (br_engine.finish_rate >= *BranchSampledThresholdFlag  ) {
            br_engine.enableSampled = true
            fmt.Printf("branch-level sampled start\n")
        } else {
            fmt.Printf("rate %.2f\n",br_engine.finish_rate)
        }
    }
}

