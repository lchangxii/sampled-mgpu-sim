package sampledrunner
import (
//	"gitlab.com/akita/mgpusim/v3/kernels"
//	"gitlab.com/akita/mgpusim/v3/profiler"
    "log"
    "fmt"
    "math"
    "flag"
    "os"
    "encoding/json"
	"gitlab.com/akita/akita/v3/sim"
	"github.com/tebeka/atexit"
)

var IPCSampledRunnerFlag = flag.Bool("ipc-sampled", false,
	"ipc sampled execution.")

var IPCSampledThresholdFlag = flag.Float64("ipc-sampled-threshold", 0.25,
	"ipc sampled execution threshold.")

const ipc_granulary uint64=3000
const frequency=1000000000
type IPCSampledEngine struct {
    instnums [ipc_granulary] uint64
    last_cycle uint64
    End_cycle uint64 `json:"end_cycle"`
    Start_cycle uint64 `json:"start_cycle"`
    interval_insts_sum uint64
    Insts_sum uint64  `json:"insts_sum"`
    Avg_IPC float64 `json:"IPC"`
    Frequency float64 `json:"freq"`
    Inited bool `json:"inited"`
    ipc_sum float64
    ipc_queue []float64
    Walltime float64 `json:"walltime"`
}

var IPC_sampled_engine * IPCSampledEngine

func InitIPCSampledEngine() *IPCSampledEngine{
    ret := &IPCSampledEngine {
        last_cycle : 0,
        Insts_sum : 0,
        interval_insts_sum : 0,
        Inited : false,
    }
    for idx := uint64( 0 ); idx < ipc_granulary ; idx++ {
        ret.instnums[idx] = 0
    }
    IPC_sampled_engine = ret
    return ret
}
func (v * IPCSampledEngine) DebugPrint( format string , args ...interface{} ) {
//        return;
fmt.Printf("")
    log.Printf( format , args... ) 
}


func ReportSampledIPCResult(walltime float64 ) {
        IPC_sampled_engine.Walltime = walltime
        jsonStr,_ := json.MarshalIndent( IPC_sampled_engine,""," ")
        file, _ := os.Create("ipc_sampled_result.json")
        defer file.Close()
        file.Write(jsonStr)
}
func (sampled_engine *IPCSampledEngine) Collect( now sim.VTimeInSec, insnum uint32 ) {
    if !*IPCSampledRunnerFlag {
        return
    }
    cycle := uint64( now * frequency )

    sampled_engine.End_cycle = cycle
    sampled_engine.Insts_sum += uint64( insnum )
//    sampled_engine.DebugPrint( "cycle %d ", cycle)
    if !sampled_engine.Inited {
        sampled_engine.Inited = true
        sampled_engine.Start_cycle =  cycle
        sampled_engine.Frequency =  1/float64( frequency)
        sampled_engine.last_cycle = cycle
        idx_round := cycle % ipc_granulary
        sampled_engine.instnums[idx_round] = uint64( insnum )
        sampled_engine.interval_insts_sum = uint64( insnum )
    } else { 
        
        if cycle == sampled_engine.last_cycle {
            idx_round := cycle % ipc_granulary
            sampled_engine.instnums[idx_round] += uint64 ( insnum)
            sampled_engine.interval_insts_sum += uint64( insnum)
        } else {
            if ( cycle - sampled_engine.Start_cycle) >= ipc_granulary {
                avg_ipc := float64( sampled_engine.interval_insts_sum) / float64( ipc_granulary )

                const inter_boxes = 1 //all history
                const intra_boxes = 2 //one history
                const choosed = intra_boxes
//                const choosed = inter_boxes
                if  choosed == intra_boxes {
                    sum := 0.0
                    for _,ipc := range(sampled_engine.instnums) {
                        ipc_float :=  float64( ipc )
                        sum += ( ipc_float - avg_ipc ) * ( ipc_float - avg_ipc )
                    }
                    std := math.Sqrt( sum / float64( ipc_granulary  ) ) 
                    std = std / avg_ipc
        //            sampled_engine.DebugPrint( "%.2f  %.2f",  float64(sampled_engine.insts_sum)/float64(ipc_granulary), std )
                    if std <= *IPCSampledThresholdFlag {
                        sampled_engine.Avg_IPC = avg_ipc
                        sampled_engine.Frequency = 1.0/float64( frequency )
                        atexit.Exit(0)
                    }
                } else{
                  sampled_engine.ipc_queue = append(sampled_engine.ipc_queue,avg_ipc)
                  sampled_engine.ipc_sum += avg_ipc
                    sum := 0.0
                  avg_allipc := sampled_engine.ipc_sum / float64( len(sampled_engine.ipc_queue) )

                    for _,ipc_float := range(sampled_engine.ipc_queue) {
                         
                        sum += ( ipc_float - avg_allipc ) * ( ipc_float - avg_allipc )
                    }
                    std := math.Sqrt( sum / float64( len(sampled_engine.ipc_queue)  ) )
               
                    sampled_engine.DebugPrint( "%.2f  %.2f",  float64(sampled_engine.interval_insts_sum)/float64(ipc_granulary), std )
                }
            }


            ////reset these cycles without instructions
            for idx := sampled_engine.last_cycle + 1; idx < cycle ; idx++  {
                idx_round := idx % ipc_granulary
                sampled_engine.interval_insts_sum -= sampled_engine.instnums[idx_round]
                sampled_engine.instnums[idx_round] = 0
            }
            idx_round := cycle % ipc_granulary

            sampled_engine.interval_insts_sum -=  sampled_engine.instnums[idx_round]

            sampled_engine.instnums[idx_round] = uint64 ( insnum)
            sampled_engine.interval_insts_sum += uint64( insnum)
            sampled_engine.last_cycle = cycle
        }
    }


}



