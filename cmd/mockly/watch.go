package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/dever-labs/mockly/internal/config"
)

// reloadConfig loads and validates the config at path, exactly like the
// start command's initial load, so --watch reloads use the same rules
// (inline schema refs resolved, defaults applied, validated) as startup.
// It never applies a reload that fails: the caller keeps serving the
// previously loaded config.
func reloadConfig(path string) (*config.Config, error) {
	cfg, err := loadResolvedConfig(path)
	if err != nil {
		return nil, err
	}
	if errs := config.Validate(cfg); len(errs) > 0 {
		msg := fmt.Sprintf("%d validation error(s)", len(errs))
		for _, e := range errs {
			msg += "\n  - " + e.Error()
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return cfg, nil
}

// watchConfig watches every file that contributed to the config at
// cfgPath (main file, directory-mode auto-included siblings, and explicit
// Include globs) and calls apply with the freshly reloaded config each
// time one of them changes, until ctx is done.
//
// Scope: a reload only updates mocks on already-running protocol servers
// (the same SetMocks/SetRules/SetMailboxes/SetFiles code path the
// management API's bulk-set endpoints use) — enabling a protocol that
// wasn't running at startup, or changing a port, requires a restart and is
// intentionally not supported here. Reload failures (invalid YAML, failed
// validation) are logged to stderr; the previous config keeps serving.
//
// fsnotify watches directories rather than individual files so edits made
// via an atomic rename (as many editors/tools do) are still seen; events
// are matched back to the watched files by absolute path, and rapid
// bursts of events (common with editor saves) are debounced.
func watchConfig(ctx context.Context, cfgPath string, apply func(*config.Config)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("starting config watcher: %w", err)
	}
	defer func() { _ = watcher.Close() }()

	watchedDirs := map[string]bool{}
	addDirsFor := func(files []string) {
		for _, f := range files {
			dir := filepath.Dir(f)
			if watchedDirs[dir] {
				continue
			}
			if err := watcher.Add(dir); err != nil {
				fmt.Fprintf(os.Stderr, "warn: watch %q: %v\n", dir, err)
				continue
			}
			watchedDirs[dir] = true
		}
	}

	files, err := config.ResolvedFiles(cfgPath)
	if err != nil {
		return fmt.Errorf("resolving files to watch: %w", err)
	}
	addDirsFor(files)
	fmt.Printf("→ Watching config   %s (%d file(s))\n", cfgPath, len(files))

	const debounce = 200 * time.Millisecond
	var timer *time.Timer
	reload := func() {
		cfg, err := reloadConfig(cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: config reload failed, keeping previous config: %v\n", err)
			return
		}
		apply(cfg)
		fmt.Println("→ Config reloaded")

		// Picking up a newly-added include file (or a sibling file in
		// directory mode) requires watching its directory too.
		if files, err := config.ResolvedFiles(cfgPath); err == nil {
			addDirsFor(files)
		}
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, reload)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "warn: config watcher error: %v\n", err)
		}
	}
}
