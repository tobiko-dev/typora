//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type ioJob struct {
	Kind        string
	DocID       string
	Path        string
	BufferPath  string
	Content     string
	LastDiskMod int64
	Revision    uint64
	Force       bool
	Manual      bool
	RemoveAfter string
}

type ioResult struct {
	Kind     string
	DocID    string
	Path     string
	Revision uint64
	Mod      int64
	Err      string
	Conflict bool
	Missing  bool
	Manual   bool
	Files    []string
	Elapsed  time.Duration
}

type sessionPersistJob struct {
	Session Session
	Closed  []ClosedDoc
}

var (
	recoveryJobs   = make(chan ioJob, 1)
	diskJobs       = make(chan ioJob, 1)
	sidebarJobs    = make(chan ioJob, 1)
	sessionJobs    = make(chan sessionPersistJob, 1)
	configJobs     = make(chan Config, 1)
	ioOnce         sync.Once
	ioResultMu     sync.Mutex
	ioResults      []ioResult
	canceledDocIDs sync.Map
)

func cancelDocIO(id string) {
	if id != "" {
		canceledDocIDs.Store(id, true)
	}
}

func docIOCanceled(id string) bool {
	if id == "" {
		return false
	}
	_, ok := canceledDocIDs.Load(id)
	return ok
}

func startIOWorkers() {
	ioOnce.Do(func() {
		go ioWorker(recoveryJobs)
		go ioWorker(diskJobs)
		go ioWorker(sidebarJobs)
		go sessionPersistWorker()
		go configPersistWorker()
	})
}

func queueSessionPersist(s Session, closed []ClosedDoc) {
	j := sessionPersistJob{Session: s, Closed: append([]ClosedDoc(nil), closed...)}
	select {
	case sessionJobs <- j:
		return
	default:
	}
	select {
	case <-sessionJobs:
	default:
	}
	select {
	case sessionJobs <- j:
	default:
	}
}

func sessionPersistWorker() {
	for j := range sessionJobs {
		_ = saveSession(j.Session)
		_ = saveClosed(j.Closed)
	}
}

func queueConfigPersist(cfg Config) {
	copyCfg := cfg
	copyCfg.SidebarOrder = append([]string(nil), cfg.SidebarOrder...)
	select {
	case configJobs <- copyCfg:
		return
	default:
	}
	select {
	case <-configJobs:
	default:
	}
	select {
	case configJobs <- copyCfg:
	default:
	}
}

func configPersistWorker() {
	for cfg := range configJobs {
		_ = saveConfig(cfg)
	}
}

func enqueueLatest(ch chan ioJob, j ioJob) {
	select {
	case ch <- j:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- j:
	default:
	}
}

func snapshotJob(d *Document) ioJob {
	return ioJob{
		DocID: d.ID, Path: d.Path, BufferPath: d.BufferPath, Content: d.Content,
		LastDiskMod: d.LastDiskMod, Revision: d.Revision,
	}
}

func queueRecovery(d *Document) {
	if d == nil {
		return
	}
	j := snapshotJob(d)
	j.Kind = "recovery"
	enqueueLatest(recoveryJobs, j)
}

func queueDiskSave(d *Document, force, manual bool) {
	if d == nil || d.Path == "" {
		return
	}
	if d.DiskMissing && !(force && manual) {
		return
	}
	j := snapshotJob(d)
	j.Kind = "save"
	j.Force = force
	j.Manual = manual
	enqueueLatest(diskJobs, j)
}

func queueRenameSave(d *Document, oldPath string) {
	if d == nil || d.Path == "" {
		return
	}
	j := snapshotJob(d)
	j.Kind = "save"
	j.Force = true
	j.RemoveAfter = oldPath
	enqueueLatest(diskJobs, j)
}

func queueSidebarRefresh(folder string) {
	if folder == "" {
		return
	}
	enqueueLatest(sidebarJobs, ioJob{Kind: "sidebar", Path: folder})
}

func ioWorker(ch <-chan ioJob) {
	for j := range ch {
		started := time.Now()
		if j.DocID != "" && docIOCanceled(j.DocID) {
			continue
		}
		r := ioResult{Kind: j.Kind, DocID: j.DocID, Path: j.Path, Revision: j.Revision, Manual: j.Manual}
		if j.Kind == "sidebar" {
			ents, err := os.ReadDir(j.Path)
			if err != nil {
				r.Err = err.Error()
			} else {
				for _, e := range ents {
					if e.IsDir() {
						continue
					}
					ext := strings.ToLower(filepath.Ext(e.Name()))
					if ext == ".md" || ext == ".markdown" || ext == ".txt" {
						r.Files = append(r.Files, e.Name())
					}
				}
				sort.Slice(r.Files, func(i, k int) bool { return strings.ToLower(r.Files[i]) < strings.ToLower(r.Files[k]) })
			}
			r.Elapsed = time.Since(started)
			publishIOResult(r)
			continue
		}
		if j.Kind == "recovery" {
			if j.BufferPath != "" {
				if err := fastAtomicWrite(j.BufferPath, []byte(j.Content), 0644); err != nil {
					r.Err = err.Error()
				}
			}
			r.Elapsed = time.Since(started)
			publishIOResult(r)
			continue
		}

		if j.Path == "" {
			r.Elapsed = time.Since(started)
			publishIOResult(r)
			continue
		}
		if j.LastDiskMod != 0 {
			now := fileModNano(j.Path)
			if now == 0 && !(j.Force && j.Manual) {
				r.Missing = true
				r.Elapsed = time.Since(started)
				publishIOResult(r)
				continue
			}
			if !j.Force && now != 0 && now != j.LastDiskMod {
				r.Conflict = true
				r.Elapsed = time.Since(started)
				publishIOResult(r)
				continue
			}
		}
		if j.DocID != "" && docIOCanceled(j.DocID) {
			continue
		}
		if err := atomicWrite(j.Path, []byte(j.Content), 0644); err != nil {
			r.Err = err.Error()
		} else {
			r.Mod = fileModNano(j.Path)
			if j.RemoveAfter != "" && !strings.EqualFold(filepath.Clean(j.RemoveAfter), filepath.Clean(j.Path)) {
				_ = os.Remove(j.RemoveAfter)
			}
		}
		r.Elapsed = time.Since(started)
		publishIOResult(r)
	}
}

func publishIOResult(r ioResult) {
	if r.Elapsed >= 500*time.Millisecond {
		log.Printf("slow background I/O kind=%s path=%q elapsed=%s err=%q conflict=%v", r.Kind, r.Path, r.Elapsed, r.Err, r.Conflict)
	}
	ioResultMu.Lock()
	ioResults = append(ioResults, r)
	ioResultMu.Unlock()
	if app.hwnd != 0 {
		procPostMessage.Call(uintptr(app.hwnd), WM_APP_IO_RESULT, 0, 0)
	}
}

func popIOResults() []ioResult {
	ioResultMu.Lock()
	out := append([]ioResult(nil), ioResults...)
	ioResults = ioResults[:0]
	ioResultMu.Unlock()
	return out
}

func findDocByID(id string) *Document {
	for _, d := range app.docs {
		if d.ID == id {
			return d
		}
	}
	return nil
}

func reconcileOpenDocsWithDisk(folder string, files []string) {
	folder = filepath.Clean(folder)
	present := make(map[string]bool, len(files))
	for _, f := range files {
		present[strings.ToLower(f)] = true
	}
	changed := false
	for _, d := range app.docs {
		if d == nil || d.Path == "" || !strings.EqualFold(filepath.Clean(filepath.Dir(d.Path)), folder) {
			continue
		}
		name := strings.ToLower(filepath.Base(d.Path))
		exists := present[name]
		if !exists && d.LastDiskMod != 0 && !d.DiskMissing {
			d.DiskMissing = true
			changed = true
			if d == activeDoc() {
				app.saveStatus = "Deleted externally • Ctrl+S to restore or Ctrl+Shift+S for Save As"
			}
			log.Printf("open note deleted externally path=%q doc=%s; autosave suspended", d.Path, d.ID)
		} else if exists && d.DiskMissing {
			d.DiskMissing = false
			d.LastDiskMod = fileModNano(d.Path)
			changed = true
			if d == activeDoc() {
				app.saveStatus = "File restored on disk"
			}
		}
	}
	if changed {
		persistSessionAsync()
		invalidateStatus()
	}
}

func handleIOResults() {
	statusDirty := false
	sidebarDirty := false
	for _, r := range popIOResults() {
		if r.Kind == "sidebar" {
			if r.Err == "" && strings.EqualFold(filepath.Clean(r.Path), filepath.Clean(app.cfg.DefaultFolder)) {
				reconcileOpenDocsWithDisk(r.Path, r.Files)
				app.sidebarFiles = applySidebarOrder(r.Files, app.cfg.SidebarOrder)
				if app.sidebarScroll > max(0, len(app.sidebarFiles)-1) {
					app.sidebarScroll = max(0, len(app.sidebarFiles)-1)
				}
				sidebarDirty = true
			}
			continue
		}
		d := findDocByID(r.DocID)
		if d == nil {
			continue
		}
		if r.Kind == "save" && d.Path != r.Path {
			continue
		}
		if r.Kind == "recovery" {
			if r.Err != "" && d == activeDoc() {
				app.saveStatus = "Recovery failed: " + r.Err
				statusDirty = true
			}
			continue
		}
		if r.Missing {
			d.DiskMissing = true
			persistSessionAsync()
			if d == activeDoc() {
				app.saveStatus = "Deleted externally • Ctrl+S to restore or Ctrl+Shift+S for Save As"
				statusDirty = true
			}
			continue
		}
		if r.Conflict {
			d.Conflict = true
			if d == activeDoc() {
				app.saveStatus = "External change detected"
				statusDirty = true
			}
			if r.Manual {
				ans := message("This file changed outside Typora.\n\nOverwrite the disk version with your current tab?\n\nChoose No to keep both versions and use Ctrl+Shift+S for Save As.", "External change detected", MB_YESNO|MB_ICONWARNING)
				if ans == IDYES {
					queueDiskSave(d, true, false)
					if d == activeDoc() {
						app.saveStatus = "Saving…"
						statusDirty = true
					}
				}
			}
			continue
		}
		if r.Err != "" {
			if d == activeDoc() {
				app.saveStatus = "Save failed: " + r.Err
				statusDirty = true
			}
			continue
		}
		if r.Mod != 0 {
			d.LastDiskMod = r.Mod
			d.DiskMissing = false
		}
		d.Conflict = false
		if d == activeDoc() {
			if d.Revision <= r.Revision {
				app.saveStatus = "Saved"
			} else {
				app.saveStatus = "Saving…"
			}
			statusDirty = true
		}
	}
	if sidebarDirty {
		invalidateSidebar()
	}
	if statusDirty {
		invalidateStatus()
	}
}
