package profiler
import (
    "encoding/json"
    "os"
    "flag"
    "log"
    "fmt"
//    "runtime"
//	"gitlab.com/akita/mem/v3/vm"

	"gitlab.com/akita/akita/v3/sim"
//	"gitlab.com/akita/mgpusim/v3/insts"
//	"gitlab.com/akita/mgpusim/v3/utils"

	//"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"
)

type InstCount struct {
    InstNums []uint64 `json:"insnums"`
    Cycles []sim.VTimeInSec `json:"cycles"`
    global_inscount uint64
    current_time sim.VTimeInSec

    IssueInstNums []uint64 `json:"issueinsnums"`
    IssueCycles []sim.VTimeInSec `json:"issuecycles"`
    global_issue_inscount uint64
    current_issuetime sim.VTimeInSec
}

var Instcount * InstCount
var ReportIPCFlag = flag.Bool("collect-ipc", false,
	"Collect IPC along cycle.")
func (v *InstCount) Name() string {
    return "InstCount"
}
func (v * InstCount) DebugPrint( format string , args ...interface{} ) {
//        return;

    log.Printf( " %s " , v.Name() ) 
    fmt.Printf( format ,  args...)
    fmt.Printf( " \n " ) 
}


func (inscount *InstCount) Count(now sim.VTimeInSec, insnum uint32) {

//    inscount.DebugPrint("%t", *reportIPC)
    if !*ReportIPCFlag {
        return
    }
    if now != inscount.current_time {
        inscount.InstNums = append(inscount.InstNums,inscount.global_inscount)
        inscount.Cycles = append(inscount.Cycles, now)
        inscount.current_time = now
    }
    inscount.global_inscount+= uint64(insnum)
}
func (inscount *InstCount) CountIssue(now sim.VTimeInSec, insnum uint32) {

//    inscount.DebugPrint("%t", *reportIPC)
    if !*ReportIPCFlag {
        return
    }
    if now != inscount.current_issuetime {

        inscount.IssueInstNums = append(inscount.IssueInstNums,inscount.global_issue_inscount)
        inscount.IssueCycles = append(inscount.IssueCycles, now)
        inscount.current_issuetime = now
    }
    inscount.global_issue_inscount+= uint64(insnum)
}

func  InitInstCount( ) {
    Instcount = &InstCount {
        global_inscount : 0 ,
        global_issue_inscount :0,
        current_time : sim.VTimeInSec(0),
        current_issuetime : sim.VTimeInSec(0),
    }
}

func ReportIPC() {
    if *ReportIPCFlag {
    //    jsonStr,_ := json.Marshal(WgFeatureVector)
        jsonStr,_ := json.MarshalIndent( Instcount,""," ")
        file, _ := os.Create("inscount.json")
        defer file.Close()
        file.Write(jsonStr)
    }

}



