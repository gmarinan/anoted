package open

// Kind is what is being opened from the TUI.
type Kind int

const (
	KindFolder Kind = iota
	KindFile
)
