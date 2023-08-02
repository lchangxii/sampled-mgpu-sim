package virtualdevice 

import (
	"log"
    "math"
	"gitlab.com/akita/akita/v3/sim"
	
//	"gitlab.com/akita/mgpusim/v3/virtualdevice"
//	"gitlab.com/akita/mem/v3/mem"
	"gitlab.com/akita/mem/v3/vm"
)
// A Block of a cache is the information that is associated with a cache line
// MSHREntry is an entry in MSHR
type MSHREntry struct {
	PID       vm.PID
	Address   uint64
	Requests  []interface{}
	Block     *Block
    Expectedtime sim.VTimeInSec
    Issuetime sim.VTimeInSec
//	ReadReq   *mem.ReadReq
//	DataReady *mem.DataReadyRsp
//	Data      []byte
}

// NewMSHREntry returns a new MSHR entry object
func NewMSHREntry() *MSHREntry {
	e := new(MSHREntry)
	e.Requests = make([]interface{}, 0)
	return e
}
func (m * MSHREntry) GetTime() sim.VTimeInSec {
    return m.Expectedtime
}


// MSHR is an interface that controls MSHR entries
type MSHR interface {
	Query(pid vm.PID, addr uint64) *MSHREntry
	Add(pid vm.PID, addr uint64, issuetime sim.VTimeInSec, expectedtime sim.VTimeInSec) *MSHREntry
	Remove(pid vm.PID, addr uint64) *MSHREntry
	AllEntries() []*MSHREntry
	IsFull() bool
	Reset()
    AvailTime() sim.VTimeInSec

    AddWhenFull(pid vm.PID, addr uint64, issuetime sim.VTimeInSec, expectedtime sim.VTimeInSec) * MSHREntry 
}

// NewMSHR returns a new MSHR object
func NewMSHR(capacity int) MSHR {
	m := new(mshrImpl)
	m.capacity = capacity
    m.Reset()
//    m.mshr_unlimited_history_queue = virtualdevice.NewFixedNumQueue(0)
	return m
}

type mshrImpl struct {
	*sim.ComponentBase
    
	capacity int
	entries  []*MSHREntry
    availabletime sim.VTimeInSec
//    mshr_unlimited_history_queue virtualdevice.FixedNumQueue
}
func (m *mshrImpl) AvailTime() sim.VTimeInSec {
    return m.availabletime
}
func (m *mshrImpl) Add(pid vm.PID, addr uint64, issuetime sim.VTimeInSec, expectedtime sim.VTimeInSec) *MSHREntry {

	for _, e := range m.entries {
		if e.PID == pid && e.Address == addr {
			panic("entry already in mshr")
            return e
		}
	}


	entry := NewMSHREntry()
	entry.PID = pid
	entry.Address = addr
    entry.Expectedtime = expectedtime

    m.availabletime = sim.VTimeInSec(math.Min(float64(m.availabletime),float64(expectedtime)))

    entry.Issuetime = issuetime
	if len(m.entries) >= m.capacity {
		log.Panic("MSHR is full")
    }
//        entry.Queuetime = issuetime
//        m.mshr_unlimited_history_queue.Push(entry)
//        ret = nil
//	} else {

	m.entries = append(m.entries, entry)
//    }

	return entry
}


func (m *mshrImpl) AddWhenFull(pid vm.PID, addr uint64, issuetime sim.VTimeInSec, expectedtime sim.VTimeInSec) * MSHREntry {
	entry := NewMSHREntry()
	entry.PID = pid
	entry.Address = addr
    entry.Expectedtime = expectedtime
    entry.Issuetime = issuetime

    return entry
}


func (m *mshrImpl) Query(pid vm.PID, addr uint64) *MSHREntry {
	for _, e := range m.entries {
		if e.PID == pid && e.Address == addr {
			return e
		}
	}
	return nil
}

func (m *mshrImpl) Remove(pid vm.PID, addr uint64) *MSHREntry {
    var ret * MSHREntry 
    ret = nil
	for i, e := range m.entries {
		if e.PID == pid && e.Address == addr {
			m.entries = append(m.entries[:i], m.entries[i+1:]...)
        //            return e
            ret = e
            break
		}
	}
    if ret == nil {

   // return nil
	    panic("trying to remove an non-exist entry")
    } else {
        if ret.Expectedtime == m.availabletime {
            m.availabletime = sim.VTimeInSec(math.MaxFloat64)
            for _, e := range m.entries {
                m.availabletime = sim.VTimeInSec( math.Min( float64(e.Expectedtime), float64(m.availabletime) ) )
            }
        }
        return ret
    }
}

// AllEntries returns all the MSHREntries that are currently in the MSHR
func (m *mshrImpl) AllEntries() []*MSHREntry {
	return m.entries
}

// IsFull returns true if no more MSHR entries can be added
func (m *mshrImpl) IsFull() bool {
	if len(m.entries) >= m.capacity {
		return true
	}
	return false
}

func (m *mshrImpl) Reset() {
	m.entries = nil
    m.availabletime = sim.VTimeInSec( math.MaxFloat64)
}
