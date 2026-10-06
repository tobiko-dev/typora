//go:build windows

package main

import (
	"os"
	"path/filepath"

	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

func loWord(v uintptr) uint16    { return uint16(v & 0xffff) }
func hiWord(v uintptr) uint16    { return uint16((v >> 16) & 0xffff) }
func signedWord(v uintptr) int32 { return int32(int16(v & 0xffff)) }
func send(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := procSendMessage.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}
func setText(hwnd syscall.Handle, s string) {
	procSetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(wstr(s))))
}
func getText(hwnd syscall.Handle) string {
	l, _, _ := procGetWindowTextLength.Call(uintptr(hwnd))
	if l == 0 {
		return ""
	}
	b := make([]uint16, l+1)
	procGetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&b[0])), uintptr(l+1))
	return syscall.UTF16ToString(b)
}
func invalidate() {
	if app.hwnd != 0 {
		procInvalidateRect.Call(uintptr(app.hwnd), 0, 0)
	}
}

func invalidateRegion(r RECT) {
	if app.hwnd == 0 {
		return
	}
	procInvalidateRect.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(&r)), 0)
}

func invalidateStatus() {
	if app.clientH <= 0 {
		return
	}
	invalidateRegion(RECT{0, app.clientH - STATUS_H, app.clientW, app.clientH})
}

func invalidateSidebar() {
	if !app.cfg.Sidebar {
		return
	}
	invalidateRegion(RECT{0, TOP_H, SIDEBAR_W, app.clientH - STATUS_H})
}

func colors() palette {
	if app.cfg.Theme == "light" {
		return palette{rgb(247, 248, 250), rgb(255, 255, 255), rgb(242, 244, 248), rgb(224, 227, 234), rgb(31, 34, 40), rgb(112, 118, 130), rgb(73, 101, 220), rgb(230, 235, 255), rgb(255, 255, 255), rgb(30, 150, 92), rgb(210, 70, 70)}
	}
	return palette{rgb(18, 21, 27), rgb(23, 27, 34), rgb(28, 32, 40), rgb(43, 48, 59), rgb(238, 240, 245), rgb(143, 151, 166), rgb(111, 143, 255), rgb(38, 46, 74), rgb(20, 24, 30), rgb(92, 214, 151), rgb(255, 105, 105)}
}

func createFont(size int32, weight int32, face string) syscall.Handle {
	r, _, _ := procCreateFont.Call(uintptr(-size), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(wstr(face))))
	return syscall.Handle(r)
}

func createChild(class string, style uint32, id int) syscall.Handle {
	hInst, _, _ := procGetModuleHandle.Call(0)
	r, _, _ := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(wstr(class))), uintptr(unsafe.Pointer(wstr(""))), uintptr(style), 0, 0, 10, 10, uintptr(app.hwnd), uintptr(id), hInst, 0)
	return syscall.Handle(r)
}

func setEditorStyle() {
	p := colors()
	if app.editorBrush != 0 {
		procDeleteObject.Call(uintptr(app.editorBrush))
		app.editorBrush = 0
	}
	if b, _, _ := procCreateSolidBrush.Call(uintptr(p.editorBg)); b != 0 {
		app.editorBrush = syscall.Handle(b)
	}
	send(app.editor, EM_SETBKGNDCOLOR, 0, uintptr(p.editorBg))
	send(app.preview, EM_SETBKGNDCOLOR, 0, uintptr(p.editorBg))
	var cf CHARFORMAT2
	cf.CbSize = uint32(unsafe.Sizeof(cf))
	cf.DwMask = CFM_COLOR | CFM_FACE | CFM_SIZE
	cf.CrTextColor = p.text
	cf.YHeight = 220
	face := utf16.Encode([]rune("Cascadia Mono"))
	copy(cf.SzFaceName[:], face)
	send(app.editor, EM_SETCHARFORMAT, SCF_ALL, uintptr(unsafe.Pointer(&cf)))
	send(app.editor, EM_SETMARGINS, EC_LEFTMARGIN|EC_RIGHTMARGIN, uintptr(24|(24<<16)))
	send(app.preview, EM_SETMARGINS, EC_LEFTMARGIN|EC_RIGHTMARGIN, uintptr(28|(28<<16)))
	send(app.editor, EM_SETEVENTMASK, 0, ENM_CHANGE)
	send(app.preview, EM_SETEVENTMASK, 0, ENM_LINK)

	send(app.preview, EM_SETEDITSTYLE, SES_NOFOCUSLINKNOTIFY|SES_HYPERLINKTOOLTIPS, SES_NOFOCUSLINKNOTIFY|SES_HYPERLINKTOOLTIPS)
	dark := int32(0)
	if app.cfg.Theme != "light" {
		dark = 1
	}
	if app.hwnd != 0 {
		procDwmSetWindowAttribute.Call(uintptr(app.hwnd), 20, uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))
	}

	theme := "Explorer"
	if dark != 0 {
		theme = "DarkMode_Explorer"
	}
	for _, h := range []syscall.Handle{app.editor, app.preview, app.renameEdit} {
		if h != 0 {
			procSetWindowTheme.Call(uintptr(h), uintptr(unsafe.Pointer(wstr(theme))), 0)
			send(h, WM_THEMECHANGED, 0, 0)
		}
	}
}

func message(text, title string, flags uintptr) int {
	r, _, _ := procMessageBox.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(wstr(text))), uintptr(unsafe.Pointer(wstr(title))), flags)
	return int(r)
}

func activeDoc() *Document {
	if app.active >= 0 && app.active < len(app.docs) {
		return app.docs[app.active]
	}
	return nil
}
func syncEditorToDoc() {
	if app.suppress {
		return
	}
	if d := activeDoc(); d != nil {
		d.Content = getText(app.editor)
	}
}
func setEditorFromDoc() {
	d := activeDoc()
	app.suppress = true
	if d != nil {
		setText(app.editor, d.Content)
	} else {
		setText(app.editor, "")
	}
	app.suppress = false
	updateWindowTitle()
	schedulePreview()
	invalidate()
}

func fileModNano(path string) int64 {
	if st, err := os.Stat(path); err == nil {
		return st.ModTime().UnixNano()
	}
	return 0
}
func writeBuffer(d *Document) error {
	if d.BufferPath == "" {
		d.BufferPath = bufferPath(d.ID)
	}
	return atomicWrite(d.BufferPath, []byte(d.Content), 0644)
}

func saveDoc(d *Document, force bool) error {
	if d == nil || d.Path == "" {
		return nil
	}
	if !force && d.LastDiskMod != 0 {
		now := fileModNano(d.Path)
		if now != 0 && now != d.LastDiskMod {
			return errExternalChange
		}
	}
	if err := writeBuffer(d); err != nil {
		return err
	}
	if err := atomicWrite(d.Path, []byte(d.Content), 0644); err != nil {
		return err
	}
	d.LastDiskMod = fileModNano(d.Path)
	d.Conflict = false
	return nil
}

func sessionSnapshot() Session {
	s := Session{Active: app.active}
	for _, d := range app.docs {
		s.Docs = append(s.Docs, SessionDoc{ID: d.ID, Title: d.Title, Path: d.Path, BufferPath: d.BufferPath, AutoCreated: d.AutoCreated, LastDiskMod: d.LastDiskMod, DiskMissing: d.DiskMissing})
	}
	return s
}

func persistSessionAsync() {
	queueSessionPersist(sessionSnapshot(), app.closed)
}

func persistSessionNow() {
	syncEditorToDoc()
	if d := activeDoc(); d != nil {
		_ = writeBuffer(d)
	}
	_ = saveSession(sessionSnapshot())
	_ = saveClosed(app.closed)
}

func scheduleAutosave() {
	procKillTimer.Call(uintptr(app.hwnd), TIMER_AUTOSAVE)
	procSetTimer.Call(uintptr(app.hwnd), TIMER_AUTOSAVE, 600, 0)
}
func schedulePreview() {
	if app.view == "edit" {
		return
	}
	procKillTimer.Call(uintptr(app.hwnd), TIMER_PREVIEW)
	procSetTimer.Call(uintptr(app.hwnd), TIMER_PREVIEW, 180, 0)
}
func scheduleRecovery() {
	procKillTimer.Call(uintptr(app.hwnd), TIMER_RECOVERY)
	procSetTimer.Call(uintptr(app.hwnd), TIMER_RECOVERY, 100, 0)
}
func saveActive(force bool) {
	syncEditorToDoc()
	d := activeDoc()
	if d == nil {
		return
	}
	if d.DiskMissing && !force {
		app.saveStatus = "Deleted externally • Ctrl+S to restore or Ctrl+Shift+S for Save As"
		invalidateStatus()
		return
	}
	queueDiskSave(d, force, false)
	app.saveStatus = "Saving…"
	invalidate()
}

func saveAll() {
	syncEditorToDoc()
	for _, d := range app.docs {
		_ = writeBuffer(d)
	}
	persistSessionNow()
}

func resolveManualSave() {
	d := activeDoc()
	if d == nil {
		return
	}
	syncEditorToDoc()
	if d.DiskMissing {
		ans := message("This file was deleted outside Typora.\n\nRecreate it at its previous location?\n\nChoose No to keep the tab only; use Ctrl+Shift+S to save it somewhere else.", "File deleted outside Typora", MB_YESNO|MB_ICONWARNING)
		if ans != IDYES {
			app.saveStatus = "Deleted externally • not restored"
			invalidateStatus()
			return
		}
		queueDiskSave(d, true, true)
		app.saveStatus = "Restoring…"
		invalidateStatus()
		return
	}
	queueDiskSave(d, false, true)
	app.saveStatus = "Saving…"
	invalidate()
}

func ensureDefaultFolder() {
	if app.cfg.DefaultFolder == "" {
		app.cfg.DefaultFolder = defaultNotesFolder()
	}
	_ = os.MkdirAll(app.cfg.DefaultFolder, 0755)
}

func newDocument() {
	if !app.cfg.Onboarded {
		return
	}
	syncEditorToDoc()
	ensureDefaultFolder()
	title := nextUntitled(app.cfg.DefaultFolder)
	path := filepath.Join(app.cfg.DefaultFolder, title+".md")
	d := &Document{ID: randomID(), Title: title, Path: path, AutoCreated: true}
	d.BufferPath = bufferPath(d.ID)
	_ = writeBuffer(d)
	app.docs = append(app.docs, d)
	app.active = len(app.docs) - 1
	app.view = "edit"
	queueDiskSave(d, true, false)
	setEditorFromDoc()
	persistSessionNow()
	refreshSidebar()
	layout()
	beginRename()
	app.saveStatus = "Saved"
	invalidate()
}

func renameCurrent(title string) {
	d := activeDoc()
	if d == nil {
		return
	}
	title = cleanTitle(title)
	if title == "" {
		title = d.Title
		if title == "" {
			title = nextUntitled(app.cfg.DefaultFolder)
		}
	}
	folder := app.cfg.DefaultFolder
	if !d.AutoCreated && filepath.Dir(d.Path) != folder {
		return
	}
	title = uniqueTitle(folder, title, d.Path)
	newPath := filepath.Join(folder, title+".md")
	old := d.Path
	d.Title = title
	d.Path = newPath
	d.LastDiskMod = 0
	d.DiskMissing = false
	_ = writeBuffer(d)
	if !strings.EqualFold(filepath.Clean(old), filepath.Clean(newPath)) {
		queueRenameSave(d, old)
	} else {
		queueDiskSave(d, true, false)
	}
	persistSessionNow()
	refreshSidebar()
	updateWindowTitle()
	app.saveStatus = "Saving…"
	invalidate()
}

func beginRename() {
	d := activeDoc()
	if d == nil {
		return
	}
	r := tabRect(app.active)
	if app.renameEdit == 0 {
		app.renameEdit = createChild("EDIT", WS_CHILD|WS_TABSTOP|ES_AUTOHSCROLL, IDC_RENAME)
		send(app.renameEdit, WM_SETFONT, uintptr(app.fontUI), 1)
	}
	app.renameOriginal = d.Title
	app.renameActive = true
	setText(app.renameEdit, d.Title)
	procMoveWindow.Call(uintptr(app.renameEdit), uintptr(r.Left+12), uintptr(r.Top+8), uintptr(max32(70, r.Right-r.Left-40)), 26, 1)
	procShowWindow.Call(uintptr(app.renameEdit), SW_SHOW)
	send(app.renameEdit, EM_SETSEL, 0, ^uintptr(0))
	procSetFocus.Call(uintptr(app.renameEdit))
}
func commitRename() {
	if app.renameEdit == 0 || !app.renameActive {
		return
	}
	app.renameActive = false
	title := getText(app.renameEdit)
	procShowWindow.Call(uintptr(app.renameEdit), SW_HIDE)
	renameCurrent(title)
	procSetFocus.Call(uintptr(app.editor))
}
func cancelRename() {
	if app.renameEdit == 0 || !app.renameActive {
		return
	}
	app.renameActive = false
	procShowWindow.Call(uintptr(app.renameEdit), SW_HIDE)
	procSetFocus.Call(uintptr(app.editor))
	invalidate()
}

func switchTab(i int) {
	if i < 0 || i >= len(app.docs) || i == app.active {
		return
	}
	saveActive(false)
	app.active = i
	setEditorFromDoc()
	layout()
	invalidate()
}
func closeTab(i int) {
	if i < 0 || i >= len(app.docs) {
		return
	}
	syncEditorToDoc()
	d := app.docs[i]
	if app.cfg.ConfirmTabClose && strings.TrimSpace(d.Content) != "" {
		closeIt, neverAsk := confirmCloseTab(app.hwnd, d.Title)
		if neverAsk {
			app.cfg.ConfirmTabClose = false
			_ = saveConfig(app.cfg)
		}
		if !closeIt {
			return
		}
	}
	_ = writeBuffer(d)
	queueDiskSave(d, false, false)
	app.closed = append([]ClosedDoc{{SessionDoc: SessionDoc{ID: d.ID, Title: d.Title, Path: d.Path, BufferPath: d.BufferPath, AutoCreated: d.AutoCreated, LastDiskMod: d.LastDiskMod, DiskMissing: d.DiskMissing}, ClosedAt: time.Now().Unix()}}, app.closed...)
	if len(app.closed) > 20 {
		app.closed = app.closed[:20]
	}
	app.docs = append(app.docs[:i], app.docs[i+1:]...)
	if len(app.docs) == 0 {
		app.active = -1
		setEditorFromDoc()
		app.saveStatus = "Ready"
		persistSessionNow()
		layout()
		invalidate()
		return
	}
	if app.active >= len(app.docs) {
		app.active = len(app.docs) - 1
	} else if i < app.active {
		app.active--
	}
	setEditorFromDoc()
	persistSessionNow()
	layout()
	invalidate()
}
func reopenClosed() {
	if len(app.closed) == 0 {
		return
	}
	c := app.closed[0]
	app.closed = app.closed[1:]
	content, _ := os.ReadFile(c.BufferPath)
	if len(content) == 0 && c.Path != "" {
		content, _ = os.ReadFile(c.Path)
	}
	d := &Document{ID: c.ID, Title: c.Title, Path: c.Path, BufferPath: c.BufferPath, Content: string(content), AutoCreated: c.AutoCreated, LastDiskMod: fileModNano(c.Path), DiskMissing: c.DiskMissing || (c.LastDiskMod != 0 && fileModNano(c.Path) == 0)}
	app.docs = append(app.docs, d)
	app.active = len(app.docs) - 1
	setEditorFromDoc()
	persistSessionNow()
	layout()
	invalidate()
}

func removeOpenDocsForPath(path string) {
	path = filepath.Clean(path)
	oldActive := app.active
	newActive := oldActive
	activeRemoved := false
	kept := make([]*Document, 0, len(app.docs))
	for i, d := range app.docs {
		if d.Path != "" && strings.EqualFold(filepath.Clean(d.Path), path) {
			cancelDocIO(d.ID)
			_ = os.Remove(d.BufferPath)
			if i < oldActive {
				newActive--
			}
			if i == oldActive {
				activeRemoved = true
			}
			continue
		}
		kept = append(kept, d)
	}
	app.docs = kept

	filtered := app.closed[:0]
	for _, c := range app.closed {
		if c.Path == "" || !strings.EqualFold(filepath.Clean(c.Path), path) {
			filtered = append(filtered, c)
		}
	}
	app.closed = filtered
	_ = saveClosed(app.closed)
	if len(app.docs) == 0 {
		app.active = -1
		setEditorFromDoc()
		app.saveStatus = "Ready"
		persistSessionNow()
		layout()
		invalidate()
		return
	}
	if activeRemoved && newActive >= len(app.docs) {
		newActive = len(app.docs) - 1
	}
	if newActive < 0 {
		newActive = 0
	}
	app.active = newActive
	setEditorFromDoc()
	persistSessionNow()
	layout()
	invalidate()
}
