package virtualcache

import (
    //"gitlab.com/akita/mem/v3/mem"
	//"gitlab.com/akita/mem/v3/vm/tlb"
//    "gitlab.com/akita/mgpusim/v3/profiler"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/akita/v3/sim"
//    "fmt"
//    "log"
  //  "reflect"
//	"gitlab.com/akita/mgpusim/v3/virtualdevice/virtualcache/internal"
//	"gitlab.com/akita/mem/v3/cache"
	"gitlab.com/akita/mgpusim/v3/virtualdevice"
  //  "runtime"
  //  "math"
)

type Builder struct {

    pipelinestagenum int
    postpipelinestagenum int
    numSets        int
	numWays        int
	numBanks        int
    numMSHREntry    int
	pageSize       uint64
	log2PageSize       uint64
    log2memorybankinterleaving int
    log2BlockSize  uint64
    latency int
    mshrfixedlatency int
    engine sim.Engine
    freq sim.Freq
    sendpipelinestagenum int
}
func MakeBuilder(  ) Builder {
    return Builder{
		numSets:        1,
		numWays:        32,
		numBanks:        1,
        numMSHREntry:    4,
        log2PageSize :  12, 
        log2BlockSize : 6,
        log2memorybankinterleaving : 7,
        latency : 1,
        mshrfixedlatency : 8,
        pipelinestagenum : 0,
        sendpipelinestagenum : 1,
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



// WithNumSets sets the number of sets in a TLB. Use 1 for fully associated
// TLBs.
func (b Builder) WithNumSets(n int) Builder {
	b.numSets = n
	return b
}
func (b Builder) WithNumMSHREntry( nummshrentry int) Builder {
	b.numMSHREntry = nummshrentry
	return b
}

func (b Builder) WithLatency( para int ) Builder {
	b.latency = para
	return b
}
func (b Builder) WithLog2BlockSize( log2blocksize uint64 ) Builder{
    b.log2BlockSize = log2blocksize
    return b
}
// WithNumWays sets the number of ways in a TLB. Set this field to the number
// of TLB entries for all the functions.
func (b Builder) WithNumWays(n int) Builder {
	b.numWays = n
	return b
}
func (b Builder) WithNumBanks(n int) Builder {
	b.numBanks = n
	return b
}
func (b Builder) WithPipelineStageNum(n int) Builder {
	b.pipelinestagenum = n
	return b
}

func (b Builder) WithPostPipelineStageNum(n int) Builder {
	b.postpipelinestagenum = n
	return b
}


func (b Builder) WithSendPipelineStageNum(n int) Builder {
	b.sendpipelinestagenum = n
	return b
}

// A WfCompletionEvent marks the completion of a wavefront

type DataReadyEvent struct {
	*sim.EventBase
    req *virtualdevice.Request
}

type EvictCompletionEvent struct {
	*sim.EventBase
    req *virtualdevice.Request
}


type ReadCompletionEvent struct {
	*sim.EventBase
    req *virtualdevice.Request
}

type WriteCompletionEvent struct {
	*sim.EventBase
    req *virtualdevice.Request
}
type ReleaseBlockLockEvent struct {
	*sim.EventBase
    req *virtualdevice.Request
}
type ReleaseMSHREvent struct {
	*sim.EventBase
    req *virtualdevice.Request
}


type ReadFromBottomCompletionEvent struct {
	*sim.EventBase
    req *virtualdevice.Request
}

type VirtualStorage interface {
    
    Evict( now sim.VTimeInSec ) 
    IntervalModel2( addr uint64, pid vm.PID , now sim.VTimeInSec) sim.VTimeInSec 
}
type AddMSHREvent struct {
	*sim.EventBase
    old_mshr_entry * virtualdevice.MSHREntry  
    new_mshr_entry * virtualdevice.MSHREntry  
}


type VirtualCache interface {

    SetRealComponent(  realcomp virtualdevice.RealComponent ) 
    AddBottomDevice( bottom virtualdevice.VirtualComponent ) 
    NewBottomReadyEvent( time sim.VTimeInSec, req *virtualdevice.Request )

    GetPipelineStageNum() int 
    IntervalModel( req * virtualdevice.Request ) (sim.VTimeInSec,bool ) 
}


func (vt *Builder)  Build(name string) VirtualCache {
    vircache := &Writearound {
        numSets : vt.numSets,
        numBanks : vt.numBanks,
        numMSHREntry : vt.numMSHREntry,
        numWays : vt.numWays,
        log2PageSize : vt.log2PageSize,
        log2BlockSize : vt.log2BlockSize,
        pageSize : 1<<vt.log2PageSize,
        latency : vt.latency,
        pipelinestagenum : vt.pipelinestagenum,
        sendpipelinestagenum : vt.sendpipelinestagenum,
        log2memorybankinterleaving : vt.log2memorybankinterleaving,
    }
    vircache.directory = virtualdevice.NewDirectory(
		vt.numSets, vt.numWays, 1<<vt.log2BlockSize,
		virtualdevice.NewLRUVictimFinder())
	vircache.TickingComponent = sim.NewTickingComponent(
		name, vt.engine, vt.freq, vircache)

	vircache.mshr = virtualdevice.NewMSHR(vt.numMSHREntry)
    return vircache
}

