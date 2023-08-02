package sampledrunner
import (
//	"gitlab.com/akita/mgpusim/v3/kernels"
//	"gitlab.com/akita/mgpusim/v3/profiler"
    "fmt"
    "encoding/json"
    "os"
    "time"
//    "math"
    "flag"
	"gitlab.com/akita/akita/v3/sim"
)








//func GetWGFeatureMem(  IDX int,  IDY int,  IDZ int ) *sampled.WGFeatureMem {
//    return Wavegroup_tensor.GetWavegroupFeature( IDX , IDY , IDZ)
//}


var SampledRunnerFlag = flag.Bool("sampled", false,
	"sampled execution.")
var SampledRunnerThresholdFlag = flag.Float64("sampled-threshold", 0.03,
	"sampled execution threshold.")



type WFFeature struct {
    Issuetime sim.VTimeInSec
    Finishtime sim.VTimeInSec
}

type StableEngine struct {
    issuetime_sum sim.VTimeInSec
    finishtime_sum sim.VTimeInSec
    intervaltime_sum sim.VTimeInSec
    mix_sum sim.VTimeInSec
    issuetime_square_sum sim.VTimeInSec
    rate float64
    granulary int
    Wffeatures [] WFFeature
    boundary float64
    enableSampled bool
    predTime sim.VTimeInSec
}
func (stable_engine *StableEngine) Analysis(  ) {

        rate_bottom := sim.VTimeInSec(stable_engine.granulary) * stable_engine.issuetime_square_sum - stable_engine.issuetime_sum * stable_engine.issuetime_sum 
        rate_top    := sim.VTimeInSec( stable_engine.granulary) *  stable_engine.mix_sum - stable_engine.issuetime_sum * stable_engine.finishtime_sum 
        rate := float64( rate_top / rate_bottom)
        stable_engine.rate = rate
        boundary := stable_engine.boundary
        stable_engine.predTime = stable_engine.intervaltime_sum / sim.VTimeInSec(stable_engine.granulary)
        if  (rate >= (1 - boundary) && rate <= (1+boundary)  ){
            stable_engine.enableSampled = true
//            endtime := time.Now()
//            duration := endtime.Sub(sampled_engine.FullSimWalltimeStart)
//            duration_seconds := duration.Seconds()
//            fmt.Printf("\ndetailed simulation time : %.2f predict time %.2f dataidx %d\n", duration_seconds,sampled_engine.predTime*1e9, sampled_engine.dataidx)
       //     fmt.Printf("rate %.2f\n",rate)
        } else {
            stable_engine.enableSampled = false
//            fmt.Printf("rate %.2f\n",rate)
        }
}

func (stable_engine *StableEngine) Reset() {
    stable_engine.Wffeatures = nil
    stable_engine.issuetime_sum = 0
    stable_engine.finishtime_sum = 0
    stable_engine.intervaltime_sum = 0
    stable_engine.mix_sum = 0
    stable_engine.issuetime_square_sum = 0
    stable_engine.predTime = 0
    stable_engine.enableSampled = false
}




func (stable_engine *StableEngine) Collect( issuetime, finishtime sim.VTimeInSec ) {
    wffeature := WFFeature {
        Issuetime : issuetime,
        Finishtime : finishtime,
    }

    stable_engine.Wffeatures = append( stable_engine.Wffeatures, wffeature )
    stable_engine.issuetime_sum += issuetime
    stable_engine.finishtime_sum += finishtime
    stable_engine.mix_sum += finishtime * issuetime
    stable_engine.issuetime_square_sum += issuetime * issuetime
    stable_engine.intervaltime_sum += ( finishtime - issuetime )

    if len(stable_engine.Wffeatures) == stable_engine.granulary {
        stable_engine.Analysis()
        ///delete old data
        wffeature2 := stable_engine.Wffeatures[0]
        stable_engine.Wffeatures = stable_engine.Wffeatures[1:]
        issuetime = wffeature2.Issuetime
        finishtime = wffeature2.Finishtime
        stable_engine.issuetime_sum -= issuetime
        stable_engine.finishtime_sum -= finishtime
        stable_engine.mix_sum -= finishtime * issuetime
        stable_engine.issuetime_square_sum -= issuetime * issuetime
        stable_engine.intervaltime_sum -= ( finishtime - issuetime )
    }
}


type SampledEngine struct {
    predTime sim.VTimeInSec
    enableSampled bool
    disableEngine bool
    Simtime float64 `json:"simtime"`
    Walltime float64 `json:"walltime"`
    FullSimWalltime float64 `json:"fullsimwalltime"`
    FullSimWalltimeStart time.Time
    datanum uint64
    dataidx uint64
    stable_engine * StableEngine
    short_stable_engine * StableEngine
    predTimeSum sim.VTimeInSec
    predTimeNum uint64
    granulary int
}

func (sampled_engine *SampledEngine) Reset() {
    sampled_engine.FullSimWalltimeStart = time.Now()    
    sampled_engine.stable_engine.Reset()
    sampled_engine.short_stable_engine.Reset()
    sampled_engine.predTime = 0
    sampled_engine.predTimeNum = 0
    sampled_engine.predTimeSum = 0
    sampled_engine.dataidx = 0
    sampled_engine.enableSampled = false
    sampled_engine.disableEngine = true
}



func ReportSampledResult( simtime , walltime float64 ) {
    if *SampledRunnerFlag || *BranchSampledFlag {
        
    Sampledengine.Simtime = simtime
    Sampledengine.Walltime = walltime
    jsonStr,_ := json.MarshalIndent( Sampledengine,""," ")
    file, _ := os.Create("sampled_result.json")
    defer file.Close()
    file.Write(jsonStr)
    }

}
//const granulary = 512
func NewSampledEngine( granulary int, boundary float64, control bool  ) *SampledEngine {

    stable_engine := &StableEngine {
        granulary : granulary,
        boundary : boundary,
    }
    short_stable_engine := &StableEngine {
        granulary : granulary/2,
        boundary:boundary,
    }
    ret := &SampledEngine {
        stable_engine : stable_engine,
        short_stable_engine : short_stable_engine,
        granulary:granulary/2,
    }
    ret.Reset()
    if control {
        ret.disableEngine = false
    }
    return ret
}

var Sampledengine * SampledEngine 
func InitSampledEngine() {
    Sampledengine = NewSampledEngine (2048,*SampledRunnerThresholdFlag,false)
}

func (sampled_engine *SampledEngine) Update( issuetime sim.VTimeInSec , finishtime sim.VTimeInSec ) bool {
    sampled_engine.short_stable_engine.Collect(issuetime,finishtime)
    if sampled_engine.short_stable_engine.enableSampled{
        sampled_engine.predTime = sampled_engine.short_stable_engine.predTime 
    }

//    sampled_engine.predTimeSum += (finishtime - issuetime )
//    sampled_engine.predTimeNum ++
//    sampled_engine.predTime = sampled_engine.predTimeSum / sim.VTimeInSec( sampled_engine.predTimeNum)
    return true
}

func (sampled_engine *SampledEngine) Disabled(  ) {
    sampled_engine.disableEngine = true 
}
func (sampled_engine *SampledEngine) Enable(  ) {
    fmt.Printf("%B\n",sampled_engine.disableEngine)
    sampled_engine.disableEngine = false
    fmt.Printf("%B\n",sampled_engine.disableEngine)
}
func (sampled_engine *SampledEngine) IfDisable(  )bool {
    return sampled_engine.disableEngine 
}

func (sampled_engine *SampledEngine) Collect( issuetime sim.VTimeInSec , finishtime sim.VTimeInSec ) {
    if sampled_engine.enableSampled || sampled_engine.disableEngine {
        return
    }

    sampled_engine.dataidx++
    if (sampled_engine.dataidx < 1024) { // discard first 1024 data
        return
    }

    sampled_engine.stable_engine.Collect( issuetime,finishtime )
    sampled_engine.short_stable_engine.Collect( issuetime,finishtime )
    stable_engine := sampled_engine.stable_engine
    short_stable_engine := sampled_engine.short_stable_engine
//    if stable_engine.enableSampled && short_stable_engine.enableSampled {
    if stable_engine.enableSampled {
//        sampled_engine.enableSampled = true
        long_time := stable_engine.predTime
        short_time := short_stable_engine.predTime
        sampled_engine.predTime = short_stable_engine.predTime
        diff := float64( (long_time - short_time) / (long_time + short_time))
        
        diff_boundary := *SampledRunnerThresholdFlag
        if diff <= diff_boundary && diff >= -diff_boundary {
            sampled_engine.enableSampled = true
            //sampled_engine.predTime = (long_time + short_time) / 2.0;
            sampled_engine.predTime = short_time;
            sampled_engine.predTimeSum  = short_time * sim.VTimeInSec(sampled_engine.granulary)
            sampled_engine.predTimeNum  = uint64(sampled_engine.granulary)
            fmt.Printf("long %.2f short %.2f relatively diff %.2f \n",stable_engine.predTime*1e9, short_stable_engine.predTime*1e9,diff)
        }
        
    } else if short_stable_engine.enableSampled {
        sampled_engine.predTime = stable_engine.predTime
    }
//    else if stable_engine.enableSampled{
//        sampled_engine.predTime = stable_engine.predTime
//    } else if short_stable_engine.enableSampled{
//
//        sampled_engine.predTime = short_stable_engine.predTime
////        fmt.Printf("long %.2f short %.2f\n",stable_engine.rate,short_stable_engine.rate)
//    }

}

func (sampled_engine *SampledEngine) DebugPrint(  ) {
    fmt.Printf( "%t\n", sampled_engine.enableSampled)
}
func (sampled_engine *SampledEngine) Predict(  ) (  sim.VTimeInSec ,bool) {

    //if sampled_engine.enableSampled 
    {
//        fmt.Printf("predict %t \n",sampled_engine.enableSampled)
    }
    return  sampled_engine.predTime,sampled_engine.enableSampled
}

