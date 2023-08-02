package profiler
import (
    "encoding/json"
    "os"
    "log"
    "fmt"
	"gitlab.com/akita/mem/v3/vm"

	"gitlab.com/akita/akita/v3/sim"
	"gitlab.com/akita/mgpusim/v3/insts"
	"gitlab.com/akita/mgpusim/v3/utils"
    "flag"
//	"gitlab.com/akita/mgpusim/v3/samples/sampledrunner"
)
func laneMasked(Exec uint64, laneID uint) bool {
	return Exec&(1<<laneID) > 0
}

type InstEmuState interface {
	PID() vm.PID
	Inst() *insts.Inst
	Scratchpad() utils.Scratchpad
    GetPC() uint64
}

type InstFeature struct{
    ADDR []uint64 `json:"addrs"`
    PC uint64 `json:"pc"`
    Opcode insts.Opcode `json:"opcode"`
}
type WavefrontFeature struct{
    Inst_types [] insts.FormatType `json:"insttype"`
    Inst_features [] InstFeature `json:"instfeature"`
}
type WorkGroupFeature struct{
    Wffeature [] *WavefrontFeature `json:"wffeature"`
    IDX int `json:"idx"`
    IDY int `json:"idy"`
    IDZ int `json:"idz"`
}

func (wgf *WorkGroupFeature) PrintSize() { //print out how many wavefront this wavegroup has
    fmt.Printf( "wavefront array size : %d\n", len( wgf.Wffeature ) )

}
func (wgf *WorkGroupFeature) GetWavefrontFeature( index int ) *WavefrontFeature {
//    wgf.PrintSize()
//    fmt.Printf("idx : %d\n", index)
    return wgf.Wffeature[index]
}

type WorkGroupFeatureTensor struct {
    XDIM int  `json:"xdim"` 
    YDIM int  `json:"ydim"`
    ZDIM int  `json:"zdim"`
    Workgroupfeatures [] *WorkGroupFeature `json:"wavegrouptensor"`
}

var  WorkGroup_tensor WorkGroupFeatureTensor

type WfDataFeature struct {
    Issuetime sim.VTimeInSec `json:"issuetime"`
    Finishtime sim.VTimeInSec `json:"finishtime"`
}

type CUDataFeature struct {
    Wfdatafeatures [] WfDataFeature `json:"wfdatafeatures"`
}

func (cudatafeature * CUDataFeature) Size() int {
    return len(cudatafeature.Wfdatafeatures)
}

type DataFeature struct {
    CUNum int
    CUdatafeature [] *CUDataFeature `json:"cudatafeature"`
}

var  Datafeature DataFeature

func ( df *DataFeature ) SetCUNum( num int )  {
//    log.Printf( "set %d", num )
    df.CUNum = num
    df.CUdatafeature = make([]*CUDataFeature, num)
    for i := 0 ; i < num ; i++ {
        df.CUdatafeature[i] = & CUDataFeature{
        }
    }

}

func ( df *DataFeature ) AddData( cuid int, issuetime sim.VTimeInSec,finishtime sim.VTimeInSec  )  {
    wfdatafeature := WfDataFeature {
        issuetime,
        finishtime,
    } 

    //log.Printf( "cuid %d", cuid )

//    log.Printf( "%d %d\n",cuid, df.CUdatafeature[cuid].Size() )
    df.CUdatafeature[cuid].Wfdatafeatures = append(df.CUdatafeature[cuid].Wfdatafeatures,wfdatafeature )

//    log.Printf( "%d %d\n",cuid, df.CUdatafeature[cuid].Size() )
//    for i := 0 ; i < df.CUNum ; i++ {
        //log.Printf( "%d %d\n",i, df.CUdatafeature[i].Size() )
//    }

}

func (wgft *WorkGroupFeatureTensor) PrintSize() {
    fmt.Printf("tensor size : %d %d %d\n", wgft.XDIM,wgft.YDIM,wgft.ZDIM)
    fmt.Printf("data array : %d\n", len(wgft.Workgroupfeatures))
    for idx,elem := range(wgft.Workgroupfeatures) {
        fmt.Printf("%x %x %x\n", idx,elem, wgft.Workgroupfeatures[idx] )
    }
}
func (wgft *WorkGroupFeatureTensor) GetSize() (int,int,int) {
    return wgft.XDIM,wgft.YDIM,wgft.ZDIM
}
func (wgft *WorkGroupFeatureTensor ) Setxyz( x,y,z int )  {
    wgft.XDIM = x
    wgft.YDIM = y
    wgft.ZDIM = z
    wgft.Workgroupfeatures = make([]*WorkGroupFeature, x*y*z)
}

func (wgft *WorkGroupFeatureTensor) GetWorkGroupFeature( idx,idy,idz int ) *WorkGroupFeature {
    index := idx + idy * wgft.XDIM + idz * wgft.XDIM * wgft.YDIM
    ret :=  wgft.Workgroupfeatures[index] 
//    fmt.Printf("idx : %d ; addr : %x\n",index,ret)
    return ret
}
func (wgt *WorkGroupFeatureTensor) AddWorkGroupFeature( wgf* WorkGroupFeature, IDX,IDY,IDZ int ) {
    global_idx := IDX + IDY * wgt.XDIM + IDZ * wgt.YDIM * wgt.XDIM

    wgt.Workgroupfeatures[global_idx] = wgf;
}

func (wgf * WorkGroupFeature) AddWavefrontFeature( wff * WavefrontFeature ) {
    wgf.Wffeature = append(wgf.Wffeature,wff)
}


func  cacheLineID(addr uint64) uint64 {
    const log2CacheLineSize = 6
	return addr >> log2CacheLineSize << log2CacheLineSize
}

func isInSameCacheLine(addr1, addr2 uint64) bool {
	return cacheLineID(addr1) == cacheLineID(addr2)
}

func findOrCreateAddr( addrs []uint64,  addr uint64 ) (addr_reqed uint64,reqed bool ) {
    for index := len(addrs)-1; index >= 0; index-- {
        addr_tmp := addrs[index]
		if isInSameCacheLine(addr, addr_tmp) {

		    return 0, false
		}
	}
    return cacheLineID(addr) , true

} 

func  instRegCount(inst *insts.Inst) int {
	switch inst.Opcode {
	case 16, 17, 18, 19, 20:
		return 1
	case 24, 25, 26, 27, 28:
		return 1
	case 21, 29:
		return 2
	case 22, 30:
		return 3
	case 23, 31:
		return 4
	default:
		panic("not supported opcode")
	}
}



func (wff * WavefrontFeature) addFlatInstFeature( state InstEmuState ) {
	inst := state.Inst()
	sp := state.Scratchpad().AsFlat()
    ADDR_TMP := make([]uint64,0, 64)
	regCount := instRegCount( inst )
    for i := uint(0) ; i < 64 ; i++ {
        if !laneMasked(sp.EXEC, i) {
			continue
		}
		addr := sp.ADDR[i]
		for j := 0; j < regCount; j++ {
            addr_reqed,reqed := findOrCreateAddr( ADDR_TMP, addr+uint64(4*j))
            if(reqed) {
                ADDR_TMP=append( ADDR_TMP,addr_reqed )
            }
		}
    }
    //fmt.Printf("%d %d\n",cap(ADDR_TMP),len(ADDR_TMP))
    ADDR_TMP = ADDR_TMP[:len(ADDR_TMP)]
    inst_feature := InstFeature{
        ADDR: ADDR_TMP,
        Opcode : inst.Opcode,
        PC : state.GetPC(),
    }
    wff.Inst_features = append( wff.Inst_features, inst_feature )
}
func (wff * WavefrontFeature) addSMEMInstFeature( state InstEmuState) {
	inst := state.Inst()
	sp := state.Scratchpad().AsSMEM()
    opcode := inst.Opcode 
    inst_feature := InstFeature{
        Opcode : opcode,
        PC : state.GetPC(),
    } 
    inst_feature.ADDR = append( inst_feature.ADDR,  sp.Base + sp.Offset)
    wff.Inst_features = append( wff.Inst_features, inst_feature )
}
func (wff * WavefrontFeature) addDefaultInstFeature( state InstEmuState) {
	inst := state.Inst()
    opcode := inst.Opcode 
    inst_feature := InstFeature{
        Opcode : opcode,
        PC : state.GetPC(),
    } 
    wff.Inst_features = append( wff.Inst_features, inst_feature )
}

func ( wff * WavefrontFeature) AddInst( state InstEmuState ) {
	inst := state.Inst()
    //fmt.Printf("%d ",inst.FormatType)
    wff.Inst_types = append(wff.Inst_types, inst.FormatType)

	switch inst.FormatType {
	case insts.SOP1:
        wff.addDefaultInstFeature(state)

	case insts.SOP2:
        wff.addDefaultInstFeature(state)
	case insts.SOPC:
        wff.addDefaultInstFeature(state)
	case insts.VOP1:
        wff.addDefaultInstFeature(state)
	case insts.VOP2:
        wff.addDefaultInstFeature(state)
	case insts.VOP3a:
        wff.addDefaultInstFeature(state)
	case insts.VOP3b:
        wff.addDefaultInstFeature(state)
	case insts.VOPC:
        wff.addDefaultInstFeature(state)
	case insts.SOPP:
        wff.addDefaultInstFeature(state)
	case insts.SOPK:
        wff.addDefaultInstFeature(state)
	case insts.DS:
        wff.addDefaultInstFeature(state)
	case insts.FLAT:
        wff.addFlatInstFeature( state )
    case insts.SMEM:
        wff.addSMEMInstFeature( state )

	default:
		log.Panicf("Inst format %s is not supported", inst.Format.FormatName)
	}
}
var ProfileApplication = flag.Bool("generate-profiler", false,
	"Collect Important data for sampled simulation to use.")
var CollectDataApplication = flag.Bool("collect-data", false,
	"Collect Important data to prove wavefront level sampling.")
var LoadWGFeatureFlag = flag.String("load-profiler", "",
	"The path of traces of wg feature.")

func ReportWGFeatureVec() {
    if *ProfileApplication {
    //    jsonStr,_ := json.Marshal(WgFeatureVector)
        jsonStr,_ := json.MarshalIndent(WorkGroup_tensor,""," ")
        file, _ := os.Create("memtrace.json")
        defer file.Close()
        file.Write(jsonStr)
    }

    if *CollectDataApplication {
        jsonStr,_ := json.MarshalIndent( Datafeature,""," ")
        file, _ := os.Create("datafeature.json")
        defer file.Close()
        file.Write(jsonStr)

    }
    ReportIPC()
}

var SampledSimulation = false
func LoadWGFeatureVec() {
    path := *LoadWGFeatureFlag
    if path != "" {
        //fmt.Printf("Load %s\n",path)
        SampledSimulation = true
        readData,_ := os.ReadFile( path )
//        WorkGroup_tensor.PrintSize()
        json.Unmarshal( []byte(readData),&WorkGroup_tensor )
//        WorkGroup_tensor.PrintSize()
    }
}

