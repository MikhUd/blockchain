package waitgroup

import (
	"fmt"
	"github.com/MikhUd/blockchain/pkg/consts"
	"github.com/MikhUd/blockchain/pkg/utils"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type CallSite struct {
	file string
	line int
}

type WaitGroup struct {
	sync.WaitGroup
	count    int64
	callSite CallSite
}

func (wg *WaitGroup) Add(delta int) {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		fmt.Println("failed to get caller information")
	}

	atomic.AddInt64(&wg.count, int64(delta))
	wg.callSite = CallSite{file: file, line: line}
	wg.WaitGroup.Add(delta)
}

func (wg *WaitGroup) Done() {
	atomic.AddInt64(&wg.count, -1)
	wg.WaitGroup.Done()
}

func (wg *WaitGroup) GetCount() int {
	return int(atomic.LoadInt64(&wg.count))
}

func (wg *WaitGroup) GetCallSite() CallSite {
	return wg.callSite
}

func (wg *WaitGroup) PeriodicCheck(periodSec time.Duration) {
	var stopCh = make(chan struct{}, 1)
	ticker := time.NewTicker(periodSec)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if wg.GetCount() == 0 {
				close(stopCh)
				return
			}
			utils.Print(consts.Blue, fmt.Sprintf("Waitgroup len:%d, callers:%v", wg.GetCount(), wg.GetCallSite()))
		case <-stopCh:
			return
		}
	}
}
