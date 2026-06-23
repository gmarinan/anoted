package wasapi

import (
	"runtime"
	"sync"
)

type workItem struct {
	fn   func()
	done chan struct{}
}

var (
	workerOnce sync.Once
	workCh     chan workItem
)

func startWorker() {
	workCh = make(chan workItem, 64)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		for item := range workCh {
			item.fn()
			close(item.done)
		}
	}()
}

// onThread runs fn on a single dedicated OS thread. All WASAPI/malgo device
// init and teardown must use this path so COM apartment state stays consistent.
func onThread(fn func()) {
	workerOnce.Do(startWorker)
	done := make(chan struct{})
	workCh <- workItem{fn: fn, done: done}
	<-done
}

// runOnThread runs fn on the dedicated OS thread and returns its error.
func runOnThread(fn func() error) error {
	var err error
	onThread(func() {
		err = fn()
	})
	return err
}
