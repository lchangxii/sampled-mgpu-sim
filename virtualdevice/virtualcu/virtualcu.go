package virtualcu

import (
    //"gitlab.com/akita/mem/v3/mem"
	//"gitlab.com/akita/mem/v3/vm/tlb"

//	"gitlab.com/akita/mgpusim/v3/timing/cu"
//	"gitlab.com/akita/mgpusim/v3/kernels"
	"gitlab.com/akita/mgpusim/v3/timing/wavefront"
	//"gitlab.com/akita/mgpusim/v3/utils"
	"gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mgpusim/v3/insts"
	//"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualtlb"
	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcache"
	"gitlab.com/akita/mgpusim/v3/virtualdevice"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/akita/v3/sim"
    "fmt"
    "log"
    //"os"
    "math"
    "reflect"
     "runtime"
)

type Builder struct {
    freq sim.Freq
    engine sim.Engine
}
func MakeBuilder(  ) Builder {
    return Builder{
    }
}
func (b Builder) WithEngine( engine sim.Engine) Builder {
	b.engine = engine
	return b
}
func (b Builder) WithFreq( freq sim.Freq) Builder {
	b.freq = freq
	return b
}





const allThreashold = 1e-8 

type Feature struct {
    Intervaltime [] sim.VTimeInSec
    Realtime sim.VTimeInSec
}

func (rf *Feature) equal( lf *Feature ) (float64,bool ){
    sum_diff := 0.0
    //fmt.Printf("%d %d\n",len(rf.Intervaltime),len(lf.Intervaltime))
    if len(rf.Intervaltime) == len(lf.Intervaltime) {
        for idx, elem := range rf.Intervaltime {
            lf_elem := lf.Intervaltime[idx]
   //     fmt.Printf("%10.20f,%10.20f\n",elem,lf_elem)
            diff := math.Abs( float64( elem - lf_elem )  )
            sum_diff += diff
        }
    } else {
        sum_diff = allThreashold
    }

    if( sum_diff > allThreashold * float64( len(rf.Intervaltime) ) ){
        return sum_diff,false
    } else {
        return sum_diff,true
    }
}

const debug_end = 2
type RestoreStatus int
const (
    FromVL1TLB RestoreStatus = iota
    FromVL1
    FromSL1
    FromSL1TLB
    FromWfStart
    FromALU
    RestoreStatusCount
)
type InflightWFFeature struct{
    inst_idx uint32
    issue_time sim.VTimeInSec
    finish_time sim.VTimeInSec
    wffeature * profiler.WavefrontFeature 
    restore_status RestoreStatus
//    reqsStatus  map[*virtualdevice.Request] RestoreStatus
}

type VirtualCU struct {
    *sim.TickingComponent
    real_device virtualdevice.RealComponent

    Virtuall1vrob * virtualcache.VirtualROB
    Virtuall1srob * virtualcache.VirtualROB
    Virtuall1irob * virtualcache.VirtualROB
    Wf2feature map[string] Feature
    Wf2realfeature map[string] Feature
    InflightSampledWF  map[string] *InflightWFFeature
//    Inflightwffeature map[string] * InflightWFFeature
//    cu * cu.ComputeUnit 
    count int
    debug_idx int
    smem_pipelinestagenum int
    vmem_pipelinestagenum int
    vmem_postpipelinestagenum int
    smem_postpipelinestagenum int
    preparewflatency int

    wavefront_queue []*virtualdevice.Request
    
    //just for performance
	dramVirtualDrams        []*virtualcache.VirtualDRAM
}


func (c *VirtualCU) Handle( e sim.Event ) error {
    switch e := e.(type) {
        case *virtualdevice.RecoverWfEvent:
            c.handleRecoverWfEvent(e)
        case sim.TickEvent:
            return c.TickingComponent.Handle(e)
        default:
		    log.Panicf("cannot handle event of %s", reflect.TypeOf(e))
    }
    return nil
}


func (c *VirtualCU) SetRealComponent( realdevice virtualdevice.RealComponent) {

    c.real_device = realdevice
}
func (c *VirtualCU) SetVirtualDramSet( vtdrams [] *virtualcache.VirtualDRAM ) {
    c.dramVirtualDrams = vtdrams
}




func (vc *VirtualCU) Setl1vrob( para * virtualcache.VirtualROB ) {
    vc.Virtuall1vrob = para;
}
func (vc *VirtualCU) Setl1irob( para * virtualcache.VirtualROB ) {
    vc.Virtuall1irob = para;
}
func (vc *VirtualCU) Setl1srob( para * virtualcache.VirtualROB ) {
    vc.Virtuall1srob = para;
}





func (vc * Builder)  Build(name string) *VirtualCU {
    vcu := &VirtualCU {
//        Realcu : realcu,
        Wf2feature : make ( map[string] Feature),
        Wf2realfeature : make ( map[string] Feature),
        InflightSampledWF : make( map[string] *InflightWFFeature ),
        vmem_pipelinestagenum : 70-8+2,
        smem_pipelinestagenum : 0,
        vmem_postpipelinestagenum : 2,
        smem_postpipelinestagenum : 0,
        preparewflatency : 12,
    }
	vcu.TickingComponent = sim.NewTickingComponent(
		name, vc.engine, vc.freq, vcu)
    return vcu;
}

func (vtlb *VirtualCU)Arbitrate() *virtualdevice.Request {
    if len(vtlb.wavefront_queue) == 0 {
        return nil
    }
    ret := vtlb.wavefront_queue[0]
    vtlb.wavefront_queue = vtlb.wavefront_queue[1:]
    return ret
}

var stable_var int 

func (v *VirtualCU) handleRecoverWfEvent(e * virtualdevice.RecoverWfEvent) {
    req := e.Req
    req.RecoverTime = e.Time()
 //   req.Now = sim.VTimeInSec(math.Max( float64(e.Time()),float64(req.Now)))
    finishtime , success :=  v.IntervalModel( req  )
    if success {
        v.DebugPrint( "%.2f %.2f\n", e.Time() * 1e9, finishtime * 1e9 )
        newEvent := wavefront.NewWfCompletionEvent(
			    finishtime  , v.real_device, req.Wf)
        	
        v.Engine.Schedule(newEvent)

//        stable_var +=1
//        if stable_var == 2 {
//            v.DebugExit()
//        }
    }
//    return
}

func (v * VirtualCU) Tick(now sim.VTimeInSec) bool {
    	
    return true
}


const discardNum = 32
func (vc * VirtualCU) RecordSimulate( id string , now sim.VTimeInSec ) bool {
    return true
//    _,found := vc.InflightSampledWF[ id ]
//    if found {
//        
//        delete(vc.InflightSampledWF,id)
//        return true
//    } else {
//        feature,found := vc.Wf2feature[id]
//        if !found {
//            return false
//        }
//        if vc.count < discardNum{
//            vc.count++
//            delete( vc.Wf2feature, id)
//            return false
//        }
//
//        feature.Realtime = now - feature.Realtime
//        vc.Wf2realfeature[id] = feature
//        
// //       fmt.Printf("record %s %10.20f\n", id, feature.Realtime)
//        delete( vc.Wf2feature, id)
//        return false
//    }
}

func (vc * VirtualCU) Search(  feature_in * Feature )( sim.VTimeInSec , bool) {
    predicted_time := sim.VTimeInSec(0)
    has_data := false
    last_diff := allThreashold
    for _, feature := range vc.Wf2realfeature {
        diff , found := feature.equal( feature_in )

    //    fmt.Printf("%s %10.20f  %10.20f\n",id, diff, feature.Realtime)
        if found && diff < last_diff {
            has_data = true
            predicted_time = feature.Realtime
            last_diff = diff
//            fmt.Printf( "%10.20f %10.20f", diff,predicted_time )
        } 
    }
//    if has_data {
//        panic("No thing")
//    }
    return predicted_time, has_data
}


func max( a,b sim.VTimeInSec ) sim.VTimeInSec {
    if( a > b ) {
        return a
    } else {
        return b
    }
}

func printTime( now sim.VTimeInSec) string {

    return fmt.Sprintf( "%.2f", now * 1e9)
}
func (v *VirtualCU) NewBottomReadyEvent(  req *virtualdevice.Request ) {
        time := req.RecoverTime
        new_event := &virtualdevice.RecoverWfEvent { 
            sim.NewEventBase( v.Freq.NextTick( time) , v),
            req,
        }
        v.Engine.Schedule(new_event)
}

func (vc *VirtualCU)handleALU( issuetime sim.VTimeInSec, finishtime  sim.VTimeInSec , interval int ) ( sim.VTimeInSec, sim.VTimeInSec) {
    issuetime = vc.Freq.NCyclesLater( interval , issuetime)
    finishtime = max(issuetime,finishtime)
    return issuetime,finishtime
}

    //globalFinishTime := sim.VTimeInSec(0)
                        //    Fetch   Issue
    const schedule_interval = 1     + 1     
                                                             //decoder   read_register  exec     write    send_request    unknowTimeInterval
                                         //scalar
    const exeunitscalar     =     1          + 1            + 1    + 1
    const exeunitsmem       =     1          + 1            + 1//?
                                                            //vector
    const exeunitvalu       =     1          + 4
                                                            //lds                      exec
                                                                    //SetLDS
    const exeunitlds        =     schedule_interval +        1          + 1            + 1     + 1

    const exeunitbranch     =      0          + 1            + 1     + 1 + 11 
                                                            //Mem         InstructionPipeline                           transaction                           access
    const exeunitvmem       =      2          + 6                  //can issue next inst   // 1(send) + ? + 1 (recv)   ? + 1(send)  1(send) + ? + 1 (recv)



func (vc * VirtualCU) IntervalModel( req *virtualdevice.Request ) ( sim.VTimeInSec,bool ) {
//    wfid := req.Wfid
    pid := req.Pid
    now := req.Now
    recoverTime := req.RecoverTime

    wavefront_feature := req.AddtionalInfo[0].(*profiler.WavefrontFeature)
    
    if len( req.AddtionalInfo ) == 1 { //first be executed
        execute_status := &virtualdevice.ExecuteStatus{
        }
        req.AddtionalInfo = append(req.AddtionalInfo, execute_status )
    }

    execute_status := req.AddtionalInfo[1].(*virtualdevice.ExecuteStatus)

    inst_types := & wavefront_feature.Inst_types
    inst_idx := execute_status.Inst_idx
    var issuetime sim.VTimeInSec
    var finishtime sim.VTimeInSec
    var recover_mode bool
    if req.RecoverMode { // load instruction
        tmp := vc.Freq.NCyclesLater( vc.preparewflatency, now )
        issuetime = tmp
        finishtime = tmp
        recover_mode = false
    } else {
        issuetime = execute_status.Issuetime
        finishtime = execute_status.Finishtime
        recover_mode = true
    }

    //issuetime := flight_wf_feature.issuetime
//    finishtime := flight_wf_feature.finishtime
 //   inst_idx := flight_wf_feature.inst_idx
//    restore_status := flight_wf_feature.restore_status    
    inst_features := & wavefront_feature.Inst_features
    execute_success := true
    continue_execute := true
    for idx := inst_idx ; (idx < len(*inst_types)) && continue_execute ; idx++ {
        inst_type := (*inst_types)[ idx ]

        inst_feature := &(*inst_features)[idx]
//        issuetime += vc.Virtuall1icache.IntervalModel( inst_feature,pid,now )
        //continue_interval_simulate := true

        switch inst_type {
        case insts.SOP1, insts.SOP2,  insts.SOPC: // ExeUnitScalar

            issuetime,finishtime = vc.handleALU( issuetime, finishtime, exeunitscalar )
    	case insts.VOP1, insts.VOP2,insts.VOP3a, insts.VOP3b, insts.VOPC: //ExeUnitVALU

            issuetime,finishtime = vc.handleALU( issuetime, finishtime, exeunitvalu )


    	case insts.SOPP: // ExeUnitBranch  0,1,10,11,12,13,14,15,18.19.20,21,22,27,28,29 ExeUnitSpecial
        {
            if  inst_feature.Opcode == 12 {
                if recover_mode {
//                    finishtime = execute_status.Finishtime
//                    issuetime = execute_status.Issuetime
//vc.DebugPrint("Recover: fini %.2f issue %.2f",finishtime*1e9,issuetime*1e9)
                    execute_status.Reset()
                    recover_mode = false

                } else {
                    finishtime = vc.Freq.NextTick( finishtime )
                    issuetime = finishtime
                    execute_status.Inst_idx = idx
                    execute_status.Finishtime = finishtime
                    execute_status.Issuetime = issuetime
                    execute_success = false
                    continue_execute = false

//vc.DebugPrint("Stall: fini %.2f issue %.2f",finishtime*1e9,issuetime*1e9)
                }

    //vc.DebugPrint("execute %.2f %t\n", (simtime-now)*1e9, success)
            } else {
                if inst_feature.Opcode == 1 {
                    finishtime = vc.Freq.NextTick( finishtime )
                    issuetime = finishtime

                } else {
                    issuetime,finishtime = vc.handleALU( issuetime, finishtime, exeunitbranch )
                }
            }
        }   
    	case insts.SOPK: //ExeUnitScalar 
            issuetime,finishtime = vc.handleALU( issuetime, finishtime, exeunitscalar )
           
    	case insts.DS: //ExeUnitLDS
            issuetime,finishtime=vc.handleALU( issuetime, finishtime, exeunitlds )
    	case insts.FLAT: //ExeUnitVMem 
            var issuetime2 sim.VTimeInSec
            var rettime sim.VTimeInSec
            mem_success := true
            if ! recover_mode {
                var issuetime2 sim.VTimeInSec
                issuetime2  = vc.Freq.NCyclesLater( exeunitvmem , issuetime )
                request := & virtualdevice.Request{
                    Now : issuetime2,
                    Pid : pid,
                }
                request.AddtionalInfo = append(request.AddtionalInfo, inst_feature )
                request.AddtionalInfo = append(request.AddtionalInfo,  req.AddtionalInfo[1])

                switch  inst_feature.Opcode {
                    case 16,18,20,21,23:
                        request.Memtype = virtualdevice.MemRead
                    case 28,29,30,31:
                        request.Memtype = virtualdevice.MemWrite
                
                }
                rettime, mem_success = vc.Virtuall1vrob.IntervalModel( request)
            } else {
                issuetime2 = issuetime
                rettime = finishtime
                mem_success = true
            }
            if mem_success {
                issuetime = issuetime2
                finishtime_tmp := rettime
                finishtime = max( finishtime_tmp , finishtime )
            } else {
                execute_status.Inst_idx = idx
                execute_status.Finishtime = finishtime
                execute_status.Issuetime = issuetime
                execute_success = false            
                continue_execute = false
                
            }

        case insts.SMEM: //ExeUnitScalar
            //issuetime  += exeunitsmem
            var issuetime2,rettime sim.VTimeInSec
            var smem_success bool
            if !recover_mode {
                issuetime2  = vc.Freq.NCyclesLater( exeunitsmem , issuetime )
                request := & virtualdevice.Request{
                    Now : issuetime2,
                    Pid : pid,
                    RecoverTime:recoverTime,
                }
                request.AddtionalInfo = append(request.AddtionalInfo, inst_feature )

                request.AddtionalInfo = append(request.AddtionalInfo,  req.AddtionalInfo[1])
                rettime, smem_success = vc.Virtuall1srob.IntervalModel(  request)
            } else {
                rettime = finishtime
                issuetime2 = issuetime
            }
            //cachetime, cache_finished := vc.IntervalModelMem( vc.Virtuall1scache, request )
            if smem_success {
                issuetime = issuetime2
                finishtime_tmp := rettime
                finishtime = max( finishtime_tmp , finishtime )
                execute_status.Reset()
            } else {
                execute_status.Inst_idx = idx
                execute_status.Finishtime = finishtime
                execute_status.Issuetime = issuetime
                execute_success = false            
                continue_execute = false
            }


        default:
		    log.Panicf("Inst format %s is not supported", inst_type)
        }

    }
    if execute_success {
        finishtime = vc.Freq.NextTick(finishtime)
    } 
    return finishtime , execute_success 
}



func (vc * VirtualCU) IntervalSimulate(  wavefront_feature * profiler.WavefrontFeature , pid vm.PID, now sim.VTimeInSec, wf_id string, wf* wavefront.Wavefront ) ( sim.VTimeInSec,bool) {
//for performace, flush out useless dram data

    for _,vdram := range vc.dramVirtualDrams{
        vdram.FlushoutBuffer(now)
    }

    req := & virtualdevice.Request{
//        Topdevice : vc.cu,
        Now : now,
        Pid : pid,
        Wfid : wf_id,
        Wf : wf,
        RecoverTime : now,
    }
//    fmt.Printf("begin %.18f\n",now)
    req.AddtionalInfo = append(req.AddtionalInfo, wavefront_feature)
    //simtime,found := vc.IntervalModel( req )
    simtime, success := vc.IntervalModel( req )
//    fmt.Printf("###############################################\n")
    return simtime, success

}

func (v *VirtualCU) DebugPrint( format string , args ...interface{} ) {
//        return;

    _,filename,line, valid := runtime.Caller(1)
    if valid {
        fmt.Printf( "%s:%d  ",filename,line)
    }
    fmt.Printf( " %s " , v.Name() ) 
    fmt.Printf( format ,  args...)
    fmt.Printf( " \n " ) 
}


