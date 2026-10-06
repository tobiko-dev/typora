//go:build windows

package main

import (
	"fmt"

	"path/filepath"

	"strings"
	"syscall"

	"unsafe"
)

func paintNormal(hdc syscall.Handle) {
	p := colors()
	fill(hdc, RECT{0, 0, app.clientW, app.clientH}, p.bg)
	fill(hdc, RECT{0, 0, app.clientW, TOP_H}, p.panel)
	fill(hdc, RECT{0, TOP_H, app.clientW, TOP_H + TAB_H}, p.panel2)
	fill(hdc, RECT{0, app.clientH - STATUS_H, app.clientW, app.clientH}, p.panel)
	if app.iconHeader != 0 {
		procDrawIconEx.Call(uintptr(hdc), 22, 15, uintptr(app.iconHeader), 24, 24, 0, 0, DI_NORMAL)
	} else {
		drawText(hdc, "T", RECT{18, 11, 48, 43}, app.fontUISemibold, p.text, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	drawText(hdc, "Typora", RECT{58, 10, 145, 36}, app.fontUISemibold, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE)

	drawText(hdc, "native • local • v"+strings.TrimSuffix(appVersion, ".0"), RECT{58, 30, 205, 48}, app.fontSmall, p.muted, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	buttons := []struct {
		name, label string
		r           RECT
	}{{"new", "New", RECT{210, 11, 262, 43}}, {"open", "Open", RECT{268, 11, 326, 43}}, {"folder", "Folder", RECT{332, 11, 400, 43}}, {"saveas", "Save As", RECT{406, 11, 484, 43}}, {"edit", "Edit", RECT{app.clientW - 324, 11, app.clientW - 270, 43}}, {"split", "Split", RECT{app.clientW - 266, 11, app.clientW - 208, 43}}, {"preview", "Preview", RECT{app.clientW - 204, 11, app.clientW - 130, 43}}, {"help", "?", RECT{app.clientW - 118, 11, app.clientW - 82, 43}}, {"theme", "◐", RECT{app.clientW - 70, 11, app.clientW - 30, 43}}}
	for _, b := range buttons {
		sel := (b.name == "edit" && app.view == "edit") || (b.name == "split" && app.view == "split") || (b.name == "preview" && app.view == "preview")
		hovered := app.hover == "top:"+b.name
		if sel {
			roundFill(hdc, b.r, p.accentSoft, 10)
		} else if hovered {
			roundFill(hdc, b.r, p.panel2, 10)
		}
		drawText(hdc, b.label, b.r, app.fontUI, func() uint32 {
			if sel || hovered {
				return p.text
			}
			return p.muted
		}(), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	if app.cfg.Sidebar {
		fill(hdc, RECT{0, TOP_H, SIDEBAR_W, app.clientH - STATUS_H}, p.panel)
		drawText(hdc, "NOTES", RECT{18, TOP_H + 13, SIDEBAR_W - 18, TOP_H + 34}, app.fontSmall, p.muted, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		folder := filepath.Base(app.cfg.DefaultFolder)
		drawText(hdc, folder, RECT{18, TOP_H + 34, SIDEBAR_W - 18, TOP_H + 59}, app.fontUI, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		y := int32(TOP_H + 70)
		start := app.sidebarScroll
		for i := start; i < len(app.sidebarFiles) && y < app.clientH-STATUS_H-24; i++ {
			name := app.sidebarFiles[i]
			rr := RECT{12, y, SIDEBAR_W - 10, y + 30}
			if app.sidebarDragging && i == app.sidebarDrag {
				roundFill(hdc, rr, p.line, 8)
				roundFill(hdc, RECT{rr.Left + 1, rr.Top + 1, rr.Right - 1, rr.Bottom - 1}, p.panel, 7)
				y += 32
				continue
			}
			active := false
			if d := activeDoc(); d != nil && strings.EqualFold(filepath.Base(d.Path), name) {
				active = true
				roundFill(hdc, rr, p.accentSoft, 8)
			} else if app.hover == fmt.Sprintf("sidebar:%d", i) {
				hoverColor := rgb(48, 56, 70)
				if app.cfg.Theme == "light" {
					hoverColor = rgb(233, 237, 246)
				}
				roundFill(hdc, rr, hoverColor, 8)
			}
			_ = active
			drawText(hdc, name, RECT{20, y, SIDEBAR_W - 16, y + 30}, app.fontUI, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
			y += 32
		}
		if app.sidebarDragging && app.sidebarDrag >= 0 && app.sidebarDrag < len(app.sidebarFiles) {
			gy := app.sidebarDragVisualY - 15
			if gy < TOP_H+70 {
				gy = TOP_H + 70
			}
			if gy+30 > app.clientH-STATUS_H-4 {
				gy = app.clientH - STATUS_H - 34
			}
			gr := RECT{12, gy, SIDEBAR_W - 10, gy + 30}
			shadow := rgb(8, 10, 14)
			if app.cfg.Theme == "light" {
				shadow = rgb(205, 210, 222)
			}
			roundFill(hdc, RECT{gr.Left + 2, gr.Top + 3, gr.Right + 2, gr.Bottom + 3}, shadow, 8)
			roundFill(hdc, gr, p.accentSoft, 8)
			drawText(hdc, app.sidebarFiles[app.sidebarDrag], RECT{gr.Left + 10, gr.Top, gr.Right - 10, gr.Bottom}, app.fontUI, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		}
	}

	savedDC, _, _ := procSaveDC.Call(uintptr(hdc))
	if savedDC != 0 {
		procIntersectClipRect.Call(uintptr(hdc), uintptr(tabStripLeft()), uintptr(TOP_H), uintptr(tabStripRight()), uintptr(TOP_H+TAB_H))
	}
	for i, d := range app.docs {
		r := tabRect(i)
		if r.Right <= tabStripLeft() || r.Left >= tabStripRight() {
			continue
		}
		if app.dragging && i == app.dragTab {
			roundFill(hdc, RECT{r.Left + 3, r.Top + 8, r.Right - 3, r.Bottom - 6}, p.line, 9)
			continue
		}
		tabHovered := app.hover == fmt.Sprintf("tab:%d", i) || app.hover == fmt.Sprintf("tabclose:%d", i)
		if i == app.active {
			roundFill(hdc, RECT{r.Left, r.Top + 5, r.Right, r.Bottom - 4}, p.panel, 9)
			fill(hdc, RECT{r.Left + 12, r.Bottom - 4, r.Right - 12, r.Bottom - 2}, p.accent)
		} else if tabHovered {
			roundFill(hdc, RECT{r.Left, r.Top + 5, r.Right, r.Bottom - 4}, p.accentSoft, 9)
			fill(hdc, RECT{r.Left + 14, r.Bottom - 4, r.Right - 14, r.Bottom - 2}, p.accent)
		}
		drawText(hdc, d.Title, RECT{r.Left + 12, r.Top, r.Right - 28, r.Bottom}, app.fontUI, func() uint32 {
			if i == app.active || tabHovered {
				return p.text
			}
			return p.muted
		}(), DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		closeR := RECT{r.Right - 26, r.Top + 6, r.Right - 6, r.Bottom - 6}
		if app.hover == fmt.Sprintf("tabclose:%d", i) {
			roundFill(hdc, closeR, p.line, 7)
		}
		drawText(hdc, "×", RECT{r.Right - 26, r.Top, r.Right - 6, r.Bottom}, app.fontUI, func() uint32 {
			if app.hover == fmt.Sprintf("tabclose:%d", i) {
				return p.text
			}
			return p.muted
		}(), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	}
	if app.dragging && app.dragTab >= 0 && app.dragTab < len(app.docs) {
		w := tabWidth()
		cx := app.dragVisualX
		if cx == 0 {
			cx = app.dragStartX
		}
		fr := RECT{cx - w/2, TOP_H + 1, cx + w/2, TOP_H + TAB_H - 1}
		if fr.Left < tabStripLeft()+2 {
			fr.Right += tabStripLeft() + 2 - fr.Left
			fr.Left = tabStripLeft() + 2
		}
		if fr.Right > tabStripRight()-2 {
			fr.Left -= fr.Right - (tabStripRight() - 2)
			fr.Right = tabStripRight() - 2
		}
		shadowColor := rgb(8, 10, 14)
		if app.cfg.Theme == "light" {
			shadowColor = rgb(205, 210, 222)
		}
		roundFill(hdc, RECT{fr.Left + 3, fr.Top + 4, fr.Right + 3, fr.Bottom + 4}, shadowColor, 10)
		roundFill(hdc, fr, p.accentSoft, 10)
		fill(hdc, RECT{fr.Left + 12, fr.Bottom - 3, fr.Right - 12, fr.Bottom - 1}, p.accent)
		drawText(hdc, app.docs[app.dragTab].Title, RECT{fr.Left + 14, fr.Top, fr.Right - 14, fr.Bottom}, app.fontUI, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
	if savedDC != 0 {
		procRestoreDC.Call(uintptr(hdc), savedDC)
	}
	pr := plusRect()
	if app.hover == "plus" {
		roundFill(hdc, pr, p.accentSoft, 8)
	} else {
		roundFill(hdc, pr, p.panel, 8)
	}
	drawText(hdc, "+", pr, app.fontUISemibold, p.text, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	if app.view == "split" && len(app.docs) > 0 {
		x := int32(SIDEBAR_W) + (app.clientW-SIDEBAR_W)/2
		fill(hdc, RECT{x - 1, TOP_H + TAB_H, x, app.clientH - STATUS_H}, p.line)
	}
	paintEmptyState(hdc)
	statusColor := p.statusGood
	if strings.Contains(strings.ToLower(app.saveStatus), "failed") || strings.Contains(strings.ToLower(app.saveStatus), "external") {
		statusColor = p.danger
	}
	drawText(hdc, app.saveStatus, RECT{16, app.clientH - STATUS_H, 340, app.clientH}, app.fontSmall, statusColor, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	if d := activeDoc(); d != nil {
		right := fmt.Sprintf("%d words   %d characters   %s", wordCount(d.Content), len([]rune(d.Content)), d.Path)
		drawText(hdc, right, RECT{360, app.clientH - STATUS_H, app.clientW - 18, app.clientH}, app.fontSmall, p.muted, DT_RIGHT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
	}
	paintTooltip(hdc)
}

func paintOnboarding(hdc syscall.Handle) {
	p := colors()
	fill(hdc, RECT{0, 0, app.clientW, app.clientH}, p.bg)
	cw := int32(760)
	ch := int32(480)
	x := (app.clientW - cw) / 2
	y := (app.clientH - ch) / 2
	card := RECT{x, y, x + cw, y + ch}
	roundFill(hdc, card, p.panel, 22)
	drawText(hdc, "T", RECT{x + 42, y + 34, x + 86, y + 78}, app.fontUISemibold, p.text, DT_CENTER|DT_VCENTER|DT_SINGLELINE)
	drawText(hdc, "Welcome to Typora", RECT{x + 104, y + 34, x + 650, y + 76}, app.fontUISemibold, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
	if app.tutorialStep == 0 {
		drawText(hdc, "A fast native Markdown notebook that behaves more like modern Notepad than a traditional editor.", RECT{x + 52, y + 116, x + 708, y + 190}, app.fontUI, p.muted, DT_LEFT|DT_WORDBREAK)
		drawText(hdc, "• Tabs restore after restart\n• Notes save continuously\n• Unsaved work has a separate recovery buffer\n• No browser, account, cloud, or subscription", RECT{x + 52, y + 204, x + 700, y + 330}, app.fontUI, p.text, DT_LEFT|DT_WORDBREAK)
		button(hdc, RECT{x + 548, y + 394, x + 698, y + 438}, "Next", true, "tutorial:next")
	} else if app.tutorialStep == 1 {
		drawText(hdc, "Choose your notes folder", RECT{x + 52, y + 112, x + 700, y + 150}, app.fontUISemibold, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		drawText(hdc, "Every new tab gets a real .md file here automatically. You can change this later.", RECT{x + 52, y + 158, x + 700, y + 214}, app.fontUI, p.muted, DT_LEFT|DT_WORDBREAK)
		roundFill(hdc, RECT{x + 52, y + 232, x + 708, y + 286}, p.panel2, 10)
		drawText(hdc, app.cfg.DefaultFolder, RECT{x + 70, y + 232, x + 686, y + 286}, app.fontMono, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE|DT_END_ELLIPSIS)
		button(hdc, RECT{x + 52, y + 312, x + 230, y + 356}, "Choose folder", false, "tutorial:folder")
		button(hdc, RECT{x + 548, y + 394, x + 698, y + 438}, "Next", true, "tutorial:next")
		button(hdc, RECT{x + 384, y + 394, x + 534, y + 438}, "Back", false, "tutorial:back0")
	} else {
		drawText(hdc, "Saving is intentionally boring", RECT{x + 52, y + 112, x + 700, y + 150}, app.fontUISemibold, p.text, DT_LEFT|DT_VCENTER|DT_SINGLELINE)
		drawText(hdc, "Type normally and Typora saves after a short idle moment. Ctrl+S forces an immediate flush. Ctrl+Shift+S opens the normal Windows Save As dialog.\n\nNew tabs begin in rename mode with Untitled 01 selected. Close the app whenever you want; your tabs and latest buffers return next launch.\n\nOpen this guide again with the small ? in the top-right. Press F1 anytime for the complete keyboard shortcut list.", RECT{x + 52, y + 164, x + 700, y + 350}, app.fontUI, p.text, DT_LEFT|DT_WORDBREAK)
		finishLabel := "Start writing"
		if app.tutorialRevisit {
			finishLabel = "Back to editor"
		}
		button(hdc, RECT{x + 518, y + 394, x + 698, y + 438}, finishLabel, true, "tutorial:finish")
		button(hdc, RECT{x + 352, y + 394, x + 504, y + 438}, "Back", false, "tutorial:back1")
	}
}
func button(hdc syscall.Handle, r RECT, label string, primary bool, key string) {
	p := colors()
	hovered := app.hover == key
	c := p.panel2
	if hovered {
		c = p.accentSoft
	}
	if primary {
		c = p.accent
		if hovered {
			roundFill(hdc, r, p.accentSoft, 10)
			r = RECT{r.Left + 2, r.Top + 2, r.Right - 2, r.Bottom - 2}
		}
	}
	roundFill(hdc, r, c, 10)
	drawText(hdc, label, r, app.fontUI, func() uint32 {
		if primary {
			return rgb(255, 255, 255)
		}
		return p.text
	}(), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
}

func paint() {
	var ps PAINTSTRUCT
	hdc, _, _ := procBeginPaint.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(uintptr(app.hwnd), uintptr(unsafe.Pointer(&ps)))
	if app.clientW <= 0 || app.clientH <= 0 {
		return
	}

	mem, _, _ := procCreateCompatibleDC.Call(hdc)
	if mem == 0 {
		if app.shortcutsVisible {
			paintShortcuts(syscall.Handle(hdc))
		} else if app.onboarding {
			paintOnboarding(syscall.Handle(hdc))
		} else {
			paintNormal(syscall.Handle(hdc))
		}
		return
	}
	bmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(app.clientW), uintptr(app.clientH))
	if bmp == 0 {
		procDeleteDC.Call(mem)
		if app.shortcutsVisible {
			paintShortcuts(syscall.Handle(hdc))
		} else if app.onboarding {
			paintOnboarding(syscall.Handle(hdc))
		} else {
			paintNormal(syscall.Handle(hdc))
		}
		return
	}
	old, _, _ := procSelectObject.Call(mem, bmp)
	if app.shortcutsVisible {
		paintShortcuts(syscall.Handle(mem))
	} else if app.onboarding {
		paintOnboarding(syscall.Handle(mem))
	} else {
		paintNormal(syscall.Handle(mem))
	}
	procBitBlt.Call(hdc, 0, 0, uintptr(app.clientW), uintptr(app.clientH), mem, 0, 0, SRCCOPY)
	procSelectObject.Call(mem, old)
	procDeleteObject.Call(bmp)
	procDeleteDC.Call(mem)
}

func topButtonHit(x, y int32) string {
	if y < 11 || y >= 43 {
		return ""
	}
	buttons := []struct {
		name string
		r    RECT
	}{{"new", RECT{210, 11, 262, 43}}, {"open", RECT{268, 11, 326, 43}}, {"folder", RECT{332, 11, 400, 43}}, {"saveas", RECT{406, 11, 484, 43}}, {"edit", RECT{app.clientW - 324, 11, app.clientW - 270, 43}}, {"split", RECT{app.clientW - 266, 11, app.clientW - 208, 43}}, {"preview", RECT{app.clientW - 204, 11, app.clientW - 130, 43}}, {"help", RECT{app.clientW - 118, 11, app.clientW - 82, 43}}, {"theme", RECT{app.clientW - 70, 11, app.clientW - 30, 43}}}
	for _, b := range buttons {
		if contains(b.r, x, y) {
			return b.name
		}
	}
	return ""
}
