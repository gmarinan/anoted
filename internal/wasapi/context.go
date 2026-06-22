//go:build windows

package wasapi

import (
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
)

var (
	ctxOnce sync.Once
	ctxInst *malgo.AllocatedContext
	ctxErr  error
)

// Context returns a process-wide malgo context.
func Context() (*malgo.AllocatedContext, error) {
	ctxOnce.Do(func() {
		ctxInst, ctxErr = malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	})
	if ctxErr != nil {
		return nil, fmt.Errorf("init malgo context: %w", ctxErr)
	}
	return ctxInst, nil
}
