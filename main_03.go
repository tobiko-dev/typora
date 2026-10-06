//go:build windows

package main

import (
	"fmt"

	"os"
	"path/filepath"

	"strings"
	"syscall"

	"unsafe"
)

func deleteNotePath(path string) {
	if path == "" {
		return
	}
	path = filepath.Clean(path)
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if !confirmDeleteNote(app.hwnd, title) {
		return
	}

	for _, d := range app.docs {
		if d.Path != "" && strings.EqualFold(filepath.Clean(d.Path), path) {
			cancelDocIO(d.ID)
		}
	}
	if err := recycleFile(app.hwnd, path); err != nil {
		if err.Error() != "delete cancelled" {
			message(err.Error(), "Could not delete note", MB_OK|MB_ICONWARNING)
		}
		return
	}
	removeOpenDocsForPath(path)
	refreshSidebar()
}

func contextMenuCommand(items []struct {
	id    uintptr
	label string
}, x, y int32) uintptr {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return 0
	}
	defer procDestroyMenu.Call(menu)
	for _, item := range items {
		if item.id == 0 {
			procAppendMenu.Call(menu, MF_SEPARATOR, 0, 0)
			continue
		}
		procAppendMenu.Call(menu, MF_STRING, item.id, uintptr(unsafe.Pointer(wstr(item.label))))
	}
	pt := POINT{X: x, Y: y}
	procClientToScreen.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(&pt)))
	cmd, _, _ := procTrackPopupMenu.Call(menu, TPM_RIGHTBUTTON|TPM_RETURNCMD, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(app.hwnd), 0)
	return cmd
}

type contextEntry struct {
	Label     string
	Action    string
	Separator bool
	Danger    bool
}

func contextEntries() []contextEntry {
	switch app.contextKind {
	case "tab":
		return []contextEntry{
			{Label: "Rename", Action: "rename"},
			{Label: "Close tab", Action: "close"},
			{Separator: true},
			{Label: "Open in Explorer", Action: "explorer"},
			{Label: "Delete note…", Action: "delete", Danger: true},
		}
	case "sidebar":
		return []contextEntry{
			{Label: "Open", Action: "open"},
			{Label: "Open in Explorer", Action: "explorer"},
			{Separator: true},
			{Label: "Sort notes A–Z", Action: "sortaz"},
			{Separator: true},
			{Label: "Delete note…", Action: "delete", Danger: true},
		}
	case "sidebar-root":
		return []contextEntry{
			{Label: "Open folder in Explorer", Action: "explorer"},
			{Label: "Sort notes A–Z", Action: "sortaz"},
		}
	}
	return nil
}

func contextMenuSize() (int32, int32) {
	h := int32(16)
	for _, e := range contextEntries() {
		if e.Separator {
			h += 9
		} else {
			h += 32
		}
	}
	return 206, h
}

func contextMenuRect() RECT {
	w, h := contextMenuSize()
	x, y := app.contextX, app.contextY
	if x+w > app.clientW-8 {
		x = app.clientW - w - 8
	}
	if y+h > app.clientH-STATUS_H-8 {
		y = app.clientH - STATUS_H - h - 8
	}
	if x < 8 {
		x = 8
	}
	if y < TOP_H+4 {
		y = TOP_H + 4
	}
	return RECT{x, y, x + w, y + h}
}

func contextItemRect(index int) (RECT, bool) {
	r := contextMenuRect()
	y := r.Top + 8
	for i, e := range contextEntries() {
		if e.Separator {
			y += 9
			continue
		}
		rr := RECT{r.Left + 6, y, r.Right - 6, y + 32}
		if i == index {
			return rr, true
		}
		y += 32
	}
	return RECT{}, false
}

func contextHit(x, y int32) int {
	if !app.contextVisible || !contains(contextMenuRect(), x, y) {
		return -1
	}
	for i, e := range contextEntries() {
		if e.Separator {
			continue
		}
		if r, ok := contextItemRect(i); ok && contains(r, x, y) {
			return i
		}
	}
	return -1
}

func openInExplorer(path string) {
	if path == "" {
		path = app.cfg.DefaultFolder
	}
	path = filepath.Clean(path)
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		procShellExecuteW.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(wstr("open"))), uintptr(unsafe.Pointer(wstr("explorer.exe"))), uintptr(unsafe.Pointer(wstr("\""+path+"\""))), 0, SW_SHOWNORMAL)
		return
	}
	params := "/select,\"" + path + "\""
	procShellExecuteW.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(wstr("open"))), uintptr(unsafe.Pointer(wstr("explorer.exe"))), uintptr(unsafe.Pointer(wstr(params))), 0, SW_SHOWNORMAL)
}

func closeContextMenu() {
	if app.contextHwnd != 0 {
		h := app.contextHwnd
		app.contextHwnd = 0
		procDestroyWindow.Call(uintptr(h))
	}
	app.contextVisible = false
	app.contextKind = ""
	app.contextPath = ""
	app.contextTab = -1
	app.contextHover = -1
}

func executeContext(index int) {
	entries := contextEntries()
	if index < 0 || index >= len(entries) || entries[index].Separator {
		closeContextMenu()
		return
	}
	action := entries[index].Action
	kind := app.contextKind
	path := app.contextPath
	tab := app.contextTab
	if app.contextHwnd != 0 {
		h := app.contextHwnd
		app.contextHwnd = 0
		procDestroyWindow.Call(uintptr(h))
	}
	app.contextVisible = false
	app.contextHover = -1
	switch action {
	case "rename":
		if kind == "tab" && tab >= 0 && tab < len(app.docs) {
			switchTab(tab)
			beginRename()
		}
	case "close":
		if kind == "tab" && tab >= 0 && tab < len(app.docs) {
			closeTab(tab)
		}
	case "open":
		if path != "" {
			openFile(path)
		}
	case "explorer":
		openInExplorer(path)
	case "sortaz":
		sortSidebarAlphabetical()
	case "delete":
		if path != "" {
			deleteNotePath(path)
		}
	}
	invalidate()
}

func paintContextMenu(hdc syscall.Handle) {
	if !app.contextVisible || app.onboarding || app.shortcutsVisible {
		return
	}
	p := colors()
	r := contextMenuRect()
	shadow := RECT{r.Left + 3, r.Top + 4, r.Right + 3, r.Bottom + 4}
	shadowColor := rgb(8, 10, 14)
	if app.cfg.Theme == "light" {
		shadowColor = rgb(205, 208, 216)
	}
	roundFill(hdc, shadow, shadowColor, 9)
	roundFill(hdc, r, p.panel2, 9)

	frame := r
	frame.Left++
	frame.Top++
	frame.Right--
	frame.Bottom--

	fill(hdc, RECT{frame.Left + 7, frame.Top, frame.Right - 7, frame.Top + 1}, p.line)
	fill(hdc, RECT{frame.Left + 7, frame.Bottom - 1, frame.Right - 7, frame.Bottom}, p.line)
	y := r.Top + 8
	for i, e := range contextEntries() {
		if e.Separator {
			fill(hdc, RECT{r.Left + 12, y + 4, r.Right - 12, y + 5}, p.line)
			y += 9
			continue
		}
		rr := RECT{r.Left + 6, y, r.Right - 6, y + 32}
		if app.hover == fmt.Sprintf("context:%d", i) {
			hoverColor := rgb(54, 64, 82)
			if app.cfg.Theme == "light" {
				hoverColor = rgb(224, 231, 247)
			}
			roundFill(hdc, rr, hoverColor, 7)
		}
		fg := p.text
		if e.Danger {
			fg = p.danger
		}
		drawText(hdc, e.Label, RECT{rr.Left + 12, rr.Top, rr.Right - 10, rr.Bottom}, app.fontUI, fg, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		y += 32
	}
}

func contextPopupItemRect(index int) (RECT, bool) {
	w, _ := contextMenuSize()
	y := int32(8)
	for i, e := range contextEntries() {
		if e.Separator {
			y += 9
			continue
		}
		rr := RECT{6, y, w - 6, y + 32}
		if i == index {
			return rr, true
		}
		y += 32
	}
	return RECT{}, false
}

func contextPopupHit(x, y int32) int {
	w, h := contextMenuSize()
	if x < 0 || y < 0 || x >= w || y >= h {
		return -1
	}
	for i, e := range contextEntries() {
		if e.Separator {
			continue
		}
		if r, ok := contextPopupItemRect(i); ok && contains(r, x, y) {
			return i
		}
	}
	return -1
}

func paintContextPopup(hwnd syscall.Handle) {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
	p := colors()
	w, h := contextMenuSize()
	fill(syscall.Handle(hdc), RECT{0, 0, w, h}, p.panel2)

	fill(syscall.Handle(hdc), RECT{0, 0, w, 1}, p.line)
	fill(syscall.Handle(hdc), RECT{0, h - 1, w, h}, p.line)
	fill(syscall.Handle(hdc), RECT{0, 0, 1, h}, p.line)
	fill(syscall.Handle(hdc), RECT{w - 1, 0, w, h}, p.line)
	y := int32(8)
	for i, e := range contextEntries() {
		if e.Separator {
			fill(syscall.Handle(hdc), RECT{12, y + 4, w - 12, y + 5}, p.line)
			y += 9
			continue
		}
		rr := RECT{6, y, w - 6, y + 32}
		if app.contextHover == i {
			hoverColor := rgb(58, 70, 92)
			if app.cfg.Theme == "light" {
				hoverColor = rgb(218, 227, 247)
			}
			roundFill(syscall.Handle(hdc), rr, hoverColor, 7)
		}
		fg := p.text
		if e.Danger {
			fg = p.danger
		}
		drawText(syscall.Handle(hdc), e.Label, RECT{rr.Left + 12, rr.Top, rr.Right - 10, rr.Bottom}, app.fontUI, fg, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		y += 32
	}
}

func contextMenuWndProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case WM_PAINT:
		paintContextPopup(hwnd)
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_MOUSEMOVE:
		x, y := signedWord(lparam), int32(int16((lparam>>16)&0xffff))
		next := contextPopupHit(x, y)
		if next != app.contextHover {
			app.contextHover = next
			procInvalidateRect.Call(uintptr(hwnd), 0, 0)
		}
		tme := TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), DwFlags: TME_LEAVE, HwndTrack: hwnd}
		procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
		return 0
	case WM_MOUSELEAVE:
		if app.contextHover != -1 {
			app.contextHover = -1
			procInvalidateRect.Call(uintptr(hwnd), 0, 0)
		}
		return 0
	case WM_LBUTTONUP:
		x, y := signedWord(lparam), int32(int16((lparam>>16)&0xffff))
		if i := contextPopupHit(x, y); i >= 0 {
			executeContext(i)
		} else {
			closeContextMenu()
		}
		return 0
	case WM_KILLFOCUS:
		if app.contextHwnd == hwnd {
			procDestroyWindow.Call(uintptr(hwnd))
		}
		return 0
	case WM_DESTROY:
		if app.contextHwnd == hwnd {
			app.contextHwnd = 0
			app.contextVisible = false
			app.contextHover = -1
			app.contextKind = ""
			app.contextPath = ""
			app.contextTab = -1
		}
		return 0
	}
	r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func showContextPopup(x, y int32) {
	if app.contextHwnd != 0 {
		procDestroyWindow.Call(uintptr(app.contextHwnd))
		app.contextHwnd = 0
	}
	w, h := contextMenuSize()
	pt := POINT{X: x, Y: y}
	procClientToScreen.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(&pt)))
	rc := RECT{pt.X, pt.Y, pt.X + w, pt.Y + h}
	hmon, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&rc)), 2)
	if hmon != 0 {
		mi := MONITORINFO{CbSize: uint32(unsafe.Sizeof(MONITORINFO{}))}
		if r, _, _ := procGetMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&mi))); r != 0 {
			if pt.X+w > mi.RcWork.Right {
				pt.X = mi.RcWork.Right - w
			}
			if pt.Y+h > mi.RcWork.Bottom {
				pt.Y = mi.RcWork.Bottom - h
			}
			if pt.X < mi.RcWork.Left {
				pt.X = mi.RcWork.Left
			}
			if pt.Y < mi.RcWork.Top {
				pt.Y = mi.RcWork.Top
			}
		}
	}
	hInst, _, _ := procGetModuleHandle.Call(0)
	className := wstr("TyporaContextMenuV413")
	hwnd, _, _ := procCreateWindowEx.Call(WS_EX_TOOLWINDOW, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(wstr(""))), WS_POPUP|WS_VISIBLE, uintptr(int64(pt.X)), uintptr(int64(pt.Y)), uintptr(w), uintptr(h), uintptr(app.hwnd), 0, hInst, 0)
	if hwnd == 0 {
		nativeItems := make([]struct {
			id    uintptr
			label string
		}, 0, len(contextEntries()))
		for i, e := range contextEntries() {
			if e.Separator {
				nativeItems = append(nativeItems, struct {
					id    uintptr
					label string
				}{0, ""})
			} else {
				nativeItems = append(nativeItems, struct {
					id    uintptr
					label string
				}{uintptr(i + 1), e.Label})
			}
		}
		if cmd := contextMenuCommand(nativeItems, x, y); cmd > 0 {
			executeContext(int(cmd - 1))
		} else {
			app.contextVisible = false
			app.contextKind = ""
			app.contextPath = ""
			app.contextTab = -1
		}
		return
	}
	app.contextHwnd = syscall.Handle(hwnd)
	app.contextHover = -1
	procSetFocus.Call(hwnd)
	procUpdateWindow.Call(hwnd)
}

func handleRightClick(x, y int32) {
	procKillTimer.Call(uintptr(app.hwnd), TIMER_TOOLTIP)
	app.tooltipVisible = false
	app.tooltipTarget = ""
	if app.onboarding || app.shortcutsVisible {
		return
	}
	app.contextVisible = false
	app.contextHover = -1
	if x >= tabStripLeft() && x < tabStripRight() && y >= TOP_H && y < TOP_H+TAB_H {
		for i := range app.docs {
			r := tabRect(i)
			if r.Right <= tabStripLeft() || r.Left >= tabStripRight() || !contains(r, x, y) {
				continue
			}
			switchTab(i)
			app.contextVisible = true
			app.contextKind = "tab"
			app.contextTab = i
			app.contextPath = app.docs[i].Path
			app.contextX, app.contextY = x, y
			showContextPopup(x, y)
			return
		}
	}
	if app.cfg.Sidebar && x < SIDEBAR_W && y > TOP_H && y < app.clientH-STATUS_H {
		idx := -1
		if y > TOP_H+70 {
			idx = app.sidebarScroll + int((y-(TOP_H+70))/32)
		}
		app.contextVisible = true
		app.contextTab = -1
		if idx >= 0 && idx < len(app.sidebarFiles) {
			app.contextKind = "sidebar"
			app.contextPath = filepath.Join(app.cfg.DefaultFolder, app.sidebarFiles[idx])
		} else {
			app.contextKind = "sidebar-root"
			app.contextPath = app.cfg.DefaultFolder
		}
		app.contextX, app.contextY = x, y
		showContextPopup(x, y)
	}
}

func openFile(path string) {
	path = filepath.Clean(path)
	for i, d := range app.docs {
		if strings.EqualFold(filepath.Clean(d.Path), path) {
			switchTab(i)
			return
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		message(err.Error(), "Could not open file", MB_OK|MB_ICONWARNING)
		return
	}
	syncEditorToDoc()
	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	d := &Document{ID: randomID(), Title: title, Path: path, Content: string(b), AutoCreated: false, LastDiskMod: fileModNano(path)}
	d.BufferPath = bufferPath(d.ID)
	_ = writeBuffer(d)
	app.docs = append(app.docs, d)
	app.active = len(app.docs) - 1
	setEditorFromDoc()
	persistSessionNow()
	layout()
	invalidate()
}
func openDialog() {
	initial := app.cfg.DefaultFolder
	if d := activeDoc(); d != nil && d.Path != "" {
		initial = filepath.Dir(d.Path)
	}
	if p, err := pickMarkdownFile(app.hwnd, initial); err == nil {
		openFile(p)
	}
}
