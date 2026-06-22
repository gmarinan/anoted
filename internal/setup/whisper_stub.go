//go:build !linux

package setup

import (
	"io"

	"anoted/internal/config"
)

func setupTranscription(_ io.Reader, _ io.Writer, _ *config.Config, _ bool) {}
