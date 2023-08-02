package profiler
import (
    "encoding/json"
    "os"
//    "log"
    "flag"
//    "fmt"
//	"gitlab.com/akita/mem/v3/vm"
//
//	"gitlab.com/akita/akita/v3/sim"
//	"gitlab.com/akita/mgpusim/v3/insts"
//	"gitlab.com/akita/mgpusim/v3/utils"
//
//	"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"
)

var InstsNumCollectFlag = flag.Bool("collect-instnum", false,
	"collect instnums in emulation.")



type InstsProfiler struct{
    Insts_num uint64 `json:"insts_num"`
    Wall_time float64 `json:"walltime"`
}
var Inst_profiler * InstsProfiler

func InitInstProfiler() {
    Inst_profiler = &InstsProfiler{
        Insts_num : 0,
    }
}

func (insts *InstsProfiler) Collect() {
    insts.Insts_num ++
}
func ReportInstsNum(walltime float64) {
    if *InstsNumCollectFlag {
        Inst_profiler.Wall_time = walltime
        jsonStr,_ := json.MarshalIndent( Inst_profiler,""," ")
        file, _ := os.Create("insnums.json")
        defer file.Close()
        file.Write(jsonStr)
    }
}

