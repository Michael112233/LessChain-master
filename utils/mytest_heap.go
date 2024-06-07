package utils

import (
	"container/heap"
	"fmt"
)

type heapdatauint64test []uint64

func (h heapdatauint64test) Len() int           { return len(h) }
func (h heapdatauint64test) Less(i, j int) bool { return h[i] < h[j] }
func (h heapdatauint64test) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *heapdatauint64test) Push(x interface{}) {
	*h = append(*h, x.(uint64))
}
func (h *heapdatauint64test) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
func mytestHeap() {

	tmp := new(heapdatauint64test)
	for i := 5; i > 0; i-- {
		heap.Push(tmp, uint64(i))
	}
	for i := 5; i < 10; i++ {
		heap.Push(tmp, uint64(i))
	}

	for tmp.Len() > 0 {
		x := heap.Pop(tmp)
		fmt.Println(x)
	}

}
