package structures

import (
	"sync"
	"sync/atomic"
)

type WaitGroupCount struct {
	sync.WaitGroup
	count   int32
	channel chan int
}

func (wg *WaitGroupCount) Add(delta int) {
	atomic.AddInt32(&wg.count, int32(delta))
	wg.WaitGroup.Add(delta)
}

func (wg *WaitGroupCount) Done() {
	if atomic.AddInt32(&wg.count, -1) < 0 {
		atomic.StoreInt32(&wg.count, 0)
	} else {
		wg.WaitGroup.Done()
	}
	if wg.channel == nil {
		wg.channel = make(chan int)
	}
	wg.channel <- 0
}

func (wg *WaitGroupCount) AllDone() {
	for {
		if wg.GetCount() == 0 {
			return
		}
		wg.Done()
	}
}

func (wg *WaitGroupCount) GetCount() int {
	return int(atomic.LoadInt32(&wg.count))
}

func (wg *WaitGroupCount) WaitFor(count int) chan struct{} {
	done := make(chan struct{})
	if count >= wg.GetCount() {
		close(done)
		return done
	}
	if wg.channel == nil {
		wg.channel = make(chan int)
	}
	go func() {
		for {
			select {
			case c, f := <-wg.channel:
				if c <= count || f {
					close(done)
					return
				}
			}
		}
	}()
	return done
}
