package runner

import (
//	"github.com/tebeka/atexit"
	"gitlab.com/akita/akita/v3/tracing"
    "gitlab.com/akita/mgpusim/v3/timing/wavefront"
	"gitlab.com/akita/akita/v3/sim"
//	"gitlab.com/akita/mgpusim/v2/timing/cu"
//    "fmt"
)
//////////////////////WGFeatureTracer
type WGFeature struct {
    count uint64
    IDX int
    IDY int
    IDZ int
}
type WGFeatureTracer struct {
    wgExecTime []float64
    wgStartTime []float64
    wgEndTime []float64
    wgInstNum  []uint64
	inflightInst map[string]tracing.Task
	wgFeature map[string] *WGFeature
  //  wfs_id [] string
    IDXs []int
    IDYs []int
    IDZs []int
}




func newWGFeatureTracer() *WGFeatureTracer {
	t := &WGFeatureTracer{
		inflightInst: map[string]tracing.Task{},
		wgFeature:    map[string]*WGFeature{},
	}
	return t
}
func (t *WGFeatureTracer) StartTask(task tracing.Task) {
	if task.Kind != "wg_feature" {
		return
	}

    wg_init := task.Detail.(map[string]interface{})["wg"].(*wavefront.WorkGroup)
    now := task.Detail.(map[string]interface{})["now"].(sim.VTimeInSec)
    task.StartTime = now
//    wfs := &wg_init.Wfs
//    for _,wf := range *wfs {
//        t.wfs_id = append(t.wfs_id, wf.UID)
//    }
    idx := wg_init.IDX
    idy := wg_init.IDY
    idz := wg_init.IDZ

	t.inflightInst[task.ID] = task
    t.wgFeature[task.ID] =  &WGFeature{
        count : 0,
        IDX : idx,
        IDY : idy,
        IDZ : idz,
    }
}

func (t *WGFeatureTracer) StepTask(task tracing.Task) {
	// Do nothing
	data, found := t.wgFeature[task.ID]
	if !found {
		return
	}
    data.count++

}

func (t *WGFeatureTracer) EndTask(task tracing.Task) {
	data, found := t.inflightInst[task.ID]
	if !found {
		return
	}
    startTime := data.StartTime
	delete(t.inflightInst, task.ID)
    wgInstNum := t.wgFeature[task.ID].count
    idx := t.wgFeature[task.ID].IDX
    idy := t.wgFeature[task.ID].IDY
    idz := t.wgFeature[task.ID].IDZ
    delete(t.wgFeature, task.ID)
    now := task.Detail.(map[string]interface{})["now"].(sim.VTimeInSec)

    task.EndTime = now
//    cu_init := data.Detail.(map[string]interface{})["cu"].(*cu.ComputeUnit)
    //fmt.Printf("ddd\n ")
//    fmt.Printf( "%s : %lf\n ",task.ID, task.EndTime- startTime)
    t.wgExecTime = append(t.wgExecTime,float64(task.EndTime - startTime))
    t.wgStartTime = append(t.wgStartTime,float64(startTime))
    t.wgEndTime = append(t.wgEndTime,float64(task.EndTime))
    t.wgInstNum  = append(t.wgInstNum, wgInstNum )
    t.IDXs  = append(t.IDXs, idx )
    t.IDYs  = append(t.IDYs, idy )
    t.IDZs  = append(t.IDZs, idz )

}

