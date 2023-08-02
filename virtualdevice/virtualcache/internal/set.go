// Package internal provides the definition required for defining TLB.
package internal

import (
	"fmt"
	"math"

	"gitlab.com/akita/akita/v3/sim"
	"github.com/google/btree"
	"gitlab.com/akita/mem/v3/vm"
	"gitlab.com/akita/mgpusim/v3/virtualdevice"
)

// A Set holds a certain number of pages.
type Set interface {
    Lookup(pid vm.PID, vAddr uint64, now sim.VTimeInSec) (	access_time sim.VTimeInSec,found bool)

    Update(  page virtualdevice.Page, accesstime sim.VTimeInSec) 
	Evict(now sim.VTimeInSec) 
	Visit(wayID int)
}

// NewSet creates a new TLB set.
func NewSet(numWays int) Set {
	s := &setImpl{}
//	s.blocks = make([]*block, numWays)
    s.numWays = numWays
//	s.visitTree = btree.New(2)
//	s.vAddrWayIDMap = make(map[string]int)
//	for i := range s.blocks {
//		b := &block{}
//		s.blocks[i] = b
//		b.wayID = i
//		s.Visit(i)
//	}
	return s
}

type block struct {
	page      virtualdevice.Page
//	wayID     int
//	lastVisit uint64
    lastVisitTime sim.VTimeInSec
}

//func (b *block) Less(anotherBlock btree.Item) bool {
//	return b.lastVisit < anotherBlock.(*block).lastVisit
//}

type setImpl struct {
	blocks        []*block
	vAddrWayIDMap map[string]int
	visitTree     *btree.BTree
	visitCount    uint64
    numWays       int
}

func (s *setImpl) keyString(pid vm.PID, vAddr uint64) string {
	return fmt.Sprintf("%d%016x", pid, vAddr)
}

func (s *setImpl) findBeginIdx(now sim.VTimeInSec ) int {
    index := len(s.blocks)
    i,j := 0, index-1
    for i < j {
        h := i + (j-i) / 2
        comp := s.blocks[h].lastVisitTime
        if comp <= now {
            j = h - 1       
        } else if comp > now {
            i = h + 1
        } 
    }
//    comp := s.blocks[i].lastVisitTime
//    if i < n {
//        index = i
//    }
    index = i
//    for idx , blk := range s.blocks {
//        if blk.lastVisitTime > now {
//            continue
//        } else {
//            index = idx    
//        }
//    }
    return index
}

func (s *setImpl) Lookup(pid vm.PID, vAddr uint64, now sim.VTimeInSec) (
	access_time sim.VTimeInSec,
    found bool,
) { // there are two cases. One is that the set will be loaded, return expected time
    // another is the set was loaded, return hit or miss(because of evict)

    insert_idx := s.findBeginIdx(now)

    block_size := len(s.blocks)

    time := sim.VTimeInSec(math.MaxFloat64)

    if( block_size == 0 ) {
        return time , false
    }
    begin_index := len(s.blocks) - 1
    found = false

    //look for the past at first
    index := insert_idx
    dis_idx := 0
    for ; index < block_size && dis_idx < s.numWays ; dis_idx++ {
        block_tmp := s.blocks[index]
        page_tmp := &block_tmp.page
        if( page_tmp.PID == pid && page_tmp.VAddr == vAddr ) {
            time = block_tmp.lastVisitTime
            found = true        
            break;
        }
        index++
    }
    if found {
	    return time,found
    }
    //look for the future
    index = begin_index - 1
    for ; index >= 0 ; index-- {
        block_tmp := s.blocks[index]
        page_tmp := &block_tmp.page
        if( page_tmp.PID == pid && page_tmp.VAddr == vAddr ) {
            time = block_tmp.lastVisitTime
            found = true        
            break;

        }
    }
	return time,found
}

func (s *setImpl) Update(  page virtualdevice.Page, accesstime sim.VTimeInSec) { 
    insert_idx := s.findBeginIdx(accesstime)

    s.blocks = append(s.blocks,&block{})
    copy( s.blocks[insert_idx+1:],s.blocks[insert_idx:] )
    s.blocks[insert_idx] = &block{
        page : page,
        lastVisitTime : accesstime,
    }
}

func (s *setImpl) Evict(now sim.VTimeInSec)  {
    insert_idx := s.findBeginIdx(now)
    insert_idx += s.numWays
//    fmt.Print("%d %d\n",insert_idx,len(s.blocks))
    if insert_idx < len(s.blocks) {
        s.blocks = s.blocks[:insert_idx]
    }
//	if s.hasNothingToEvict() {
//		return 0, false
//	}

//	wayID = s.visitTree.DeleteMin().(*block).wayID
//	return wayID, true
}

func (s *setImpl) Visit(wayID int) {
//	block := s.blocks[wayID]
//	s.visitTree.Delete(block)
//
//	s.visitCount++
//	block.lastVisit = s.visitCount
//	s.visitTree.ReplaceOrInsert(block)
}

func (s *setImpl) hasNothingToEvict() bool {
	return s.visitTree.Len() == 0
}
