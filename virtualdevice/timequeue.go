package virtualdevice
import (
	//"container/heap"
	"container/list"
//	"sync"
    
	"gitlab.com/akita/akita/v3/sim"
)


type Elem interface {
    GetTime() sim.VTimeInSec
}





type FixedNumQueue interface {
	Push( time Elem )
	Pop() Elem
	Peek() Elem
    Len() int
}
type FixedNumQueueImpl struct {
    l *list.List
    queue_cap int
}

func (q *FixedNumQueueImpl) Len() int {
    return q.l.Len()
}
func NewFixedNumQueue(  capa int ) *FixedNumQueueImpl { //if capa==0, it is a unlimited queue
    q := &FixedNumQueueImpl{
        queue_cap : capa,
        l : list.New(),
    }
    return q
}
func (q *FixedNumQueueImpl) Push( expiretime Elem ) {
	var ele *list.Element

	for ele = q.l.Front(); ele != nil; ele = ele.Next() {
		if ele.Value.(Elem).GetTime() > expiretime.GetTime() {
			break
		}
	}

	// Insertion
	if ele != nil {
		q.l.InsertBefore(expiretime, ele)
	} else {
		q.l.PushBack(expiretime)
	}
    if q.queue_cap != 0 && q.l.Len() > q.queue_cap {
        q.Pop()
    }
}
func (q *FixedNumQueueImpl) Pop() Elem {
	time := q.l.Remove(q.l.Front())
	return time.(Elem)
}

func (q *FixedNumQueueImpl) Peek() Elem {
	evt := q.l.Front().Value.(Elem)
	return evt
}
