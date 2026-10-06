//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// sidebarWatchGeneration invalidates the previous watcher whenever the user
// changes the notes folder. The watcher intentionally compares only the set of
// visible note filenames, so Typora's atomic temp-file autosaves do not trigger
// needless sidebar refreshes.
var sidebarWatchGeneration atomic.Uint64

func visibleNoteNames(folder string) ([]string, error) {
	ents, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".md" || ext == ".markdown" || ext == ".txt" {
			out = append(out, e.Name())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out, nil
}

func sameFilenameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}

func startSidebarFolderWatcher(folder string) {
	folder = filepath.Clean(folder)
	gen := sidebarWatchGeneration.Add(1)
	go func() {
		previous, err := visibleNoteNames(folder)
		if err != nil {
			log.Printf("sidebar watcher could not read folder=%q: %v", folder, err)
			previous = nil
		}
		log.Printf("sidebar watcher started folder=%q", folder)

		ticker := time.NewTicker(400 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if sidebarWatchGeneration.Load() != gen {
				log.Printf("sidebar watcher stopped folder=%q", folder)
				return
			}
			current, err := visibleNoteNames(folder)
			if err != nil {
				continue
			}
			if sameFilenameSet(previous, current) {
				continue
			}
			previous = append(previous[:0], current...)
			log.Printf("sidebar watcher detected external filename change folder=%q", folder)
			queueSidebarRefresh(folder)
		}
	}()
}

func stopSidebarFolderWatcher() {
	sidebarWatchGeneration.Add(1)
}
