//go:build windows

package main

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

func changeDefaultFolder() {
	p, err := pickFolder(app.hwnd, app.cfg.DefaultFolder)
	if err != nil || p == "" {
		return
	}
	app.cfg.DefaultFolder = p
	_ = os.MkdirAll(p, 0755)
	_ = saveConfig(app.cfg)
	startSidebarFolderWatcher(app.cfg.DefaultFolder)
	refreshSidebar()
	invalidate()
}

func saveAsCurrent() {
	d := activeDoc()
	if d == nil { return }
	syncEditorToDoc()
	initial := app.cfg.DefaultFolder
	if d.Path != "" { initial = filepath.Dir(d.Path) }
	name := d.Title + ".md"
	p, err := saveAsDialog(app.hwnd, initial, name)
	if err != nil { return }
	if filepath.Ext(p) == "" { p += ".md" }
	old := d.Path
	oldAuto := d.AutoCreated
	if err := atomicWrite(p, []byte(d.Content), 0644); err != nil {
		message(err.Error(), "Save As failed", MB_OK|MB_ICONWARNING)
		return
	}
	d.Path = p
	d.Title = strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	d.AutoCreated = false
	d.LastDiskMod = fileModNano(p)
	d.DiskMissing = false
	if oldAuto && old != "" && !strings.EqualFold(old, p) { _ = os.Remove(old) }
	_ = writeBuffer(d)
	persistSessionNow()
	refreshSidebar()
	updateWindowTitle()
	app.saveStatus = "Saved as " + filepath.Base(p)
	invalidate()
}

func restoreSession() {
	ensureDefaultFolder()
	s := loadSession()
	for _, sd := range s.Docs {
		var content []byte
		if sd.BufferPath != "" { content, _ = os.ReadFile(sd.BufferPath) }
		if content == nil && sd.Path != "" { content, _ = os.ReadFile(sd.Path) }
		title := sd.Title
		if title == "" && sd.Path != "" { title = strings.TrimSuffix(filepath.Base(sd.Path), filepath.Ext(sd.Path)) }
		if title == "" { title = nextUntitled(app.cfg.DefaultFolder) }
		id := sd.ID
		if id == "" { id = randomID() }
		bp := sd.BufferPath
		if bp == "" { bp = bufferPath(id) }
		diskMod := fileModNano(sd.Path)
		lastMod := sd.LastDiskMod
		if lastMod == 0 { lastMod = diskMod }
		d := &Document{ID: id, Title: title, Path: sd.Path, BufferPath: bp, Content: string(content), AutoCreated: sd.AutoCreated, LastDiskMod: lastMod, DiskMissing: sd.DiskMissing || (sd.Path != "" && sd.LastDiskMod != 0 && diskMod == 0)}
		app.docs = append(app.docs, d)
	}
	if len(app.docs) == 0 {
		app.active = -1
		setEditorFromDoc()
		app.saveStatus = "Ready"
		persistSessionNow()
		layout()
		invalidate()
		return
	}
	app.active = s.Active
	if app.active < 0 || app.active >= len(app.docs) { app.active = 0 }
	setEditorFromDoc()
	persistSessionNow()
}

func applySidebarOrder(files, order []string) []string {
	if len(files) == 0 { return nil }
	if len(order) == 0 { return append([]string(nil), files...) }
	byLower := make(map[string]string, len(files))
	for _, f := range files { byLower[strings.ToLower(f)] = f }
	out := make([]string, 0, len(files))
	used := make(map[string]bool, len(files))
	for _, wanted := range order {
		k := strings.ToLower(wanted)
		if f, ok := byLower[k]; ok && !used[k] {
			out = append(out, f)
			used[k] = true
		}
	}
	for _, f := range files {
		k := strings.ToLower(f)
		if !used[k] { out = append(out, f) }
	}
	return out
}

func rememberSidebarOrder() {
	app.cfg.SidebarOrder = append([]string(nil), app.sidebarFiles...)
	queueConfigPersist(app.cfg)
}

func sortSidebarAlphabetical() {
	sort.SliceStable(app.sidebarFiles, func(i, j int) bool {
		return strings.ToLower(app.sidebarFiles[i]) < strings.ToLower(app.sidebarFiles[j])
	})
	app.cfg.SidebarOrder = nil
	queueConfigPersist(app.cfg)
	app.sidebarScroll = 0
	invalidateSidebar()
}

func refreshSidebar() {
	ensureDefaultFolder()
	queueSidebarRefresh(app.cfg.DefaultFolder)
}

func renderPreview() {
	if app.view == "edit" || app.preview == 0 { return }
	d := activeDoc()
	if d == nil { return }
	renderNativePreview(app.preview, d.Content)
}

func updateWindowTitle() {
	title := appName
	if d := activeDoc(); d != nil { title = d.Title + " — " + appName }
	setText(app.hwnd, title)
}

func setView(v string) {
	if v != "edit" && v != "split" && v != "preview" { return }
	log.Printf("View change requested: %s -> %s", app.view, v)
	app.view = v
	layout()
	if v != "edit" {
		syncEditorToDoc()
		log.Printf("Preview render before persisting view: begin mode=%s", v)
		renderPreview()
		log.Printf("Preview render before persisting view: complete mode=%s", v)
	}
	app.cfg.View = v
	_ = saveConfig(app.cfg)
	invalidate()
	if v != "preview" { procSetFocus.Call(uintptr(app.editor)) }
}

func openTutorial() {
	if app.renameActive { commitRename() }
	if app.cfg.Onboarded {
		syncEditorToDoc()
		app.tutorialRevisit = true
	} else { app.tutorialRevisit = false }
	app.tutorialStep = 0
	app.onboarding = true
	layout()
	invalidate()
}

func closeTutorial() {
	if !app.tutorialRevisit { return }
	app.onboarding = false
	app.tutorialRevisit = false
	layout()
	invalidate()
	if app.editor != 0 && app.view != "preview" { procSetFocus.Call(uintptr(app.editor)) }
}

func normalizePreviewTarget(raw string) string {
	t := strings.TrimSpace(strings.Trim(raw, "<>"))
	if t == "" || strings.HasPrefix(t, "#") { return "" }
	lower := strings.ToLower(t)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "mailto:") { return t }
	if d := activeDoc(); d != nil && d.Path != "" {
		candidate := t
		if !filepath.IsAbs(candidate) { candidate = filepath.Join(filepath.Dir(d.Path), filepath.FromSlash(candidate)) }
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() { return candidate }
	}
	if !strings.ContainsAny(t, " 	
") && strings.Contains(t, ".") && !strings.Contains(t, "://") { return "https://" + t }
	return ""
}

func openPreviewTarget(raw string) {
	target := normalizePreviewTarget(raw)
	if target == "" { return }
	if !strings.Contains(target, "://") && !strings.HasPrefix(strings.ToLower(target), "mailto:") {
		if strings.EqualFold(filepath.Ext(target), ".md") { openFile(target); return }
	}
	r, _, _ := procShellExecuteW.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(wstr("open"))), uintptr(unsafe.Pointer(wstr(target))), 0, 0, SW_SHOWNORMAL)
	if r <= 32 { app.saveStatus = "Could not open link"; invalidateStatus() }
}

func openPreviewLinkAt(start, end int32) {
	for _, l := range app.previewLinks {
		if start < int32(l.end) && end > int32(l.start) { openPreviewTarget(l.target); return }
	}
}

func toggleTheme() {
	if app.cfg.Theme == "light" { app.cfg.Theme = "dark" } else { app.cfg.Theme = "light" }
	_ = saveConfig(app.cfg)
	setEditorStyle()
	renderPreview()
	invalidate()
}

func wordCount(s string) int { return len(strings.Fields(s)) }

func tabStripLeft() int32 {
	if app.cfg.Sidebar { return SIDEBAR_W + 12 }
	return 12
}

func tabStripRight() int32 {
	r := app.clientW - 58
	minR := tabStripLeft() + 80
	if r < minR { return minR }
	return r
}

func tabWidth() int32 {
	n := len(app.docs)
	avail := tabStripRight() - tabStripLeft()
	if avail < 108 { return 108 }
	w := int32(176)
	if n > 0 && int32(n)*w > avail { w = avail / int32(n) }
	if w < 116 { w = 116 }
	if w > 176 { w = 176 }
	return w
}

func maxTabScroll() int32 {
	content := int32(len(app.docs)) * tabWidth()
	avail := tabStripRight() - tabStripLeft()
	if content <= avail { return 0 }
	return content - avail
}

func ensureActiveTabVisible() {
	if app.active < 0 || app.active >= len(app.docs) { app.tabScroll = 0; return }
	maxScroll := maxTabScroll()
	if maxScroll <= 0 { app.tabScroll = 0; return }
	if app.tabScroll < 0 { app.tabScroll = 0 }
	if app.tabScroll > maxScroll { app.tabScroll = maxScroll }
	w := tabWidth()
	left := tabStripLeft()
	right := tabStripRight()
	r := RECT{left + int32(app.active)*w - app.tabScroll, TOP_H, left + int32(app.active+1)*w - app.tabScroll - 4, TOP_H + TAB_H}
	if r.Left < left { app.tabScroll -= left - r.Left } else if r.Right > right { app.tabScroll += r.Right - right }
	if app.tabScroll < 0 { app.tabScroll = 0 }
	if app.tabScroll > maxScroll { app.tabScroll = maxScroll }
}

func tabRect(i int) RECT {
	w := tabWidth()
	x := tabStripLeft() + int32(i)*w - app.tabScroll
	return RECT{x, TOP_H, x + w - 4, TOP_H + TAB_H}
}

func plusRect() RECT { return RECT{app.clientW - 48, TOP_H + 7, app.clientW - 14, TOP_H + 35} }

func contains(r RECT, x, y int32) bool { return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom }

func max32(a, b int32) int32 { if a > b { return a }; return b }

func max(a, b int) int { if a > b { return a }; return b }

func shortcutCardRect() RECT {
	w := int32(760)
	h := int32(540)
	if app.clientW < w+48 { w = max32(520, app.clientW-48) }
	if app.clientH < h+48 { h = max32(420, app.clientH-48) }
	x := (app.clientW - w) / 2
	y := (app.clientH - h) / 2
	return RECT{x, y, x + w, y + h}
}

func shortcutCloseRect() RECT {
	r := shortcutCardRect()
	return RECT{r.Right - 54, r.Top + 18, r.Right - 18, r.Top + 54}
}

func openShortcuts() {
	if app.shortcutsVisible { return }
	if app.renameActive { cancelRename() }
	if !app.onboarding { syncEditorToDoc() }
	app.shortcutsVisible = true
	app.tooltipVisible = false
	app.hover = ""
	layout()
	invalidate()
}

func closeShortcuts() {
	if !app.shortcutsVisible { return }
	app.shortcutsVisible = false
	layout()
	invalidate()
	if !app.onboarding {
		if app.view == "preview" && app.preview != 0 { procSetFocus.Call(uintptr(app.preview)) } else if app.editor != 0 { procSetFocus.Call(uintptr(app.editor)) }
	}
}

func paintShortcuts(hdc syscall.Handle) {
	paintNormal(hdc)
	p := colors()
	r := shortcutCardRect()
	roundFill(hdc, r, p.panel, 20)
	penColor := p.line
	fill(hdc, RECT{r.Left, r.Top, r.Right, r.Top + 1}, penColor)
	fill(hdc, RECT{r.Left, r.Bottom - 1, r.Right, r.Bottom}, penColor)
	fill(hdc, RECT{r.Left, r.Top, r.Left + 1, r.Bottom}, penColor)
	fill(hdc, RECT{r.Right - 1, r.Top, r.Right, r.Bottom}, penColor)

	drawText(hdc, "Keyboard shortcuts", RECT{r.Left + 34, r.Top + 22, r.Right - 90, r.Top + 58}, app.fontUISemibold, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	drawText(hdc, "F1 or Esc to close", RECT{r.Left + 34, r.Top + 58, r.Right - 90, r.Top + 82}, app.fontSmall, p.muted, DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	cr := shortcutCloseRect()
	if app.hover == "shortcuts:close" { roundFill(hdc, cr, p.accentSoft, 9) }
	drawText(hdc, "×", cr, app.fontUISemibold, p.text, DT_CENTER|DT_VCENTER|DT_SINGLELINE)

	leftX := r.Left + 38
	rightX := r.Left + (r.Right-r.Left)/2 + 14
	top := r.Top + 106
	rowH := int32(34)
	drawText(hdc, "FILES & TABS", RECT{leftX, top, rightX - 20, top + 24}, app.fontSmall, p.muted, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	leftRows := [][2]string{{"Ctrl+N", "New note"}, {"Ctrl+O", "Open file"}, {"Ctrl+S", "Save now"}, {"Ctrl+Shift+S", "Save As…"}, {"Ctrl+W", "Close tab"}, {"Ctrl+Tab", "Next tab"}, {"Ctrl+Shift+Tab", "Previous tab"}, {"Ctrl+Shift+T", "Reopen closed tab"}}
	y := top + 30
	for _, row := range leftRows {
		drawText(hdc, row[0], RECT{leftX, y, leftX + 132, y + rowH}, app.fontMono, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		drawText(hdc, row[1], RECT{leftX + 146, y, rightX - 20, y + rowH}, app.fontUI, p.muted, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		y += rowH
	}
	drawText(hdc, "EDITING & MOUSE", RECT{rightX, top, r.Right - 38, top + 24}, app.fontSmall, p.muted, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	rightRows := [][2]string{{"F2", "Rename current note"}, {"Enter", "Finish renaming"}, {"Esc", "Cancel rename / close sheet"}, {"F1", "Keyboard shortcuts"}, {"Middle click", "Close a tab"}, {"Drag tab", "Rearrange tabs"}, {"Right click", "Tab / note actions"}, {"Mouse wheel", "Scroll long documents"}}
	y = top + 30
	for _, row := range rightRows {
		drawText(hdc, row[0], RECT{rightX, y, rightX + 130, y + rowH}, app.fontMono, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		drawText(hdc, row[1], RECT{rightX + 144, y, r.Right - 38, y + rowH}, app.fontUI, p.muted, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		y += rowH
	}
	drawText(hdc, "Typora v"+appVersion+"  •  native  •  local", RECT{r.Left + 38, r.Bottom - 34, r.Right - 38, r.Bottom - 12}, app.fontSmall, p.muted, DT_RIGHT|DT_VCENTER|DT_SINGLELINE)
}
