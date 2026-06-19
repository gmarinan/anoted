package doctor

import (
	"strings"

	"meetctl/internal/config"
	"meetctl/internal/open"
)

func desktopCheck(cfg config.Config) Check {
	folder := open.Detected(cfg.Desktop, open.KindFolder)
	file := open.Detected(cfg.Desktop, open.KindFile)
	detail := "folders: " + folder + "; files: " + file
	if cfg.Desktop.Opener == "" || cfg.Desktop.Opener == "auto" {
		opts := strings.Join(open.AvailableOpeners(), ", ")
		detail += " (Sessions → f to choose — " + opts + ")"
	}
	return Check{Name: "desktop_opener", Status: "ok", Detail: detail}
}
