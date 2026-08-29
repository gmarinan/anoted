package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"anoted/internal/audio"
	"anoted/internal/autostart"
	"anoted/internal/buildinfo"
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
	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

var (
	cfgPath    string
	useMock    bool
	forceDummy bool
	logLevel   string
)

// idleFriendlyFPS caps the Bubble Tea renderer well below its 60 FPS default.
// This app is idle most of its life, and every frame tick is a timer wakeup
// that keeps the CPU out of deep sleep states on battery.
const idleFriendlyFPS = 15

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "anoted",
		Short:   "Meeting detection and audio recording TUI",
		Version: buildinfo.Version(),
		RunE:    runTUI,
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
	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	root.PersistentFlags().BoolVar(&useMock, "mock-detector", false, "use mock meeting detector")
	root.PersistentFlags().BoolVar(&forceDummy, "dummy-recorder", false, "use dummy recorder backend")

	root.AddCommand(
		setupCmd(),
		watchCmd(),
		statusCmd(),
		sessionsCmd(),
		transcribeCmd(),
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
	// Not named "level": that would shadow the internal/level package below.
	logLvl, err := logging.ParseLevel(logLevel)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}
	logger, logFile, err := logging.SetupFile(logLvl)
	if err != nil {
		// Not fatal — the TUI still runs — but say so, because the log is where
		// every diagnostic instruction points.
		fmt.Fprintf(os.Stderr, "anoted: file logging unavailable: %v\n", err)
	}
	defer func() { _ = logFile.Close() }()
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

	// One recorder per machine. Two instances sharing sessions.db is easy to
	// hit — an autostart entry plus a manual launch — and means two processes
	// recording the same meeting to different files.
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return err
	}
	lock, err := session.AcquireInstanceLock(cfgDir)
	if err != nil {
		cmd.SilenceUsage = true
		return err
	}
	defer func() { _ = lock.Release() }()

	store, err := openStore()
	if err != nil {
		return err
	}
	defer store.Close()

	// Recover anything the previous run left behind before the UI reads the
	// list: rows stuck "active" because anoted was killed mid-recording, and
	// recordings on disk that never made it into the database at all.
	if outDir, dirErr := cfg.ResolvedOutputDir(); dirErr == nil {
		if res, rErr := session.Reconcile(cmd.Context(), store, outDir); rErr != nil {
			logger.Warn("session reconciliation failed", "err", rErr)
		} else if res.Closed > 0 || res.Adopted > 0 || res.Secured > 0 {
			logger.Info("reconciled sessions",
				"closed", res.Closed, "adopted", res.Adopted, "secured", res.Secured)
		}
	}

	// The database and log predate the permission change on most installs, and
	// an existing file keeps its old mode however it is opened.
	if dbPath, pErr := storePath(); pErr == nil {
		if err := session.SecureFile(dbPath); err != nil {
			logger.Warn("could not restrict database permissions", "err", err)
		}
	}
	if logPath, pErr := logging.Path(); pErr == nil {
		if err := session.SecureFile(logPath); err != nil {
			logger.Warn("could not restrict log permissions", "err", err)
		}
	}
	if err := session.SecureFile(path); err != nil {
		logger.Warn("could not restrict config permissions", "err", err)
	}

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

	rec := recorder.New(cfg, plat, forceDummy)
	// Defence in depth: whichever path ends the event loop, never return from
	// runTUI with a capture child still running.
	defer func() { _ = rec.Stop(context.Background()) }()

	deps := tui.Deps{
		Config:       cfg,
		ConfigPath:   path,
		Platform:     plat,
		Detector:     detector.New(cfg, plat, useMock),
		Recorder:     rec,
		Store:        store,
		Audio:        audioProvider,
		LevelMonitor: level.NewMonitor(audioProvider),
		Transcriber:  transcribe.New(cfg),
		Tray:         tr,
	}

	// anoted sits idle for hours waiting for a meeting, so the renderer's default
	// 60 FPS ticker is 3600 pointless timer wakeups per minute — the single
	// largest source of idle CPU wakeups even with the level meter off. 15 FPS
	// caps repaint latency at 67ms, which is imperceptible for this UI.
	p := tea.NewProgram(tui.NewModel(deps),
		tea.WithFilter(tui.SessionScrollFilter),
		tea.WithFPS(idleFriendlyFPS),
	)
	// Send a message rather than calling p.Quit(): Bubble Tea returns from the
	// event loop on QuitMsg without dispatching it to Update, which skipped
	// performQuit and left an active recording running after the app exited.
	tr.OnQuit(func() { p.Send(tui.TrayQuitMsg{}) })
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

			recs, err := store.List(cmd.Context(), 50)
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

func transcribeCmd() *cobra.Command {
	var backend, model, device string
	cmd := &cobra.Command{
		Use:   "transcribe <session-dir>",
		Short: "Transcribe a recorded session and report how long it took",
		Long: "Transcribe a session directory containing recording.wav.\n\n" +
			"--backend lets you re-run the same audio through a different engine, " +
			"which is the simplest way to compare speed and output on your own hardware.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			if backend != "" {
				cfg.Transcription.Backend = backend
				// A configured CLI path pins the engine, so clear it when the
				// caller explicitly asks for a different backend.
				cfg.Transcription.Binary = ""
			}
			if model != "" {
				cfg.Transcription.Model = model
			}
			if device != "" {
				cfg.Transcription.Device = device
			}

			sessionDir := args[0]
			audio := transcribe.AudioPath(sessionDir)
			dur, durErr := transcribe.AudioDuration(audio)

			started := time.Now()
			res, err := transcribe.New(cfg).TranscribeSessionWithProgress(
				cmd.Context(), sessionDir,
				func(p transcribe.Progress) {
					if p.SegmentText != "" {
						fmt.Fprintf(os.Stderr, "\r%5.1f%% %-70.70s", p.Percent, p.SegmentText)
					}
				})
			elapsed := time.Since(started)
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}

			fmt.Printf("backend:  %s\n", cfg.Transcription.Backend)
			fmt.Printf("model:    %s\n", cfg.Transcription.Model)
			fmt.Printf("elapsed:  %s\n", elapsed.Round(time.Millisecond))
			if durErr == nil && dur > 0 {
				fmt.Printf("audio:    %s (%.2fx realtime)\n", dur.Round(time.Second), dur.Seconds()/elapsed.Seconds())
			}
			for _, f := range res.Files {
				fmt.Printf("wrote:    %s\n", f)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&backend, "backend", "", "override transcription backend (auto, openai-whisper, faster-whisper, whisper-cpp)")
	cmd.Flags().StringVar(&model, "model", "", "override model (e.g. large-v3, turbo)")
	cmd.Flags().StringVar(&device, "device", "", "override device (cpu, cuda, auto)")
	return cmd
}

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show config file path and contents",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configFilePath()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fmt.Printf("Config: %s\n\n%s", path, string(data))
			return nil
		},
	}
	cmd.AddCommand(configValidateCmd())
	return cmd
}

// configValidateCmd checks a config without starting anything, so a bad value
// can be caught before it fails in the middle of a meeting.
func configValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Check the config file for values that are out of range",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, path, err := loadConfig()
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				cmd.SilenceUsage = true
				return err
			}
			fmt.Printf("%s: OK\n", path)
			return nil
		},
	}
}

func configFilePath() (string, error) {
	if cfgPath != "" {
		return cfgPath, nil
	}
	return config.EnsureDefault()
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
			// Exit non-zero on failures so `anoted doctor && anoted watch` and
			// CI checks actually gate on the result instead of always passing.
			if failed := rep.Failures(); len(failed) > 0 {
				cmd.SilenceUsage = true
				return fmt.Errorf("doctor: %d check(s) failed: %s", len(failed), strings.Join(failed, ", "))
			}
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

func storePath() (string, error) {
	cfgDir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "sessions.db"), nil
}

func openStore() (session.Store, error) {
	dbPath, err := storePath()
	if err != nil {
		return nil, err
	}
	store := session.NewSQLiteStore(dbPath)
	if err := store.Open(); err != nil {
		return nil, err
	}
	return store, nil
}
