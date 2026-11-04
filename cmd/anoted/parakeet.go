package main

import (
	"fmt"
	"os"

	"anoted/internal/config"
	"anoted/internal/transcribe"
	"github.com/spf13/cobra"
)

func parakeetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parakeet",
		Short: "Manage the Parakeet transcription backend (NVIDIA Parakeet TDT 0.6B v3)",
		Long: "Parakeet is a multilingual speech model that transcribes a meeting in seconds on a GPU,\n" +
			"many times faster than Whisper. It cannot be told the language: on a non-English meeting\n" +
			"it occasionally switches a passage to English, which anoted detects and re-decodes.\n" +
			"Setting transcription.language (e.g. es) makes that check stricter.",
	}
	var use bool
	install := &cobra.Command{
		Use:   "install",
		Short: "Install the Parakeet runtime and download the model (~2.5 GB with a GPU)",
		Long: "Create a separate venv next to the Whisper one, install onnx-asr with the onnxruntime\n" +
			"build for this machine (GPU when an NVIDIA driver is present), and download the model.\n" +
			"Re-running it repairs or updates the install.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, cfgPath, err := loadConfig()
			if err != nil {
				return err
			}
			fmt.Println("Installing Parakeet:")
			if err := transcribe.InstallParakeet(os.Stdout, os.Stdout, os.Stderr); err != nil {
				return err
			}
			if !use {
				if cfg.Transcription.Backend != transcribe.BackendParakeet {
					fmt.Println("To use it: set transcription.backend: parakeet (Config tab → backend), or re-run with --use.")
				}
				return nil
			}
			cfg.Transcription.Backend = transcribe.BackendParakeet
			if err := config.Save(cfgPath, cfg); err != nil {
				return err
			}
			fmt.Println("transcription.backend set to parakeet. Restart anoted watch for it to take effect.")
			return nil
		},
	}
	install.Flags().BoolVar(&use, "use", false, "also set transcription.backend: parakeet")
	cmd.AddCommand(install)
	return cmd
}
