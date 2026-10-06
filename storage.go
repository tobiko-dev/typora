//go:build windows

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type Config struct {
	Version         int      `json:"version"`
	Theme           string   `json:"theme"`
	DefaultFolder   string   `json:"default_folder"`
	Onboarded       bool     `json:"onboarded"`
	View            string   `json:"view"`
	Sidebar         bool     `json:"sidebar"`
	ConfirmTabClose bool     `json:"confirm_tab_close"`
	WindowX         int32    `json:"window_x,omitempty"`
	WindowY         int32    `json:"window_y,omitempty"`
	WindowW         int32    `json:"window_w,omitempty"`
	WindowH         int32    `json:"window_h,omitempty"`
	WindowMaximized bool     `json:"window_maximized,omitempty"`
	SidebarOrder    []string `json:"sidebar_order,omitempty"`
}

type SessionDoc struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Path        string `json:"path"`
	BufferPath  string `json:"buffer_path"`
	Cursor      int    `json:"cursor"`
	ScrollLine  int    `json:"scroll_line"`
	AutoCreated bool   `json:"auto_created"`
	LastDiskMod int64  `json:"last_disk_mod,omitempty"`
	DiskMissing bool   `json:"disk_missing,omitempty"`
}

type Session struct {
	Version int          `json:"version"`
	Active  int          `json:"active"`
	Docs    []SessionDoc `json:"docs"`
}

type ClosedDoc struct {
	SessionDoc
	ClosedAt int64 `json:"closed_at"`
}

func localDataDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	if base == "" {
		base = os.TempDir()
	}
	d := filepath.Join(base, "TyporaLocalV4")
	_ = os.MkdirAll(d, 0755)
	return d
}

func configPath() string  { return filepath.Join(localDataDir(), "config.json") }
func sessionPath() string { return filepath.Join(localDataDir(), "session.json") }
func buffersDir() string  { return filepath.Join(localDataDir(), "Session", "Buffers") }
func closedPath() string  { return filepath.Join(localDataDir(), "closed.json") }

func defaultNotesFolder() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, "Documents", "Typora Notes")
	}
	return filepath.Join(localDataDir(), "Notes")
}

func loadConfig() Config {
	cfg := Config{Version: 5, Theme: "dark", DefaultFolder: defaultNotesFolder(), View: "edit", Sidebar: true, ConfirmTabClose: true}
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(b, &cfg)
	}
	if cfg.DefaultFolder == "" {
		cfg.DefaultFolder = defaultNotesFolder()
	}
	if cfg.Theme == "" {
		cfg.Theme = "dark"
	}
	if cfg.View == "" {
		cfg.View = "edit"
	}
	if cfg.Version < 2 {
		cfg.Version = 2
		cfg.View = "edit"
	}
	if cfg.Version < 3 {
		cfg.Version = 3
	}
	if cfg.Version < 4 {
		cfg.Version = 4
	}
	if cfg.Version < 5 {
		cfg.Version = 5
		_ = saveConfig(cfg)
	}
	return cfg
}

func saveConfig(cfg Config) error {
	if cfg.Version < 5 {
		cfg.Version = 5
	}
	if err := os.MkdirAll(filepath.Dir(configPath()), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return fastAtomicWrite(configPath(), b, 0644)
}

func loadSession() Session {
	var s Session
	b, err := os.ReadFile(sessionPath())
	if err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func saveSession(s Session) error {
	s.Version = 1
	if err := os.MkdirAll(buffersDir(), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fastAtomicWrite(sessionPath(), b, 0644)
}

func loadClosed() []ClosedDoc {
	var c []ClosedDoc
	if b, err := os.ReadFile(closedPath()); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour).Unix()
	out := c[:0]
	for _, d := range c {
		if d.ClosedAt >= cutoff {
			out = append(out, d)
		}
	}
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func saveClosed(c []ClosedDoc) error {
	if len(c) > 20 {
		c = c[:20]
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return fastAtomicWrite(closedPath(), b, 0644)
}

func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

func bufferPath(id string) string {
	_ = os.MkdirAll(buffersDir(), 0755)
	return filepath.Join(buffersDir(), id+".txt")
}

var storageKernel32 = syscall.NewLazyDLL("kernel32.dll")
var procStorageMoveFileEx = storageKernel32.NewProc("MoveFileExW")

const (
	moveFileReplaceExisting = 0x00000001
	moveFileWriteThrough    = 0x00000008
)

func fastAtomicWrite(path string, data []byte, perm os.FileMode) error {
	if path == "" {
		return errors.New("empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + fmt.Sprintf(".tmp-%d", time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	from, _ := syscall.UTF16PtrFromString(tmp)
	to, _ := syscall.UTF16PtrFromString(path)
	r, _, callErr := procStorageMoveFileEx.Call(
		uintptr(unsafe.Pointer(from)),
		uintptr(unsafe.Pointer(to)),
		moveFileReplaceExisting,
	)
	if r == 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomic replace failed: %v", callErr)
	}
	return nil
}

func atomicWrite(path string, data []byte, perm os.FileMode) error {
	if path == "" {
		return errors.New("empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + fmt.Sprintf(".tmp-%d", time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	from, _ := syscall.UTF16PtrFromString(tmp)
	to, _ := syscall.UTF16PtrFromString(path)
	r, _, callErr := procStorageMoveFileEx.Call(
		uintptr(unsafe.Pointer(from)),
		uintptr(unsafe.Pointer(to)),
		moveFileReplaceExisting|moveFileWriteThrough,
	)
	if r == 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomic replace failed: %v", callErr)
	}
	return nil
}

var badFileChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)

func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(strings.TrimSuffix(s, ".md"), ".markdown")
	s = badFileChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, ". ")
	if len([]rune(s)) > 120 {
		s = string([]rune(s)[:120])
	}
	return s
}

func uniqueTitle(folder, wanted string, excludePath string) string {
	wanted = cleanTitle(wanted)
	if wanted == "" {
		wanted = "Untitled 01"
	}
	candidate := wanted
	ext := ".md"
	exists := func(name string) bool {
		p := filepath.Join(folder, name+ext)
		if excludePath != "" && strings.EqualFold(filepath.Clean(p), filepath.Clean(excludePath)) {
			return false
		}
		_, err := os.Stat(p)
		return err == nil
	}
	if !exists(candidate) {
		return candidate
	}
	base := wanted
	re := regexp.MustCompile(`^(.*?)(?:\s+(\d{2,}))?$`)
	if m := re.FindStringSubmatch(wanted); len(m) > 0 && m[1] != "" {
		base = strings.TrimSpace(m[1])
	}
	for i := 2; i < 10000; i++ {
		c := fmt.Sprintf("%s %02d", base, i)
		if !exists(c) {
			return c
		}
	}
	return wanted + " " + randomID()[:4]
}

func nextUntitled(folder string) string {
	inUse := func(n string) bool {
		wanted := filepath.Join(folder, n+".md")
		for _, d := range app.docs {
			if d != nil && d.Path != "" && strings.EqualFold(filepath.Clean(d.Path), filepath.Clean(wanted)) {
				return true
			}
		}
		_, err := os.Stat(wanted)
		return err == nil
	}
	for i := 1; i < 10000; i++ {
		n := fmt.Sprintf("Untitled %02d", i)
		if !inUse(n) {
			return n
		}
	}
	return "Untitled " + randomID()[:4]
}
