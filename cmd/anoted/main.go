package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"anoted/internal/audio"
	"anoted/internal/autostart"
	"anoted/internal/config"
	"anoted/internal/detector"
	"anoted/internal/doctor"
	"anoted/internal/level"
	"anoted/internal/logging"
	"anoted/internal/open"
	"anoted/internal/platform"
	"anoted/internal/recorder"
	"anoted/internal/session"
	"anoted/internal/setup"
	"anoted/internal/transcribe"
	"anoted/internal/tray"
	"anoted/internal/tui"
)

var (
	cfgPath    string
	useMock    bool
	forceDummy bool
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "anoted",
		Short: "Meeting detection and audio recording TUI",
		RunE:  runTUI,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.EnsureDefault()
			if err != nil {
				return err
			}
			cfgDir, err := config.ConfigDir()
			if err != nil {
				return err
			}
			if err := session.MigrateLegacyIfEmpty(filepath.Join(cfgDir, "sessions.db")); err != nil {
				return err
			}
			_ = path
			return nil
		},
	}

	root.PersistentFlags().StringVar(&cfgPath, "config", "", "config file path")
	root.PersistentFlags().BoolVar(&useMock, "mock-detector", false, "use mock meeting detector")
	root.PersistentFlags().BoolVar(&forceDummy, "dummy-recorder", false, "use dummy recorder backend")

	root.AddCommand(
		setupCmd(),
		watchCmd(),
		statusCmd(),
		sessionsCmd(),
		configCmd(),
		doctorCmd(),
		autostartCmd(),
	)
	return root
}

func setupCmd() *cobra.Command {
	var tool, mode string
	var install bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Guided first-time setup (detection, config)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfig()
			if err != nil {
				return err
			}
			plat := platform.Detect()
			_, err = setup.Run(cfg, path, plat, setup.Options{
				Mode:    mode,
				Tool:    tool,
				Install: install,
			})
			return err
		},
	}
	cmd.Flags().StringVar(&mode, "mode", "", "detection mode: mic, window, both, none")
	cmd.Flags().StringVar(&tool, "tool", "", "window tool: xdotool, wmctrl, none")
	cmd.Flags().BoolVar(&install, "install", false, "install window tool without confirmation")
	return cmd
}

func watchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Start the TUI and watch for meetings",
		RunE:  runTUI,
	}
}

func runTUI(cmd *cobra.Command, args []string) error {
	logger, err := logging.SetupFile(slog.LevelInfo)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)

	cfg, path, err := loadConfig()
	if err != nil {
		return err
	}
	logger.Info("loaded config", "path", path)

	plat := platform.Detect()
	if setup.NeedsSetup(cfg, plat) {
		fmt.Fprintln(os.Stderr, "Tip: press S in the TUI to run setup, or: anoted setup")
	}

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	audioProvider := audio.NewProvider()

	tr := tray.New(tray.Options{
		Enabled: cfg.Privacy.TrayIndicator,
		OnOpenFolder: func() error {
			dir, err := cfg.ResolvedOutputDir()
			if err != nil {
				return err
			}
			return open.Open(dir, cfg.Desktop, open.KindFolder)
		},
	})
	if cfg.Privacy.TrayIndicator {
		if err := tray.EnsureLinuxBridge(); err != nil {
			logger.Warn("tray bridge unavailable", "err", err)
		}
		if err := tr.Start(); err != nil {
			logger.Warn("system tray unavailable", "err", err)
			tr = tray.New(tray.Options{Enabled: false})
		} else {
			defer tr.Stop()
		}
	}

	deps := tui.Deps{
		Config:       cfg,
		ConfigPath:   path,
		Platform:     plat,
		Detector:     detector.New(cfg, plat, useMock),
		Recorder:     recorder.New(cfg, plat, forceDummy),
		Store:        store,
		Audio:        audioProvider,
		LevelMonitor: level.NewMonitor(audioProvider),
		Transcriber:  transcribe.New(cfg),
		Tray:         tr,
	}

	p := tea.NewProgram(tui.NewModel(deps), tea.WithFilter(tui.SessionScrollFilter))
	tr.OnQuit(func() { p.Quit() })
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print current detection and recorder status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			plat := platform.Detect()
			det := detector.New(cfg, plat, useMock)
			snap, err := det.Poll(cmd.Context())
			if err != nil {
				return err
			}
			rec := recorder.New(cfg, plat, forceDummy)
			st := rec.Status()
			fmt.Printf("Platform: %s\n", plat.Name())
			fmt.Printf("Detector: %s (mode: %s)\n", det.Name(), cfg.Detection.Mode)
			fmt.Printf("In meeting: %v\n", snap.State.InMeeting)
			fmt.Printf("Provider: %s\n", snap.State.Provider)
			fmt.Printf("Recorder: %s (%s)\n", rec.Name(), st.Status)
			if st.SessionDir != "" {
				fmt.Printf("Session dir: %s\n", st.SessionDir)
			}
			return nil
		},
	}
}

func sessionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List recorded sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openStore()
			if err != nil {
				return err
			}
			defer store.Close()

			recs, err := store.List(50)
			if err != nil {
				return err
			}
			if len(recs) == 0 {
				fmt.Println("No sessions recorded yet.")
				return nil
			}
			for _, r := range recs {
				fmt.Printf("#%-4d %s %-12s %s\n",
					r.ID, session.FormatLocalTime(r.StartedAt, "2006-01-02 15:04"), r.Provider, r.Dir)
			}
			return nil
		},
	}
}

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show config file path and contents",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := config.EnsureDefault()
			if err != nil {
				return err
			}
			if cfgPath != "" {
				path = cfgPath
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Printf("Config: %s\n\n%s", path, string(data))
			return nil
		},
	}
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check system dependencies and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			rep := doctor.Run(cfg)
			fmt.Print(doctor.Format(rep))
			return nil
		},
	}
}

func autostartCmd() *cobra.Command {
	var enableRecord bool
	cmd := &cobra.Command{
		Use:   "autostart",
		Short: "Configure launch at login",
	}
	enable := &cobra.Command{
		Use:   "enable",
		Short: "Start anoted automatically when you log in",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !autostart.Available() {
				return autostart.ErrUnavailable
			}
			cfg, cfgPath, err := loadConfig()
			if err != nil {
				return err
			}
			entry, err := autostart.EntryFromConfig(cfg)
			if err != nil {
				return err
			}
			if err := autostart.Enable(entry); err != nil {
				return err
			}
			path, _ := autostart.Path()
			fmt.Println("Launch at login enabled.")
			if path != "" {
				fmt.Println("Entry:", path)
			}
			if !enableRecord {
				fmt.Println("Tip: add --record to also enable auto_record for meetings.")
				return nil
			}
			cfg.AutoRecord = true
			cfg.AutoRecordRequiresConfirmation = false
			if err := config.Save(cfgPath, cfg); err != nil {
				return err
			}
			fmt.Println("auto_record enabled (confirmation disabled).")
			fmt.Println("You are responsible for participant consent and local recording laws.")
			return nil
		},
	}
	enable.Flags().BoolVar(&enableRecord, "record", false, "also enable auto_record without confirmation")
	disable := &cobra.Command{
		Use:   "disable",
		Short: "Remove anoted from login startup",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := autostart.Disable(); err != nil {
				return err
			}
			fmt.Println("Launch at login disabled.")
			return nil
		},
	}
	status := &cobra.Command{
		Use:   "status",
		Short: "Show whether launch at login is enabled",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !autostart.Available() {
				return autostart.ErrUnavailable
			}
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			path, _ := autostart.Path()
			fmt.Printf("Launch at login: %v\n", autostart.Enabled())
			if path != "" {
				fmt.Printf("Entry: %s\n", path)
			}
			fmt.Printf("auto_record: %v\n", cfg.AutoRecord)
			fmt.Printf("auto_record_requires_confirmation: %v\n", cfg.AutoRecordRequiresConfirmation)
			return nil
		},
	}
	cmd.AddCommand(enable, disable, status)
	return cmd
}

func loadConfig() (config.Config, string, error) {
	if cfgPath != "" {
		cfg, err := config.Load(cfgPath)
		return cfg, cfgPath, err
	}
	return config.LoadDefault()
}

func openStore() (session.Store, error) {
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(cfgDir, "sessions.db")
	store := session.NewSQLiteStore(dbPath)
	if err := store.Open(); err != nil {
		return nil, err
	}
	return store, nil
}
