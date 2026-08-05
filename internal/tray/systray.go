//go:build linux || windows

package tray

import (
	"sync"

	"fyne.io/systray"
)

func newPlatformIndicator(opts Options) Indicator {
	return &systrayIndicator{
		opts:  opts,
		state: StateMonitoring,
		ready: make(chan struct{}),
	}
}

type systrayIndicator struct {
	opts Options

	mu        sync.Mutex
	state     State
	tooltip   string
	onQuit    func()
	ready     chan struct{}
	readyOnce sync.Once
	started   bool
}

func (t *systrayIndicator) Start() error {
	t.mu.Lock()
	if t.started {
		t.mu.Unlock()
		return nil
	}
	t.started = true
	t.mu.Unlock()

	go systray.Run(t.onReady, t.onExit)
	<-t.ready
	t.SetState(StateMonitoring)
	return nil
}

func (t *systrayIndicator) onReady() {
	openItem := systray.AddMenuItem("Open recordings folder", "")
	quitItem := systray.AddMenuItem("Quit", "")

	go func() {
		for range openItem.ClickedCh {
			if t.opts.OnOpenFolder != nil {
				_ = t.opts.OnOpenFolder()
			}
		}
	}()
	go func() {
		for range quitItem.ClickedCh {
			t.mu.Lock()
			fn := t.onQuit
			if fn == nil {
				fn = t.opts.OnQuit
			}
			t.mu.Unlock()
			if fn != nil {
				fn()
			}
			systray.Quit()
			return
		}
	}()

	t.readyOnce.Do(func() { close(t.ready) })
}

func (t *systrayIndicator) onExit() {}

func (t *systrayIndicator) SetState(state State) {
	t.mu.Lock()
	t.state = state
	tooltip := t.tooltip
	t.mu.Unlock()
	if tooltip == "" {
		tooltip = tooltipFor(state)
	}
	systray.SetIcon(iconFor(state))
	systray.SetTooltip(tooltip)
}

func (t *systrayIndicator) SetTooltip(tooltip string) {
	t.mu.Lock()
	t.tooltip = tooltip
	state := t.state
	t.mu.Unlock()
	if tooltip == "" {
		tooltip = tooltipFor(state)
	}
	systray.SetTooltip(tooltip)
}

func (t *systrayIndicator) OnQuit(fn func()) {
	t.mu.Lock()
	t.onQuit = fn
	t.mu.Unlock()
}

func (t *systrayIndicator) Stop() {
	systray.Quit()
}
