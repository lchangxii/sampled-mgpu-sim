package emu
import (
	"log"
//	"reflect"
    "fmt"
	"encoding/binary"
	//"github.com/rs/xid"
	"gitlab.com/akita/akita/v3/sim"
//	"gitlab.com/akita/akita/v3/tracing"
	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm"
	//"gitlab.com/akita/mgpusim/v3/
	"gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mgpusim/v3/insts"
	"gitlab.com/akita/mgpusim/v3/kernels"
	"gitlab.com/akita/mgpusim/v3/protocol"
//	"gitlab.com/akita/mgpusim/v3/timing/wavefront"
//	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcu"
	"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"

    //"fmt"
)



type StaticComputeUnit struct {
	freq sim.Freq
	alu                ALU
    decoder            Decoder
	scratchpadPreparer ScratchpadPreparer
	storageAccessor    *storageAccessor
	LDSStorage  []byte
	GlobalMemStorage *mem.Storage
    last_pc uint64
	wfs         map[*kernels.WorkGroup][]*Wavefront
    //analysis results
    Bbvset map[profiler.BBL] uint32
    Bbv2insts map[profiler.BBL] []*insts.Inst
}
func (cu * StaticComputeUnit) Reset( )  {
    if cu == nil {
        return
    }
    cu.Bbvset = make( map[profiler.BBL] uint32)
    cu.Bbv2insts = make( map[profiler.BBL][]*insts.Inst)
}

var Static_compute_unit * StaticComputeUnit


func NewStaticComputeUnit(
	name string,
	freq sim.Freq,
	decoder Decoder,
	scratchpadPreparer ScratchpadPreparer,
	alu ALU,
	sAccessor *storageAccessor,
) *StaticComputeUnit {
	cu := new(StaticComputeUnit)
	cu.freq = freq

	cu.decoder = decoder
	cu.scratchpadPreparer = scratchpadPreparer
	cu.alu = alu
	cu.storageAccessor = sAccessor

	cu.wfs = make(map[*kernels.WorkGroup][]*Wavefront)
    log.Printf("wf init")
	cu.Bbvset = make(map[profiler.BBL]uint32)
	cu.Bbv2insts = make(map[profiler.BBL][]*insts.Inst)

	return cu
}


func BuildStaticComputeUnit(
	name string,
	freq sim.Freq,
	decoder Decoder,
	pageTable vm.PageTable,
	log2PageSize uint64,
	storage *mem.Storage,
	addrConverter mem.AddressConverter,
) *StaticComputeUnit {
    name += "-static"
	scratchpadPreparer := NewScratchpadPreparerImpl()
	sAccessor := NewStorageAccessor(
		storage, pageTable, log2PageSize, addrConverter)
	alu := NewALU(sAccessor)
	cu := NewStaticComputeUnit(name, freq, decoder,
		scratchpadPreparer, alu, sAccessor)
	return cu
}

func CreateUniqStaticComputeUnit(
name string,
freq sim.Freq,
decoder Decoder,
pageTable vm.PageTable,
log2PageSize uint64,
storage *mem.Storage,
addrConverter mem.AddressConverter,
) {
    if *sampledrunner.BranchSampledFlag {
        Static_compute_unit = BuildStaticComputeUnit(
            name,
            freq,
            decoder,
            pageTable,
            log2PageSize,
            storage,
            addrConverter,
        )

    }
}
func (cu * StaticComputeUnit) GetBBLInsts( bbl profiler.BBL) []*insts.Inst {
    res,found := cu.Bbv2insts[bbl]
    if !found {
        panic("static analysis has some issue; we can not find this BBL")
    }
    return res
}
func (cu * StaticComputeUnit) initWfRegs(wf *Wavefront) {
	co := wf.CodeObject
	pkt := wf.Packet

	wf.PC = pkt.KernelObject + co.KernelCodeEntryByteOffset
//    log.Printf("%d\n",wf.PC)
	wf.Exec = wf.InitExecMask

	SGPRPtr := 0
	if co.EnableSgprPrivateSegmentBuffer() {
		// log.Printf("EnableSgprPrivateSegmentBuffer is not supported")
		//fmt.Printf("s%d SGPRPrivateSegmentBuffer\n", SGPRPtr/4)
		SGPRPtr += 16
	}

	if co.EnableSgprDispatchPtr() {
		binary.LittleEndian.PutUint64(wf.SRegFile[SGPRPtr:SGPRPtr+8], wf.PacketAddress)
		//fmt.Printf("s%d SGPRDispatchPtr\n", SGPRPtr/4)
		SGPRPtr += 8
	}

	if co.EnableSgprQueuePtr() {
		log.Printf("EnableSgprQueuePtr is not supported")
		//fmt.Printf("s%d SGPRQueuePtr\n", SGPRPtr/4)
		SGPRPtr += 8
	}

	if co.EnableSgprKernelArgSegmentPtr() {
		binary.LittleEndian.PutUint64(wf.SRegFile[SGPRPtr:SGPRPtr+8], pkt.KernargAddress)
		//fmt.Printf("s%d SGPRKernelArgSegmentPtr\n", SGPRPtr/4)
		SGPRPtr += 8
	}

	if co.EnableSgprDispatchID() {
		log.Printf("EnableSgprDispatchID is not supported")
		//fmt.Printf("s%d SGPRDispatchID\n", SGPRPtr/4)
		SGPRPtr += 8
	}

	if co.EnableSgprFlatScratchInit() {
		log.Printf("EnableSgprFlatScratchInit is not supported")
		//fmt.Printf("s%d SGPRFlatScratchInit\n", SGPRPtr/4)
		SGPRPtr += 8
	}

	if co.EnableSgprPrivateSegementSize() {
		log.Printf("EnableSgprPrivateSegmentSize is not supported")
		//fmt.Printf("s%d SGPRPrivateSegmentSize\n", SGPRPtr/4)
		SGPRPtr += 4
	}

	if co.EnableSgprGridWorkGroupCountX() {
		binary.LittleEndian.PutUint32(wf.SRegFile[SGPRPtr:SGPRPtr+4],
			(pkt.GridSizeX+uint32(pkt.WorkgroupSizeX)-1)/uint32(pkt.WorkgroupSizeX))
		//fmt.Printf("s%d WorkGroupCountX\n", SGPRPtr/4)
		SGPRPtr += 4
	}

	if co.EnableSgprGridWorkGroupCountY() {
		binary.LittleEndian.PutUint32(wf.SRegFile[SGPRPtr:SGPRPtr+4],
			(pkt.GridSizeY+uint32(pkt.WorkgroupSizeY)-1)/uint32(pkt.WorkgroupSizeY))
		//fmt.Printf("s%d WorkGroupCountY\n", SGPRPtr/4)
		SGPRPtr += 4
	}

	if co.EnableSgprGridWorkGroupCountZ() {
		binary.LittleEndian.PutUint32(wf.SRegFile[SGPRPtr:SGPRPtr+4],
			(pkt.GridSizeZ+uint32(pkt.WorkgroupSizeZ)-1)/uint32(pkt.WorkgroupSizeZ))
		//fmt.Printf("s%d WorkGroupCountZ\n", SGPRPtr/4)
		SGPRPtr += 4
	}

	if co.EnableSgprWorkGroupIDX() {
		binary.LittleEndian.PutUint32(wf.SRegFile[SGPRPtr:SGPRPtr+4],
			uint32(wf.WG.IDX))
		//fmt.Printf("s%d WorkGroupIdX\n", SGPRPtr/4)
		SGPRPtr += 4
	}

	if co.EnableSgprWorkGroupIDY() {
		binary.LittleEndian.PutUint32(wf.SRegFile[SGPRPtr:SGPRPtr+4],
			uint32(wf.WG.IDY))
		//fmt.Printf("s%d WorkGroupIdY\n", SGPRPtr/4)
		SGPRPtr += 4
	}

	if co.EnableSgprWorkGroupIDZ() {
		binary.LittleEndian.PutUint32(wf.SRegFile[SGPRPtr:SGPRPtr+4],
			uint32(wf.WG.IDZ))
		//fmt.Printf("s%d WorkGroupIdZ\n", SGPRPtr/4)
		SGPRPtr += 4
	}

	if co.EnableSgprWorkGroupInfo() {
		log.Printf("EnableSgprPrivateSegmentSize is not supported")
		SGPRPtr += 4
	}

	if co.EnableSgprPrivateSegmentWaveByteOffset() {
		log.Printf("EnableSgprPrivateSegentWaveByteOffset is not supported")
		SGPRPtr += 4
	}

	var x, y, z int
	for i := wf.FirstWiFlatID; i < wf.FirstWiFlatID+64; i++ {
		z = i / (wf.WG.SizeX * wf.WG.SizeY)
		y = i % (wf.WG.SizeX * wf.WG.SizeY) / wf.WG.SizeX
		x = i % (wf.WG.SizeX * wf.WG.SizeY) % wf.WG.SizeX
		laneID := i - wf.FirstWiFlatID

		wf.WriteReg(insts.VReg(0), 1, laneID, insts.Uint32ToBytes(uint32(x)))

		if co.EnableVgprWorkItemID() > 0 {
			wf.WriteReg(insts.VReg(1), 1, laneID, insts.Uint32ToBytes(uint32(y)))
		}

		if co.EnableVgprWorkItemID() > 1 {
			wf.WriteReg(insts.VReg(2), 1, laneID, insts.Uint32ToBytes(uint32(z)))
		}
	}
}


func (cu *StaticComputeUnit) initLDS(wg *kernels.WorkGroup, req *protocol.MapWGReq) []byte {
	ldsSize := req.WorkGroup.Packet.GroupSegmentSize
	lds := make([]byte, ldsSize)
	return lds
}

func (cu *StaticComputeUnit) initWfs(
	wg *kernels.WorkGroup,
	req *protocol.MapWGReq,
) error {
	lds := cu.initLDS(wg, req)

	for _, wf := range wg.Wavefronts {
		managedWf := NewWavefront(wf)
		managedWf.LDS = lds
		managedWf.SetPID( req.PID)
//        _,found := cu.wfs[wg]
		cu.wfs[wg] = append(cu.wfs[wg], managedWf)
	}

	for _, managedWf := range cu.wfs[wg] {
		cu.initWfRegs(managedWf)
	}

	return nil
}



func (cu * StaticComputeUnit ) PrintData() {
    log.Printf("static analysis for bbv:")
    for bbv,_ := range cu.Bbvset{
        fmt.Printf("pc: %d ; insn: %d\n",bbv.PC,bbv.InsNum) 
    }
}

func (cu * StaticComputeUnit ) AnalysisKernel(req *protocol.MapWGReq) {

    wg := req.WorkGroup 
	cu.initWfs(wg, req)
    wfs := cu.wfs[wg]
    wf:=wfs[0]

	cu.alu.SetLDS(wf.LDS)
    cu.analysisWfUntilEnd(wf)
}


func (cu * StaticComputeUnit) analysisWfUntilEnd(wf *Wavefront)  {
    continue_execute := true
    ///analysis
    pc2endingpc :=make(map[uint64]uint64)
    pc2idx :=make(map[uint64]int)
    startpc := wf.PC
    pcs_without_endingpc := make([]uint64,0)
    bbstartpc := wf.PC - startpc
    anotherpcs := make([]uint64,0)
    var serialinsts []*insts.Inst
    inst_idx := 0
	for ;continue_execute;{
		instBuf := cu.storageAccessor.Read(wf.PID(), wf.PC, 8)
		inst, _ := cu.decoder.Decode(instBuf)

        pc2idx[wf.PC-startpc] = inst_idx
        serialinsts = append(serialinsts,inst)
        inst_idx ++
		wf.inst = inst
        is_branch_or_barrier := false
		if inst.FormatType == insts.SOPP {
            switch inst.Opcode {
                case 1: //complete
                    continue_execute = false
                case 2,4,5,6,7,8,9,10:
                    is_branch_or_barrier = true
                default:

            }
        }
        if is_branch_or_barrier || (!continue_execute) {

            size_of_bbv := len(pcs_without_endingpc)

            new_bbv := profiler.BBL{
                PC : bbstartpc,
                InsNum : uint64(size_of_bbv)+1,
            }
            cu.Bbvset[new_bbv] = 1
            for idx ,pc := range(pcs_without_endingpc) {
                pc2endingpc[pc] = uint64(size_of_bbv + 1 - idx)
            }
            pc2endingpc[wf.PC] = 1
            if continue_execute {
                anotherPC := uint64(wf.PC) + uint64(inst.ByteSize)

		        wf.PC += uint64(inst.ByteSize)

	            cu.scratchpadPreparer.Prepare(wf, wf)
	            sp := wf.Scratchpad().AsSOPP()
                imm := asInt16(uint16(sp.IMM & 0xffff))
		        anotherPC = uint64( int64(anotherPC) + int64(imm)*4)
                anotherpcs = append(anotherpcs,anotherPC)
                pcs_without_endingpc = nil
            }
        } else {
            pcs_without_endingpc = append(pcs_without_endingpc,wf.PC)

		    wf.PC += uint64(inst.ByteSize)
        }
        if is_branch_or_barrier {
            bbstartpc = wf.PC - startpc
        }
	}
//    for key,ins := range pc2endingpc{
//        log.Printf("pc%d num %d\n",key,ins)
//    }
    for _,pc:= range anotherpcs{
            another_bbv := profiler.BBL{
                PC:pc-startpc,
                InsNum:pc2endingpc[pc],
            }
//            log.Printf("%d %d\n",another_bbv.PC,another_bbv.InsNum)
            cu.Bbvset[another_bbv] = 1

    }
    //init insts for all bbl
    for bbl,_ :=range cu.Bbvset {

        _,found := cu.Bbv2insts[bbl]
        if !found {
            idx :=  pc2idx[bbl.PC]
            cu.Bbv2insts[bbl] = serialinsts[ idx: idx+int(bbl.InsNum)]
        }
    }
    wf.PC = startpc
}
