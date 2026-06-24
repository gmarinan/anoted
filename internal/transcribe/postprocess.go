package transcribe

import (
	"fmt"

	"anoted/internal/config"
)

func postProcessTranscription(cfg config.TranscriptionConfig, sessionDir, outDir string) (Result, error) {
	formats := config.NormalizeOutputFormats(cfg.OutputFormats)
	keep := keepNativeFormats(formats)
	fileBase := outputFileBase(cfg, sessionDir)

	if config.WantsMarkdown(formats) {
		if err := WriteMeetingMarkdown(sessionDir, outDir, cfg); err != nil {
			return Result{}, err
		}
	}

	pruneTranscriptFiles(outDir, keep, fileBase)
	removeTemporaryTxt(outDir, formats, fileBase)

	files := listOutputFiles(outDir, cfg, sessionDir)
	if len(files) == 0 {
		return Result{}, fmt.Errorf("no transcript files produced for formats %v", formats)
	}
	return Result{SessionDir: sessionDir, TranscriptDir: outDir, Files: files}, nil
}
