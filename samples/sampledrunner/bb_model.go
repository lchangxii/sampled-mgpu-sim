package sampledrunner
import (
    "log"
	"gitlab.com/akita/mgpusim/v3/insts"
	"gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/akita/v3/sim"
)
func max( a,b sim.VTimeInSec ) sim.VTimeInSec {
    if( a > b ) {
        return a
    } else {
        return b
    }
}
type BBModel struct{
   Freq sim.Freq 
   initsmemreadtime sim.VTimeInSec
   initvmemreadtime sim.VTimeInSec
   initvmemwritetime sim.VTimeInSec
}

var Global_bbmode * BBModel
func  InitUniqBBModel( freq sim.Freq) {
    Global_bbmode = &BBModel{
        initsmemreadtime : 128*(1e-9),
        initvmemreadtime : 32 * (1e-9), 
        initvmemwritetime : (4) * (1e-9),
        Freq : freq,
    }
}

func (bb * BBModel)handleALU( issuetime sim.VTimeInSec, finishtime  sim.VTimeInSec , interval int ) ( sim.VTimeInSec, sim.VTimeInSec) {
    issuetime = bb.Freq.NCyclesLater( interval , issuetime)
    finishtime = max(issuetime,finishtime)
    return issuetime,finishtime
}

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


func (bb * BBModel) IntervalModel( bbtrace[]* insts.Inst ) sim.VTimeInSec  {
    issuetime := sim.VTimeInSec(0)
    finishtime := sim.VTimeInSec(0)
    //we specially process ending of wavefronts as there is not any subseqyent instructions finishtime - issuetime_begin
    //for others we use issuetime_end - issuetime_begin
    endpgm := false
    for _, inst := range bbtrace {
        inst_format := inst.FormatType
//        log.Printf("format %d opcode %d time: %.2f \n",inst_format,inst.Opcode,issuetime*1e9)
        switch inst_format {
        case insts.SOP1, insts.SOP2,  insts.SOPC: // ExeUnitScalar
            issuetime,finishtime = bb.handleALU( issuetime, finishtime, exeunitscalar )
        case insts.VOP1, insts.VOP2,insts.VOP3a, insts.VOP3b, insts.VOPC: //ExeUnitVALU
            issuetime,finishtime = bb.handleALU( issuetime, finishtime, exeunitvalu )
        case insts.SOPK: //ExeUnitScalar 
            issuetime,finishtime = bb.handleALU( issuetime, finishtime, exeunitscalar )
        case insts.DS: //ExeUnitLDS
            issuetime,finishtime= bb.handleALU( issuetime, finishtime, exeunitlds )
        case insts.SOPP: // SOPP=4 // ExeUnitBranch  0,1,10,11,12,13,14,15,18.19.20,21,22,27,28,29 ExeUnitSpecial
        {
            if  inst.Opcode == 12 {
                finishtime = bb.Freq.NextTick( finishtime )
                issuetime = finishtime
            } else {
                if inst.Opcode == 1 {
                    finishtime = bb.Freq.NextTick( finishtime )
                    issuetime = finishtime
                    endpgm = true
                } else {
                    issuetime,finishtime = bb.handleALU( issuetime, finishtime, exeunitbranch )
                }
            }
        }
    	case insts.FLAT: //ExeUnitVMem 
            finishtime2 := issuetime

            issuetime  = bb.Freq.NCyclesLater( exeunitvmem , issuetime )

            time, found := profiler.Global_inst_feature.Predict(inst)
            log.Printf("time %.2f found %t opcode %d\n",time*1e9,found,inst.Opcode)
            if found {
                finishtime2 += time
            } else {
            switch  inst.Opcode {
                    case 16,18,20,21,23://read
                        finishtime2 += bb.initvmemreadtime
                    case 28,29,30,31://write
                        finishtime2 +=  bb.initvmemwritetime
            }
            }
            finishtime = max(finishtime2,finishtime)

        case insts.SMEM: //ExeUnitScalar number five

            finishtime2 := issuetime
            issuetime  = bb.Freq.NCyclesLater( exeunitsmem , issuetime )

            time, found := profiler.Global_inst_feature.Predict(inst)
            if found {
                finishtime2 += time
            } else {
                finishtime2 += bb.initsmemreadtime
            }
            finishtime = max(finishtime, finishtime2 )

        default:
		    log.Panicf("Inst format %s is not supported", inst_format)
        }
    }
    var ret sim.VTimeInSec
    if endpgm {

        ret = finishtime
    } else {
        ret = issuetime 
    }
    log.Printf("interval %.2f",ret*1e9)
    return ret
}


