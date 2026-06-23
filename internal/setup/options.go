package setup

import "io"

const (
	ToolAuto    = "auto"
	ToolXdotool = "xdotool"
	ToolWmctrl  = "wmctrl"
	ToolNone    = "none"

	DetMic    = "mic"
	DetWindow = "window"
	DetBoth   = "both"
	DetNone   = "none"
)

// Options configures the guided setup flow.
type Options struct {
	Reader  io.Reader
	Writer  io.Writer
	Mode    string // mic, window, both, none
	Tool    string // xdotool, wmctrl, none (window/both on X11)
	Install bool
}
