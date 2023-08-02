package sampledrunner
import (
    "encoding/json"
    "os"
    "log"
    "flag"
    //"math"
    //"sort"
    "fmt"
//	"gitlab.com/akita/mem/v3/vm"
    "github.com/pkg/math"
	"gitlab.com/akita/akita/v3/sim"
//	"gitlab.com/akita/mgpusim/v3/utils"

	"gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mgpusim/v3/insts"
//	"gitlab.com/akita/mgpusim/v3/utils"
)
var KernelSampledFlag = flag.Bool("kernel-sampled", false,
	"kernel level sampled machanism.")
var KernelSampledThreshold = flag.Uint64("kernel-sampled-threshold", 32,
	"kernel level sampled machanism threshold.")
var KernelSampledDistanceThreshold = flag.Uint64("kernel-sampled-distance-threshold", 16,
	"kernel level sampled machanism distance threshold.")


type KernalSampledFeature struct {
    wffeature []* profiler.WfFeatureElem
}
type HistoryTable struct {
    Issuetime []sim.VTimeInSec `json:"issuetimes"`
    Finishtime []sim.VTimeInSec `json:"finishtimes"`
    Intervaltime []sim.VTimeInSec 
    Bbvs [] *profiler.OnlineBbv `json:"Bbvs"`
    history_queue []uint64
    Sampled bool
    Stable_time sim.VTimeInSec
    init bool  
    kernel_sampled_start_time sim.VTimeInSec 
}

func (history_table *HistoryTable ) Size(  ) uint64 {
    return uint64(len(history_table.Issuetime))
}
func (history_table *HistoryTable ) Predict( wfidx uint64 ) (sim.VTimeInSec,bool) {
    if wfidx < uint64( len( history_table.history_queue ) ){
        //return history_table.Finishtime[wfidx] - history_table.Issuetime[wfidx]
        return history_table.Intervaltime[wfidx],true
    } else {
        if history_table.init{
        fmt.Printf("Use stable time %.2f \n",history_table.Stable_time*1e9)
        history_table.init=false
        }
        return history_table.Stable_time,false
    }
}
func (history_table *HistoryTable ) Distance( history_queue []uint64 ) uint64{
    dis1 := len(history_table.history_queue)
    dis2 := len(history_queue)
    if dis1 < dis2 && ( !history_table.Sampled ){
        return *KernelSampledDistanceThreshold
    }
    dis := math.MinInt(dis1,dis2)
    distance := 0
    for i:= 0 ; i < dis ; i++ {
        if( history_table.history_queue[i] != history_queue[i] ) {
            distance ++
        }
    }
    fmt.Printf("distance %d\n",distance)
    return uint64(distance)
}


type  HistoryTables struct {
    bbv2id [][profiler.BbvDim]uint64 
    history_tables [] *HistoryTable
    history_table_to_predict *HistoryTable
    tableidx uint64
}
func InitHistoryTables() * HistoryTables {
    return &HistoryTables{
    }
}
func (history_tables *HistoryTables ) Size() int {
    return len(history_tables.history_tables)
}





func (history_tables *HistoryTables ) LoadHistoryTables( wffinalfeature *profiler.WfFinalFeature)  {
    if wffinalfeature == nil {
        return
    }
    if wffinalfeature.KernelSampledIdx != -1 {
        table2replace := history_tables.history_tables[wffinalfeature.KernelSampledIdx]

        if len( wffinalfeature.Issuetime) <= len( table2replace.Issuetime) {
            return 
        } else {
            finishtimes := wffinalfeature.Finishtime
            issuetimes := wffinalfeature.Issuetime
            intervaltimes := make([]sim.VTimeInSec,len(issuetimes))
            for idx,_ := range(finishtimes) {
                //intervaltimes_float64[idx] = float64(finishtimes[idx] - issuetimes[idx])
                intervaltimes[idx] = finishtimes[idx] - issuetimes[idx]
            }
            table2replace.Issuetime = issuetimes
            table2replace.Finishtime = finishtimes
            table2replace.kernel_sampled_start_time = wffinalfeature.KernelStart
            table2replace.Intervaltime = intervaltimes

            return
        }
    }
    issuetimes := wffinalfeature.Issuetime
    if uint64( len(issuetimes)) < *KernelSampledThreshold && (!wffinalfeature.Sampled){
        return
    }
    finishtimes := wffinalfeature.Finishtime
    intervaltimes := make([]sim.VTimeInSec,len(issuetimes))
  //  intervaltimes_float64 := make([]float64,len(issuetimes))
    for idx,_ := range(finishtimes) {
        //intervaltimes_float64[idx] = float64(finishtimes[idx] - issuetimes[idx])
        intervaltimes[idx] = finishtimes[idx] - issuetimes[idx]
    }
//    sort.Float64s(intervaltimes_float64)
    bbvs := wffinalfeature.Bbvs
    history_queue := history_tables.getHistoryQueue( bbvs )
    history_table := &HistoryTable{
        ///need to update
        Issuetime:issuetimes,
        Finishtime:finishtimes,
        kernel_sampled_start_time : wffinalfeature.KernelStart,
        Intervaltime:intervaltimes,
        //stable
        Bbvs:bbvs,
        history_queue:history_queue,
        Sampled : wffinalfeature.Sampled,
        Stable_time : wffinalfeature.Stable_time,
        init:true,
    }
    history_tables.history_tables = append(history_tables.history_tables,history_table)
}
func (history_tables *HistoryTables ) Predict( wfidx uint64 ) (sim.VTimeInSec,bool) {
    return history_tables.history_table_to_predict.Predict(wfidx)
}
func (history_tables *HistoryTables ) KernelSampledTime( ) sim.VTimeInSec {

    return history_tables.history_table_to_predict.kernel_sampled_start_time
}


func (history_tables *HistoryTables ) Issuetime( wfidx uint64 ) sim.VTimeInSec {

    return history_tables.history_table_to_predict.Issuetime[wfidx]
}


func (history_tables *HistoryTables ) getBbvId( bbv [profiler.BbvDim]uint64 ) uint64{

    equal := false
    for idx,elem := range(history_tables.bbv2id) {
        equal = true
        for i := 0 ; i < profiler.BbvDim ; i++ {
            if bbv[i] != elem[i] {
                equal = false
                break
            }
        }
        if equal {
            return uint64(idx)
        }
    }
    if !equal {
        history_tables.bbv2id = append( history_tables.bbv2id,bbv)
        return uint64( len(history_tables.bbv2id) - 1 )
    }
    panic("can not archive here")
    return 0
}

func (history_tables * HistoryTables ) Print( )  {
    log.Printf("history tables")
}
func (history_tables * HistoryTables ) getHistoryQueue(onlinebbvs []*profiler.OnlineBbv ) []uint64 {
    ret := make([]uint64,0)
    //log.Printf("%d",len(onlinebbvs))
    for _,elem := range(onlinebbvs ){
        bbvid := history_tables.getBbvId( elem.Bbv_vec )
        ret = append(ret , bbvid )
    }
    return ret
}


func (history_tables * HistoryTables) FindSuitableHistoryTable( onlinebbvs []*profiler.OnlineBbv ) bool {
    history_queue := history_tables.getHistoryQueue(onlinebbvs)
    for _,elem := range(history_queue) {
        fmt.Printf("%d ",elem)
    }
    fmt.Printf("\n")
    for idx, history_table := range(history_tables.history_tables ) {
        if history_table.Distance( history_queue ) < *KernelSampledDistanceThreshold {
            history_tables.tableidx = uint64( idx )
            history_tables.history_table_to_predict = history_table
            return true
        }
    }
    return false
}

func ( engine *KernelSampledEngine ) Issuetime( wfidx uint64 ) sim.VTimeInSec {
    return engine.history_tables.Issuetime(wfidx) + engine.offset
}
func (engine *KernelSampledEngine) SetOffset( now sim.VTimeInSec) {
    engine.offset = now - engine.history_tables.KernelSampledTime()
}
type KernelSampledEngine struct {
    history_tables * HistoryTables
    bbvs []*profiler.OnlineBbv
    wf2wffeature map[string]*profiler.WfFeatureElem
    enable_sampled bool
    stable bool
    predtime sim.VTimeInSec
    intervaltime2predict []sim.VTimeInSec
    offset sim.VTimeInSec 
}
func ( engine * KernelSampledEngine  ) search( history *HistoryTable,wfnum uint64 ) uint64 {
    Issuetimes      := history.Issuetime
    Finishtimes     := history.Finishtime
//    Intervaltimes   := history.Intervaltime
    issuesize := uint64( len(Issuetimes))
    if wfnum <= issuesize {
        return wfnum - 1
    } else {
        max_idx := int( issuesize )-1

        lastissuetime := Issuetimes[max_idx]
        max_idx = int(issuesize) - 1
        for idx := max_idx -2;idx>=0;idx--{
            if Finishtimes[idx] >= lastissuetime {
                max_idx = idx
            }
        }
        return uint64( max_idx )
    }

}
func ( engine * KernelSampledEngine ) HistorySize( ) int  {
    if engine.history_tables == nil {
        return 0
    }
    return engine.history_tables.Size()
}

func ( engine * KernelSampledEngine ) EnableSampled( ) bool  {
    return engine.enable_sampled
}
func ( engine * KernelSampledEngine ) CollectWfStart( wfid string, now sim.VTimeInSec )  {
    if !*KernelSampledFlag {
        return
    }
    wf_feature := profiler.InitWfFeatureElem(now)
    engine.wf2wffeature[wfid] =  wf_feature
}

func ( engine * KernelSampledEngine  ) Predict( wfidx uint64 ) (sim.VTimeInSec,bool) {
//    return engine.intervaltime2predict[wfidx]
    time, wait2issue := engine.history_tables.history_table_to_predict.Predict(wfidx)
    return time,wait2issue
}


func ( engine * KernelSampledEngine  ) Analysis( bbvs [] * profiler.OnlineBbv,numwf uint64 ) uint64 { //ret wf num to skip
    if *KernelSampledFlag {
    fmt.Printf("%d\n",len(bbvs))
    if uint64( len(bbvs) ) >= *KernelSampledThreshold {
        ret := engine.history_tables.FindSuitableHistoryTable( bbvs )
        if ret {
            engine.enable_sampled = true
            fmt.Printf( "history size %d", engine.history_tables.history_table_to_predict.Size() )
            skipwfs := engine.search( engine.history_tables.history_table_to_predict,numwf )
            fmt.Printf("numwf: %d skipwfs: %d\n",numwf,skipwfs)
            if profiler.Wffinalfeature != nil {
                profiler.Wffinalfeature.KernelSampledIdx = int( engine.history_tables.tableidx )
            }
            return skipwfs
        }
    }
    }
    return 0
}


func ( engine * KernelSampledEngine  ) CollectWfEnd( wfid string, now sim.VTimeInSec )  {
    if !*KernelSampledFlag {
        return
    }
    wf_feature , found := engine.wf2wffeature[wfid]
    if found {
        engine.bbvs = append( engine.bbvs, wf_feature.Bbv )
        delete(engine.wf2wffeature,wfid)
        if uint64( len(engine.bbvs) ) >= *KernelSampledThreshold {
            ret := engine.history_tables.FindSuitableHistoryTable( engine.bbvs )
            if ret {
                engine.enable_sampled = true
                return
            }
        }
    }
}
var LoadHistoryTableFlag = flag.String("load-history-table", "",
	"The path of history tables.")

func (engine *KernelSampledEngine) Reset(inited bool) {
    engine.wf2wffeature = make(map[string]*profiler.WfFeatureElem)
    engine.enable_sampled = false
    if (inited)  {
        engine.history_tables = InitHistoryTables()
    } else {
        engine.history_tables.LoadHistoryTables( profiler.Wffinalfeature )
    }

    engine.stable = false

}
var Kernelsampledengine * KernelSampledEngine
func InitKernelSampledEngine() {
    if *KernelSampledFlag {
        Kernelsampledengine = &KernelSampledEngine {
        }
        Kernelsampledengine.Reset(true)
        Kernelsampledengine.LoadHistoryTables()
        if profiler.Wffinalfeature != nil {
            profiler.Wffinalfeature.Reset()
        }
    }
}


func (engine *KernelSampledEngine ) LoadHistoryTables( ) {
    path := *LoadHistoryTableFlag
    if *KernelSampledFlag && path != "" {
        readData,_ := os.ReadFile( path )
        var wffinalfeature profiler.WfFinalFeature
        wffinalfeature.Reset()
        json.Unmarshal( []byte(readData),&wffinalfeature )
        engine.history_tables.LoadHistoryTables( &wffinalfeature )
    }

}



func (engine *KernelSampledEngine) Collect( wfid string, now sim.VTimeInSec, inst *insts.Inst)  {
    if !*KernelSampledFlag {
        return
    }
    wf_feature,_ := engine.wf2wffeature[wfid]
    wf_feature.Bbv.CountInst(inst)
}
