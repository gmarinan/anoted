package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"anoted/internal/config"
	"anoted/internal/retention"
	"anoted/internal/session"
	"github.com/spf13/cobra"
)

// retentionListLimit matches the TUI: a sweep has to see the oldest rows,
// which are the ones a small LIST limit would hide.
const retentionListLimit = 100000

func retentionCmd() *cobra.Command {
	var apply bool
	cmd := &cobra.Command{
		Use:   "retention",
		Short: "Show which recordings the audio retention policy would delete",
		Long: "Measure recording.wav across the library and list what the retention section of\n" +
			"the config would delete. Nothing is removed unless --apply is given.\n\n" +
			"--apply deletes even with auto_delete off: running it is the opt-in. It still needs\n" +
			"keep_days or keep_recordings, and it refuses while anoted watch is running, because\n" +
			"only that process knows which session is being recorded or transcribed.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig()
			if err != nil {
				return err
			}
			if apply {
				cfgDir, err := config.ConfigDir()
				if err != nil {
					return err
				}
				lock, err := session.AcquireInstanceLock(cfgDir)
				if err != nil {
					if errors.Is(err, session.ErrAlreadyRunning) {
						cmd.SilenceUsage = true
						return fmt.Errorf("%w; it applies the retention policy itself", err)
					}
					return err
				}
				defer func() { _ = lock.Release() }()
			}
			store, err := openStore()
			if err != nil {
				return err
			}
			defer store.Close()
			return runRetention(cmd.Context(), cmd.OutOrStdout(), cfg, store, time.Now(), apply)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "delete the listed recordings instead of only listing them")
	return cmd
}

func runRetention(ctx context.Context, w io.Writer, cfg config.Config, store session.Store, now time.Time, apply bool) error {
	root, err := cfg.ResolvedOutputDir()
	if err != nil {
		return err
	}
	recs, err := store.List(ctx, retentionListLimit)
	if err != nil {
		return err
	}
	pol := retention.PolicyFrom(cfg.Retention)
	items := retention.Scan(root, recs, cfg.Transcription, nil)
	plan := retention.BuildPlan(now, pol, items)

	fmt.Fprintf(w, "Audio on disk: %s\n", retention.Summarize(items))
	fmt.Fprintf(w, "Policy:        auto_delete=%v keep_days=%d keep_recordings=%d protect_untranscribed=%v\n",
		cfg.Retention.AutoDelete, cfg.Retention.KeepDays, cfg.Retention.KeepRecordings, pol.ProtectUntranscribed)
	if !pol.LimitsSet() {
		fmt.Fprintln(w, "No keep_days or keep_recordings set — nothing to delete.")
		return nil
	}
	for _, r := range plan.Remove {
		fmt.Fprintf(w, "  %-10s %s  (%s)\n", retention.FormatBytes(r.Bytes), r.Path, r.Reason)
	}
	if plan.Protected > 0 {
		fmt.Fprintf(w, "Keeping %d untranscribed recording(s) (%s).\n", plan.Protected, retention.FormatBytes(plan.ProtectedBytes))
	}
	if len(plan.Remove) == 0 {
		fmt.Fprintln(w, "Nothing to delete.")
		return nil
	}
	freed, _ := plan.Totals()
	if !apply {
		fmt.Fprintf(w, "Would free %s across %d file(s). Run with --apply to delete.\n", retention.FormatBytes(freed), len(plan.Remove))
		return nil
	}
	removed, freedBytes, err := retention.Apply(plan)
	fmt.Fprintf(w, "Freed %s across %d file(s). Transcripts kept.\n", retention.FormatBytes(freedBytes), removed)
	return err
}
