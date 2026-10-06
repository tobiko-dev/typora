//go:build windows

package main

import (
	"strings"
	"syscall"
	"time"

	"unsafe"
)

func installRichEditSubclasses() {
	if richEditSubclassCallback == 0 {
		richEditSubclassCallback = syscall.NewCallback(richEditSubclassProc)
	}
	if app.editor != 0 && oldEditorWndProc == 0 {
		r, _, _ := procSetWindowLongPtr.Call(uintptr(app.editor), ^uintptr(3), richEditSubclassCallback)
		oldEditorWndProc = r
	}
	if app.preview != 0 && oldPreviewWndProc == 0 {
		r, _, _ := procSetWindowLongPtr.Call(uintptr(app.preview), ^uintptr(3), richEditSubclassCallback)
		oldPreviewWndProc = r
	}
}

func rememberWindowPlacement() {
	if app.hwnd == 0 {
		return
	}
	wp := WINDOWPLACEMENT{Length: uint32(unsafe.Sizeof(WINDOWPLACEMENT{}))}
	r, _, _ := procGetWindowPlacement.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(&wp)))
	if r == 0 {
		return
	}
	rc := wp.RcNormalPosition
	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	if w < 520 || h < 360 {
		return
	}
	app.cfg.WindowX = rc.Left
	app.cfg.WindowY = rc.Top
	app.cfg.WindowW = w
	app.cfg.WindowH = h
	app.cfg.WindowMaximized = wp.ShowCmd == SW_SHOWMAXIMIZED
}

func startupWindowPlacement() (x, y, w, h int32, maximized, ok bool) {
	w, h = app.cfg.WindowW, app.cfg.WindowH
	x, y = app.cfg.WindowX, app.cfg.WindowY
	maximized = app.cfg.WindowMaximized
	if w < 520 || h < 360 {
		return 0, 0, 1360, 860, false, false
	}
	rc := RECT{x, y, x + w, y + h}
	hmon, _, _ := procMonitorFromRect.Call(uintptr(unsafe.Pointer(&rc)), 0)
	if hmon == 0 {
		return 0, 0, 1360, 860, false, false
	}
	mi := MONITORINFO{CbSize: uint32(unsafe.Sizeof(MONITORINFO{}))}
	if r, _, _ := procGetMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&mi))); r != 0 {
		work := mi.RcWork
		if w > work.Right-work.Left {
			w = work.Right - work.Left
		}
		if h > work.Bottom-work.Top {
			h = work.Bottom - work.Top
		}
		if x < work.Left {
			x = work.Left
		}
		if y < work.Top {
			y = work.Top
		}
		if x+w > work.Right {
			x = work.Right - w
		}
		if y+h > work.Bottom {
			y = work.Bottom - h
		}
	}
	return x, y, w, h, maximized, true
}

func layout() {
	if app.hwnd == 0 {
		return
	}
	var rc RECT
	procGetClientRect.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(&rc)))
	app.clientW = rc.Right
	app.clientH = rc.Bottom
	ensureActiveTabVisible()
	if app.onboarding || app.shortcutsVisible {
		procShowWindow.Call(uintptr(app.editor), SW_HIDE)
		procShowWindow.Call(uintptr(app.preview), SW_HIDE)
		if app.renameEdit != 0 {
			procShowWindow.Call(uintptr(app.renameEdit), SW_HIDE)
		}
		return
	}
	if len(app.docs) == 0 || activeDoc() == nil {
		procShowWindow.Call(uintptr(app.editor), SW_HIDE)
		procShowWindow.Call(uintptr(app.preview), SW_HIDE)
		if app.renameEdit != 0 {
			procShowWindow.Call(uintptr(app.renameEdit), SW_HIDE)
		}
		return
	}
	top := int32(TOP_H + TAB_H)
	bottom := app.clientH - STATUS_H
	side := int32(0)
	if app.cfg.Sidebar {
		side = SIDEBAR_W
	}
	x := side
	w := app.clientW - side
	h := bottom - top
	if h < 1 {
		h = 1
	}
	switch app.view {
	case "split":
		half := w / 2
		procMoveWindow.Call(uintptr(app.editor), uintptr(x), uintptr(top), uintptr(half), uintptr(h), 1)
		procMoveWindow.Call(uintptr(app.preview), uintptr(x+half), uintptr(top), uintptr(w-half), uintptr(h), 1)
		procShowWindow.Call(uintptr(app.editor), SW_SHOW)
		procShowWindow.Call(uintptr(app.preview), SW_SHOW)
	case "preview":
		procMoveWindow.Call(uintptr(app.preview), uintptr(x), uintptr(top), uintptr(w), uintptr(h), 1)
		procShowWindow.Call(uintptr(app.editor), SW_HIDE)
		procShowWindow.Call(uintptr(app.preview), SW_SHOW)
	default:
		procMoveWindow.Call(uintptr(app.editor), uintptr(x), uintptr(top), uintptr(w), uintptr(h), 1)
		procShowWindow.Call(uintptr(app.preview), SW_HIDE)
		procShowWindow.Call(uintptr(app.editor), SW_SHOW)
	}
	if app.renameEdit != 0 {
		procShowWindow.Call(uintptr(app.renameEdit), SW_HIDE)
	}
}

func drawText(hdc syscall.Handle, text string, r RECT, font syscall.Handle, color uint32, flags uintptr) {
	old, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(font))
	procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
	procSetTextColor.Call(uintptr(hdc), uintptr(color))
	u := syscall.StringToUTF16(text)
	procDrawText.Call(uintptr(hdc), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&r)), flags)
	procSelectObject.Call(uintptr(hdc), old)
}
func measureTextWidth(hdc syscall.Handle, text string, font syscall.Handle) int32 {
	if text == "" {
		return 0
	}
	old, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(font))
	u := syscall.StringToUTF16(text)
	var sz SIZE
	procGetTextExtentPoint32.Call(uintptr(hdc), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&sz)))
	procSelectObject.Call(uintptr(hdc), old)
	return sz.Cx
}
func fill(hdc syscall.Handle, r RECT, color uint32) {
	b, _, _ := procCreateSolidBrush.Call(uintptr(color))
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&r)), b)
	procDeleteObject.Call(b)
}
func roundFill(hdc syscall.Handle, r RECT, color uint32, radius int32) {
	b, _, _ := procCreateSolidBrush.Call(uintptr(color))
	oldB, _, _ := procSelectObject.Call(uintptr(hdc), b)
	pen, _, _ := procCreatePen.Call(PS_SOLID, 1, uintptr(color))
	oldP, _, _ := procSelectObject.Call(uintptr(hdc), pen)
	procRoundRect.Call(uintptr(hdc), uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius), uintptr(radius))
	procSelectObject.Call(uintptr(hdc), oldB)
	procSelectObject.Call(uintptr(hdc), oldP)
	procDeleteObject.Call(b)
	procDeleteObject.Call(pen)
}

func tooltipFor(key string) string {
	switch key {
	case "top:new", "plus":
		return "New note  Ctrl+N"
	case "top:open":
		return "Open file  Ctrl+O"
	case "top:folder":
		return "Change notes folder"
	case "top:saveas":
		return "Save As  Ctrl+Shift+S"
	case "top:edit":
		return "Edit only"
	case "top:split":
		return "Edit + preview"
	case "top:preview":
		return "Preview only"
	case "top:help":
		return "Tutorial"
	case "top:theme":
		return "Switch theme"
	}
	if strings.HasPrefix(key, "tabclose:") {
		return "Close tab  Ctrl+W"
	}
	return ""
}

func paintTooltip(hdc syscall.Handle) {
	if !app.tooltipVisible || app.onboarding || app.shortcutsVisible || app.tooltipTarget == "" {
		return
	}
	text := tooltipFor(app.tooltipTarget)
	if text == "" {
		return
	}
	w := int32(22 + len([]rune(text))*7)
	if w < 94 {
		w = 94
	}
	if w > 230 {
		w = 230
	}
	h := int32(28)
	x := app.mouseX + 10
	y := int32(TOP_H + 5)
	if app.mouseY >= TOP_H {
		y = TOP_H - h - 2
	}
	if x+w > app.clientW-8 {
		x = app.clientW - w - 8
	}
	if x < 8 {
		x = 8
	}
	if y < 4 {
		y = 4
	}
	r := RECT{x, y, x + w, y + h}
	bg := rgb(43, 49, 60)
	fg := rgb(245, 247, 250)
	if app.cfg.Theme == "light" {
		bg = rgb(42, 45, 52)
	}
	roundFill(hdc, r, bg, 7)
	drawText(hdc, text, RECT{r.Left + 9, r.Top, r.Right - 9, r.Bottom}, app.fontSmall, fg, DT_CENTER|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
}

func dailyQuote() string {
	quotes := []string{
		"A blank page is room, not pressure.",
		"Write the next useful sentence.",
		"Small notes become clear thinking.",
		"Capture it now; organize it later.",
		"Clarity often starts as a rough draft.",
		"A good note only needs to help future you.",
		"Start messy. Keep what matters.",
		"One line is enough to begin.",
		"Ideas get lighter once they are written down.",
		"Make a place for the thought before it disappears.",
		"The shortest useful note is still useful.",
		"Write first. Refine when it earns the time.",
		"A quiet page can hold a loud idea.",
		"Progress can be a sentence.",
		"Notes are checkpoints for your brain.",
		"Give the idea somewhere to live.",
		"Keep the useful part; delete the rest.",
		"Today’s scratch note can be tomorrow’s answer.",
		"You do not need a perfect structure to start.",
		"Good systems make forgetting harmless.",
		"Write what you would want to find later.",
		"A note is a save point for a thought.",
		"Make it easy to resume.",
		"The page is ready when you are.",
	}
	if len(quotes) == 0 {
		return ""
	}
	return quotes[(time.Now().YearDay()-1)%len(quotes)]
}

func emptyStateRects() (RECT, RECT, RECT) {
	left := int32(0)
	if app.cfg.Sidebar {
		left = SIDEBAR_W
	}
	top := int32(TOP_H + TAB_H)
	bottom := app.clientH - STATUS_H
	cx := left + (app.clientW-left)/2
	cy := top + (bottom-top)/2 - 18
	icon := RECT{cx - 24, cy - 92, cx + 24, cy - 44}
	newR := RECT{cx - 142, cy + 26, cx - 8, cy + 64}
	openR := RECT{cx + 8, cy + 26, cx + 142, cy + 64}
	return icon, newR, openR
}

func paintEmptyState(hdc syscall.Handle) {
	if len(app.docs) != 0 || app.onboarding || app.shortcutsVisible {
		return
	}
	p := colors()
	iconR, newR, openR := emptyStateRects()
	cx := (iconR.Left + iconR.Right) / 2
	cy := iconR.Bottom + 28

	if app.iconHeader != 0 {
		procDrawIconEx.Call(uintptr(hdc), uintptr(iconR.Left+8), uintptr(iconR.Top+8), uintptr(app.iconHeader), 32, 32, 0, 0, DI_NORMAL)
	} else {
		roundFill(hdc, RECT{cx - 17, iconR.Top + 7, cx + 17, iconR.Top + 41}, p.panel2, 9)
		drawText(hdc, "T", RECT{cx - 17, iconR.Top + 7, cx + 17, iconR.Top + 41}, app.fontUISemibold, p.text, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	drawText(hdc, "No notes open", RECT{cx - 220, cy - 6, cx + 220, cy + 24}, app.fontUISemibold, p.text, DT_CENTER|DT_VCENTER|DT_SINGLELINE)

	for _, b := range []struct {
		r     RECT
		key   string
		label string
	}{
		{newR, "empty:new", "+  New note"},
		{openR, "empty:open", "Open file"},
	} {
		bg := p.panel2
		if app.hover == b.key {
			bg = p.accentSoft
		}
		roundFill(hdc, b.r, bg, 9)
		drawText(hdc, b.label, b.r, app.fontUI, p.text, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	drawText(hdc, "Ctrl+N                                      Ctrl+O", RECT{newR.Left, newR.Bottom + 8, openR.Right, openR.Bottom + 30}, app.fontSmall, p.muted, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}
