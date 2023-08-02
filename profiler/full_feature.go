package profiler
import (
	"encoding/json"
    "os"
)
type FullSim struct {
    Simtime float64 `json:"simtime"`
    Walltime float64 `json:"walltime"`
    Simtimes []float64 `json:"simtimes"`
    Walltimes []float64 `json:"walltimes"`
    Analysistimes []float64 `json:"analysistimes"`
    Analysistime float64
}
var Fullsim FullSim
func (fullsim * FullSim ) FFlush(simtime,walltime float64) {
    fullsim.Simtimes = append(fullsim.Simtimes,simtime)
    fullsim.Walltimes = append( fullsim.Walltimes,walltime )
    fullsim.Analysistimes = append( fullsim.Analysistimes, fullsim.Analysistime )
}
func ReportFullResult( simtime , walltime float64 ) {
    Fullsim.Simtime = simtime
    Fullsim.Walltime = walltime
    jsonStr,_ := json.MarshalIndent( Fullsim,""," ")
    file, _ := os.Create("result.json")
    defer file.Close()
    file.Write(jsonStr)

}

