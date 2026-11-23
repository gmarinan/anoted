package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"anoted/internal/config"
	"anoted/internal/retention"
	"anoted/internal/transcribe"
	"github.com/spf13/cobra"
)

func modelsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List the transcription runtimes and models on disk, and how much space each takes",
		Long: "List what anoted installed (the Whisper and Parakeet venvs, the Parakeet model) and the\n" +
			"speech models its backends downloaded (faster-whisper models in the Hugging Face cache,\n" +
			"openai-whisper and whisper.cpp models in ~/.cache/whisper).\n\n" +
			"Anything the configured backend needs is marked in use and cannot be removed.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			printDownloads(cmd.OutOrStdout(), transcribe.Downloads(cfg.Transcription))
			return nil
		},
	}
	rm := &cobra.Command{
		Use:   "rm <id>...",
		Short: "Delete downloaded runtimes or models to free disk space",
		Long: "Delete the downloads with the given ids, as shown by `anoted models`.\n" +
			"Removing a runtime or model you later need again means reinstalling it\n" +
			"(`anoted setup` for Whisper, `anoted parakeet install` for Parakeet), or a\n" +
			"fresh download on the next transcription.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cfgPath, err := loadConfig()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			var failed error
			for _, id := range args {
				d, err := transcribe.RemoveDownload(cfg.Transcription, id)
				if err != nil {
					fmt.Fprintf(os.Stderr, "✗ %v\n", err)
					failed = errors.Join(failed, err)
					continue
				}
				fmt.Fprintf(out, "✓ removed %s (%s) — %s freed\n", d.Label, d.Path, retention.FormatBytes(d.Bytes))
				if d.Contains(cfg.Transcription.Binary) {
					// A binary path into a deleted venv would make the Whisper
					// backends fail later with a confusing "no such file".
					cfg.Transcription.Binary = ""
					if err := config.Save(cfgPath, cfg); err != nil {
						return fmt.Errorf("clear transcription.binary: %w", err)
					}
					fmt.Fprintln(out, "  transcription.binary pointed into it and has been cleared")
				}
			}
			if failed != nil {
				return errors.New("some downloads were not removed")
			}
			return nil
		},
	}
	cmd.AddCommand(rm)
	return cmd
}

func printDownloads(out io.Writer, ds []transcribe.Download) {
	if len(ds) == 0 {
		fmt.Fprintln(out, "No downloaded runtimes or models found.")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSIZE\tSTATUS\tPATH")
	var total, freeable int64
	for _, d := range ds {
		status := ""
		switch {
		case d.InUse:
			status = "in use"
		case d.Shared:
			status = "shared cache"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", d.ID, retention.FormatBytes(d.Bytes), status, d.Path)
		total += d.Bytes
		if !d.InUse {
			freeable += d.Bytes
		}
	}
	_ = tw.Flush()
	fmt.Fprintf(out, "\n%s on disk, %s can be freed with `anoted models rm <id>`.\n",
		retention.FormatBytes(total), retention.FormatBytes(freeable))
}
