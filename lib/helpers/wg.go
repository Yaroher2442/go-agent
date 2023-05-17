package helpers

import (
	"sync"
	"sync/atomic"
)

type WaitGroupCount struct {
	sync.WaitGroup
	count int64
}

func NewWaitGroupCount() *WaitGroupCount {
	w := WaitGroupCount{WaitGroup: sync.WaitGroup{}, count: 0}
	return &w
}

func (wg *WaitGroupCount) Add(delta int) {
	atomic.AddInt64(&wg.count, int64(delta))
	wg.WaitGroup.Add(delta)
}

func (wg *WaitGroupCount) Done() {
	atomic.AddInt64(&wg.count, -1)
	wg.WaitGroup.Done()
}

func (wg *WaitGroupCount) GetCount() int {
	return int(atomic.LoadInt64(&wg.count))
}
