package emu

import (
	"log"
//	"reflect"
    "encoding/json"
	"encoding/binary"
    "os"
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
	//"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"

    //"fmt"
)
type BBVComputeUnit struct {
	freq sim.Freq
    decoder            Decoder
	scratchpadPreparer ScratchpadPreparer
	alu                ALU
	storageAccessor    *storageAccessor
	wfs         map[*kernels.WorkGroup][]*Wavefront
	LDSStorage  []byte
	GlobalMemStorage *mem.Storage
    last_pc uint64
    //for debugging
    bbvset map[profiler.BBL] uint32
    allinsnums []uint64 
    allonlinebbvs []*profiler.OnlineBbv 
    AllinsnumsEachKernel [][]uint64 `json:"insnums"`
    AllonlinebbvsEachKernel [][]*profiler.OnlineBbv `json:"bbvs"`
    Analysistimes [] float64 `json:"analysistimes"`
    Analysistime float64
    Wfnum uint64
    Wfnums []uint64 `json:"wfnums"`
}
func (cu *BBVComputeUnit) GetAllonlineBBVs() []*profiler.OnlineBbv {
    return cu.allonlinebbvs
}
func (cu *BBVComputeUnit) FFlush() {
    cu.AllonlinebbvsEachKernel = append( cu.AllonlinebbvsEachKernel, cu.allonlinebbvs )
    cu.AllinsnumsEachKernel = append( cu.AllinsnumsEachKernel, cu.allinsnums )
    cu.Analysistimes = append( cu.Analysistimes, cu.Analysistime )
    cu.Wfnums = append( cu.Wfnums, cu.Wfnum )
    cu.allonlinebbvs= nil
    cu.allinsnums = nil
}

func (cu *BBVComputeUnit) runWfUntilBarrier(wf *Wavefront, onlinebbv *profiler.OnlineBbv) uint64 {
    continue_execute := true
    insnum := uint64(0)
	for ;continue_execute;{
		instBuf := cu.storageAccessor.Read(wf.PID(), wf.PC, 8)

		inst, _ := cu.decoder.Decode(instBuf)
        wf.SetInst( inst )
        inst.PC = wf.PC
        onlinebbv.CountInst(inst)
		if inst.FormatType == insts.SOPP {
               switch inst.Opcode {
                case 10:
			        wf.AtBarrier = true
			        continue_execute = false
                case 1:
                    wf.Completed = true
                    continue_execute = false
                default:
            }
        }
		wf.PC += uint64(inst.ByteSize)
        if continue_execute {
		    cu.executeInst(wf)
        }
        insnum++
	}
    return insnum
}
func (cu *BBVComputeUnit) executeInst(wf *Wavefront) {
	cu.scratchpadPreparer.Prepare(wf, wf)
	cu.alu.Run(wf)
	cu.scratchpadPreparer.Commit(wf, wf)
}
func (cu *BBVComputeUnit) initWfs(
	wg *kernels.WorkGroup,
	req *protocol.MapWGReq,
) error {
	lds := cu.initLDS(wg, req)

	for _, wf := range wg.Wavefronts {
		managedWf := NewWavefront(wf)
		managedWf.LDS = lds
		managedWf.SetPID( req.PID)
		cu.wfs[wg] = append(cu.wfs[wg], managedWf)
	}

	for _, managedWf := range cu.wfs[wg] {
		cu.initWfRegs(managedWf)
	}

	return nil
}

func (cu *BBVComputeUnit) initLDS(wg *kernels.WorkGroup, req *protocol.MapWGReq) []byte {
	ldsSize := req.WorkGroup.Packet.GroupSegmentSize
	lds := make([]byte, ldsSize)
	return lds
}
func (cu *BBVComputeUnit) initWfRegs(wf *Wavefront) {
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

func (cu *BBVComputeUnit) isAllWfCompleted(wg *kernels.WorkGroup) bool {
	for _, wf := range cu.wfs[wg] {
		if !wf.Completed {
			return false
		}
	}
	return true
}


func (cu *BBVComputeUnit) resolveBarrier(wg *kernels.WorkGroup) {
	if cu.isAllWfCompleted(wg) {
		return
	}

	for _, wf := range cu.wfs[wg] {
		if !wf.AtBarrier {
			log.Panic("not all wavefronts at barrier")
		}
		wf.AtBarrier = false
	}
}




func (cu *BBVComputeUnit) RunWG(
	req *protocol.MapWGReq,
) {
    wg := req.WorkGroup 
	cu.initWfs(wg, req)
    wfs := cu.wfs[wg]
    onlinebbvs := make([]*profiler.OnlineBbv,len(wfs))
    insnums := make([]uint64,len(wfs))
    for i,_ := range(onlinebbvs) {
        onlinebbvs[i] = profiler.InitOnlineBbv()
        insnums[i] = 0
    }
	for !cu.isAllWfCompleted(wg) {
		for i, wf := range wfs  {
			cu.alu.SetLDS(wf.LDS)
            insnum := cu.runWfUntilBarrier(wf,onlinebbvs[i])
            insnums[i] += insnum
		}
		cu.resolveBarrier(wg)
	}
    cu.allonlinebbvs = append(cu.allonlinebbvs,onlinebbvs...)
    cu.allinsnums = append(cu.allinsnums,insnums...)
}
// NewComputeUnit creates a new ComputeUnit with the given name
func NewBBVComputeUnit(
	name string,
	freq sim.Freq,
	decoder Decoder,
	scratchpadPreparer ScratchpadPreparer,
	alu ALU,
	sAccessor *storageAccessor,
) *BBVComputeUnit {
	cu := new(BBVComputeUnit)
	cu.freq = freq

	cu.decoder = decoder
	cu.scratchpadPreparer = scratchpadPreparer
	cu.alu = alu
	cu.storageAccessor = sAccessor

	cu.wfs = make(map[*kernels.WorkGroup][]*Wavefront)

	cu.bbvset = make(map[profiler.BBL]uint32)

	return cu
}



func BuildBBVComputeUnit(
	name string,
	freq sim.Freq,
	decoder Decoder,
	pageTable vm.PageTable,
	log2PageSize uint64,
	storage *mem.Storage,
	addrConverter mem.AddressConverter,
) *BBVComputeUnit {
    name += "-bbv-sampled"
	scratchpadPreparer := NewScratchpadPreparerImpl()
	sAccessor := NewStorageAccessor(
		storage, pageTable, log2PageSize, addrConverter)
	alu := NewALU(sAccessor)
	cu := NewBBVComputeUnit(name, freq, decoder,
		scratchpadPreparer, alu, sAccessor)
	return cu
}
var Bbvcomputeunit * BBVComputeUnit
func CreateUniqBBVComputeUnit(
name string,
freq sim.Freq,
decoder Decoder,
pageTable vm.PageTable,
log2PageSize uint64,
storage *mem.Storage,
addrConverter mem.AddressConverter,
) {
    Bbvcomputeunit = BuildBBVComputeUnit(
        name,
        freq,
        decoder,
        pageTable,
        log2PageSize,
        storage,
        addrConverter,
    )
}
func  ReportBBVFeature(  ) {
    if Bbvcomputeunit!=nil && len(Bbvcomputeunit.AllonlinebbvsEachKernel)>0{
        jsonStr,err := json.MarshalIndent( Bbvcomputeunit,""," ")
        if err != nil {
            log.Printf("error happened")
            panic(err)
        }
        file, _ := os.Create("bbv_feature.json")
        defer file.Close()
        file.Write(jsonStr)
    }
}


