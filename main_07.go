//go:build windows

package main

import (
	"fmt"

	"path/filepath"

	"unsafe"
)

func hoverTargetAt(x, y int32) string {
	if app.shortcutsVisible {
		if contains(shortcutCloseRect(), x, y) {
			return "shortcuts:close"
		}
		return ""
	}
	if app.onboarding {
		cw := int32(760)
		ch := int32(480)
		cx := (app.clientW - cw) / 2
		cy := (app.clientH - ch) / 2
		if app.tutorialStep == 0 && contains(RECT{cx + 548, cy + 394, cx + 698, cy + 438}, x, y) {
			return "tutorial:next"
		}
		if app.tutorialStep == 1 {
			if contains(RECT{cx + 52, cy + 312, cx + 230, cy + 356}, x, y) {
				return "tutorial:folder"
			}
			if contains(RECT{cx + 384, cy + 394, cx + 534, cy + 438}, x, y) {
				return "tutorial:back0"
			}
			if contains(RECT{cx + 548, cy + 394, cx + 698, cy + 438}, x, y) {
				return "tutorial:next"
			}
		}
		if app.tutorialStep == 2 {
			if contains(RECT{cx + 352, cy + 394, cx + 504, cy + 438}, x, y) {
				return "tutorial:back1"
			}
			if contains(RECT{cx + 518, cy + 394, cx + 698, cy + 438}, x, y) {
				return "tutorial:finish"
			}
		}
		return ""
	}
	if len(app.docs) == 0 {
		_, newR, openR := emptyStateRects()
		if contains(newR, x, y) {
			return "empty:new"
		}
		if contains(openR, x, y) {
			return "empty:open"
		}
	}
	if name := topButtonHit(x, y); name != "" {
		return "top:" + name
	}
	if contains(plusRect(), x, y) {
		return "plus"
	}
	if x >= tabStripLeft() && x < tabStripRight() && y >= TOP_H && y < TOP_H+TAB_H {
		for i := range app.docs {
			r := tabRect(i)
			if r.Right <= tabStripLeft() || r.Left >= tabStripRight() {
				continue
			}
			if contains(RECT{r.Right - 30, r.Top, r.Right, r.Bottom}, x, y) {
				return fmt.Sprintf("tabclose:%d", i)
			}
			if contains(r, x, y) {
				return fmt.Sprintf("tab:%d", i)
			}
		}
	}
	if app.cfg.Sidebar && x < SIDEBAR_W && y > TOP_H+70 && y < app.clientH-STATUS_H {
		idx := app.sidebarScroll + int((y-(TOP_H+70))/32)
		if idx >= 0 && idx < len(app.sidebarFiles) {
			return fmt.Sprintf("sidebar:%d", idx)
		}
	}
	return ""
}

func handleMouseMove(x, y int32) {
	app.mouseX, app.mouseY = x, y
	if !app.mouseTracking {
		tme := TRACKMOUSEEVENT{CbSize: uint32(unsafe.Sizeof(TRACKMOUSEEVENT{})), DwFlags: TME_LEAVE, HwndTrack: app.hwnd}
		procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
		app.mouseTracking = true
	}
	next := hoverTargetAt(x, y)
	if next != app.hover {
		procKillTimer.Call(uintptr(app.hwnd), TIMER_TOOLTIP)
		app.tooltipVisible = false
		app.tooltipTarget = ""
		app.hover = next
		if tooltipFor(next) != "" {
			procSetTimer.Call(uintptr(app.hwnd), TIMER_TOOLTIP, 450, 0)
		}
		invalidate()
	}
}

func clearHover() {
	app.mouseTracking = false
	procKillTimer.Call(uintptr(app.hwnd), TIMER_TOOLTIP)
	if app.hover != "" || app.tooltipVisible {
		app.hover = ""
		app.tooltipVisible = false
		app.tooltipTarget = ""
		invalidate()
	}
}

func handleClick(x, y int32) {
	procKillTimer.Call(uintptr(app.hwnd), TIMER_TOOLTIP)
	app.tooltipVisible = false
	app.tooltipTarget = ""
	if app.shortcutsVisible {
		if contains(shortcutCloseRect(), x, y) || !contains(shortcutCardRect(), x, y) {
			closeShortcuts()
		}
		return
	}
	if app.onboarding {
		cw := int32(760)
		ch := int32(480)
		cx := (app.clientW - cw) / 2
		cy := (app.clientH - ch) / 2
		if app.tutorialStep == 0 && contains(RECT{cx + 548, cy + 394, cx + 698, cy + 438}, x, y) {
			app.tutorialStep = 1
			invalidate()
			return
		}
		if app.tutorialStep == 1 {
			if contains(RECT{cx + 52, cy + 312, cx + 230, cy + 356}, x, y) {
				changeDefaultFolder()
				return
			}
			if contains(RECT{cx + 384, cy + 394, cx + 534, cy + 438}, x, y) {
				app.tutorialStep = 0
				invalidate()
				return
			}
			if contains(RECT{cx + 548, cy + 394, cx + 698, cy + 438}, x, y) {
				app.tutorialStep = 2
				invalidate()
				return
			}
		}
		if app.tutorialStep == 2 {
			if contains(RECT{cx + 352, cy + 394, cx + 504, cy + 438}, x, y) {
				app.tutorialStep = 1
				invalidate()
				return
			}
			if contains(RECT{cx + 518, cy + 394, cx + 698, cy + 438}, x, y) {
				if app.tutorialRevisit {
					closeTutorial()
					return
				}
				app.cfg.Onboarded = true
				_ = saveConfig(app.cfg)
				app.onboarding = false
				refreshSidebar()
				layout()
				newDocument()
				invalidate()
				return
			}
		}
		return
	}
	if len(app.docs) == 0 {
		_, newR, openR := emptyStateRects()
		if contains(newR, x, y) {
			newDocument()
			return
		}
		if contains(openR, x, y) {
			openDialog()
			return
		}
	}
	if name := topButtonHit(x, y); name != "" {
		switch name {
		case "new":
			newDocument()
		case "open":
			openDialog()
		case "folder":
			changeDefaultFolder()
		case "saveas":
			saveAsCurrent()
		case "edit":
			setView("edit")
		case "split":
			setView("split")
		case "preview":
			setView("preview")
		case "help":
			openTutorial()
		case "theme":
			toggleTheme()
		}
		return
	}
	if contains(plusRect(), x, y) {
		newDocument()
		return
	}
	if x >= tabStripLeft() && x < tabStripRight() && y >= TOP_H && y < TOP_H+TAB_H {
		for i := range app.docs {
			r := tabRect(i)
			if r.Right <= tabStripLeft() || r.Left >= tabStripRight() {
				continue
			}
			if contains(RECT{r.Right - 30, r.Top, r.Right, r.Bottom}, x, y) {
				closeTab(i)
				return
			}
			if contains(r, x, y) {
				switchTab(i)
				return
			}
		}
	}
	if app.cfg.Sidebar && x < SIDEBAR_W && y > TOP_H+70 && y < app.clientH-STATUS_H {
		idx := app.sidebarScroll + int((y-(TOP_H+70))/32)
		if idx >= 0 && idx < len(app.sidebarFiles) {
			openFile(filepath.Join(app.cfg.DefaultFolder, app.sidebarFiles[idx]))
		}
	}
}

func startTabDrag(x, y int32) bool {
	if app.onboarding || y < TOP_H || y >= TOP_H+TAB_H || x < tabStripLeft() || x >= tabStripRight() {
		return false
	}
	for i := range app.docs {
		r := tabRect(i)
		if r.Right <= tabStripLeft() || r.Left >= tabStripRight() || !contains(r, x, y) {
			continue
		}
		if contains(RECT{r.Right - 30, r.Top, r.Right, r.Bottom}, x, y) {
			return false
		}
		switchTab(i)
		app.dragTab = i
		app.dragStartX, app.dragStartY = x, y
		app.dragVisualX = x
		app.dragging = false
		app.dragMoved = false
		procSetCapture.Call(uintptr(app.hwnd))
		return true
	}
	return false
}

func moveTab(from, to int) {
	if from < 0 || from >= len(app.docs) || to < 0 || to >= len(app.docs) || from == to {
		return
	}
	d := app.docs[from]
	if from < to {
		copy(app.docs[from:to], app.docs[from+1:to+1])
	} else {
		copy(app.docs[to+1:from+1], app.docs[to:from])
	}
	app.docs[to] = d
	app.active = to
	app.dragTab = to
	app.dragMoved = true
	ensureActiveTabVisible()
	invalidateRegion(RECT{tabStripLeft(), TOP_H, app.clientW, TOP_H + TAB_H})
}

func handleTabDrag(x, y int32) {
	if app.dragTab < 0 || len(app.docs) < 2 {
		return
	}
	app.dragVisualX = x
	if app.dragging {
		invalidateRegion(RECT{tabStripLeft(), TOP_H, tabStripRight(), TOP_H + TAB_H})
	}
	if !app.dragging {
		dx := x - app.dragStartX
		dy := y - app.dragStartY
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		if dx < 5 && dy < 5 {
			return
		}
		app.dragging = true
		invalidateRegion(RECT{tabStripLeft(), TOP_H, tabStripRight(), TOP_H + TAB_H})
	}
	left, right := tabStripLeft(), tabStripRight()
	if x < left+24 && app.tabScroll > 0 {
		app.tabScroll -= 18
		if app.tabScroll < 0 {
			app.tabScroll = 0
		}
	} else if x > right-24 && app.tabScroll < maxTabScroll() {
		app.tabScroll += 18
		if app.tabScroll > maxTabScroll() {
			app.tabScroll = maxTabScroll()
		}
	}
	w := tabWidth()
	virtualX := x + app.tabScroll - left
	to := int(virtualX / w)
	if x <= left {
		to = 0
	}
	if x >= right {
		to = len(app.docs) - 1
	}
	if to < 0 {
		to = 0
	}
	if to >= len(app.docs) {
		to = len(app.docs) - 1
	}
	moveTab(app.dragTab, to)
}

func finishTabDrag() {
	if app.dragTab < 0 {
		return
	}
	moved := app.dragMoved
	procReleaseCapture.Call()
	app.dragTab = -1
	app.dragVisualX = 0
	app.dragging = false
	app.dragMoved = false
	invalidateRegion(RECT{tabStripLeft(), TOP_H, tabStripRight(), TOP_H + TAB_H})
	if moved {
		persistSessionAsync()
	}
}

func sidebarItemRect(i int) RECT {
	y := int32(TOP_H+70) + int32(i-app.sidebarScroll)*32
	return RECT{12, y, SIDEBAR_W - 10, y + 30}
}

func startSidebarDrag(x, y int32) bool {
	if app.onboarding || app.shortcutsVisible || !app.cfg.Sidebar || x >= SIDEBAR_W || y <= TOP_H+70 || y >= app.clientH-STATUS_H {
		return false
	}
	idx := app.sidebarScroll + int((y-(TOP_H+70))/32)
	if idx < 0 || idx >= len(app.sidebarFiles) {
		return false
	}
	app.sidebarDrag = idx
	app.sidebarDragStartY = y
	app.sidebarDragVisualY = y
	app.sidebarDragging = false
	app.sidebarDragMoved = false
	procSetCapture.Call(uintptr(app.hwnd))
	return true
}

func moveSidebarItem(from, to int) {
	if from < 0 || from >= len(app.sidebarFiles) || to < 0 || to >= len(app.sidebarFiles) || from == to {
		return
	}
	name := app.sidebarFiles[from]
	if from < to {
		copy(app.sidebarFiles[from:to], app.sidebarFiles[from+1:to+1])
	} else {
		copy(app.sidebarFiles[to+1:from+1], app.sidebarFiles[to:from])
	}
	app.sidebarFiles[to] = name
	app.sidebarDrag = to
	app.sidebarDragMoved = true
	invalidateSidebar()
}

func handleSidebarDrag(x, y int32) {
	if app.sidebarDrag < 0 || len(app.sidebarFiles) == 0 {
		return
	}
	app.sidebarDragVisualY = y
	if !app.sidebarDragging {
		dy := y - app.sidebarDragStartY
		if dy < 0 {
			dy = -dy
		}
		if dy < 5 {
			return
		}
		app.sidebarDragging = true
		invalidateSidebar()
	}
	if y < TOP_H+86 && app.sidebarScroll > 0 {
		app.sidebarScroll--
	} else if y > app.clientH-STATUS_H-34 {
		visible := int((app.clientH - STATUS_H - (TOP_H + 70)) / 32)
		maxScroll := max(0, len(app.sidebarFiles)-visible)
		if app.sidebarScroll < maxScroll {
			app.sidebarScroll++
		}
	}
	to := app.sidebarScroll + int((y-(TOP_H+70))/32)
	if to < 0 {
		to = 0
	}
	if to >= len(app.sidebarFiles) {
		to = len(app.sidebarFiles) - 1
	}
	moveSidebarItem(app.sidebarDrag, to)
}

func finishSidebarDrag() {
	if app.sidebarDrag < 0 {
		return
	}
	idx := app.sidebarDrag
	moved := app.sidebarDragMoved
	name := ""
	if idx >= 0 && idx < len(app.sidebarFiles) {
		name = app.sidebarFiles[idx]
	}
	procReleaseCapture.Call()
	app.sidebarDrag = -1
	app.sidebarDragVisualY = 0
	app.sidebarDragging = false
	app.sidebarDragMoved = false
	invalidateSidebar()

	procUpdateWindow.Call(uintptr(app.hwnd))
	if moved {
		rememberSidebarOrder()
		return
	}
	if name != "" {
		openFile(filepath.Join(app.cfg.DefaultFolder, name))
	}
}

func handleMiddleClick(x, y int32) {
	if app.onboarding || y < TOP_H || y >= TOP_H+TAB_H || x < tabStripLeft() || x >= tabStripRight() {
		return
	}
	for i := range app.docs {
		r := tabRect(i)
		if r.Right <= tabStripLeft() || r.Left >= tabStripRight() {
			continue
		}
		if contains(r, x, y) {
			closeTab(i)
			return
		}
	}
}

func handleWheel(delta int16, x, y int32) {
	if y >= TOP_H && y < TOP_H+TAB_H && x >= tabStripLeft() && x < app.clientW {
		step := int32(88)
		if delta < 0 {
			app.tabScroll += step
		} else {
			app.tabScroll -= step
		}
		if app.tabScroll < 0 {
			app.tabScroll = 0
		}
		if m := maxTabScroll(); app.tabScroll > m {
			app.tabScroll = m
		}
		invalidateRegion(RECT{tabStripLeft(), TOP_H, app.clientW, TOP_H + TAB_H})
		return
	}
	if !app.cfg.Sidebar || x >= SIDEBAR_W {
		return
	}
	if delta < 0 {
		app.sidebarScroll += 3
	} else {
		app.sidebarScroll -= 3
	}
	maxScroll := max(0, len(app.sidebarFiles)-1)
	if app.sidebarScroll < 0 {
		app.sidebarScroll = 0
	}
	if app.sidebarScroll > maxScroll {
		app.sidebarScroll = maxScroll
	}
	invalidateSidebar()
}

func handleTimer(id uintptr) {
	switch id {
	case TIMER_AUTOSAVE:
		procKillTimer.Call(uintptr(app.hwnd), TIMER_AUTOSAVE)
		syncEditorToDoc()
		if d := activeDoc(); d != nil {
			if d.DiskMissing {
				app.saveStatus = "Deleted externally • Ctrl+S to restore or Ctrl+Shift+S for Save As"
				invalidateStatus()
			} else {
				queueDiskSave(d, false, false)
			}
		}
	case TIMER_PREVIEW:
		procKillTimer.Call(uintptr(app.hwnd), TIMER_PREVIEW)
		syncEditorToDoc()
		renderPreview()
	case TIMER_RECOVERY:
		procKillTimer.Call(uintptr(app.hwnd), TIMER_RECOVERY)
		syncEditorToDoc()
		if d := activeDoc(); d != nil {
			queueRecovery(d)
		}
	case TIMER_TOOLTIP:
		procKillTimer.Call(uintptr(app.hwnd), TIMER_TOOLTIP)
		if app.hover != "" && tooltipFor(app.hover) != "" {
			app.tooltipVisible = true
			app.tooltipTarget = app.hover
			invalidate()
		}
	case TIMER_SMOOTH_SCROLL:
		if app.scrollTarget == 0 || app.smoothScrollRemaining == 0 {
			procKillTimer.Call(uintptr(app.hwnd), TIMER_SMOOTH_SCROLL)
			return
		}
		step := 1
		if app.smoothScrollRemaining < 0 {
			step = -1
		}
		send(app.scrollTarget, EM_LINESCROLL, 0, uintptr(int64(step)))
		app.smoothScrollRemaining -= step
		if app.smoothScrollRemaining == 0 {
			procKillTimer.Call(uintptr(app.hwnd), TIMER_SMOOTH_SCROLL)
		}
	}
}
