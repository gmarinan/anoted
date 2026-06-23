package wasapi

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestOnThreadSerializesConcurrentJobs(t *testing.T) {
	var active int32
	var maxActive int32
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			onThread(func() {
				cur := atomic.AddInt32(&active, 1)
				for {
					prev := atomic.LoadInt32(&maxActive)
					if cur <= prev || atomic.CompareAndSwapInt32(&maxActive, prev, cur) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
				atomic.AddInt32(&active, -1)
			})
		}()
	}
	wg.Wait()

	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("max concurrent jobs on worker = %d, want 1", got)
	}
}

func TestRunOnThreadReturnsError(t *testing.T) {
	err := runOnThread(func() error {
		return errWorkerTest
	})
	if err != errWorkerTest {
		t.Fatalf("got %v", err)
	}
}

var errWorkerTest = &workerTestErr{}

type workerTestErr struct{}

func (e *workerTestErr) Error() string { return "worker test error" }
