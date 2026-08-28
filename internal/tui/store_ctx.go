package tui

import (
	"context"
	"time"
)

// storeTimeout bounds a session-store query.
//
// SQLite is opened with a five-second busy timeout, so a contended write could
// previously block that long with no way to cancel — and several of these calls
// ran on the Bubble Tea Update loop, freezing the UI with it. A context makes
// the wait bounded and cancellable.
const storeTimeout = 5 * time.Second

func storeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), storeTimeout)
}
