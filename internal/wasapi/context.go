//go:build windows

package wasapi

import (
	"fmt"

	"github.com/gen2brain/malgo"
)

var (
	ctxInst  *malgo.AllocatedContext
	ctxErr   error
	ctxReady bool
)

func ensureContext() {
	if ctxReady {
		return
	}
	ctxInst, ctxErr = malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	ctxReady = true
}

// contextOnWorker returns the malgo context. Call only from onThread worker jobs.
func contextOnWorker() (*malgo.AllocatedContext, error) {
	ensureContext()
	if ctxErr != nil {
		return nil, fmt.Errorf("init malgo context: %w", ctxErr)
	}
	return ctxInst, nil
}

// Context returns a process-wide malgo context initialized on the WASAPI worker thread.
func Context() (*malgo.AllocatedContext, error) {
	onThread(ensureContext)
	if ctxErr != nil {
		return nil, fmt.Errorf("init malgo context: %w", ctxErr)
	}
	return ctxInst, nil
}
