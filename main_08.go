//go:build windows

package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"

	"syscall"
	"time"

	"unsafe"
)

func handleHotkey(msg *MSG) bool {
	if msg.Message != WM_KEYDOWN {
		return false
	}
	key := uint32(msg.WParam)
	ctrl, _, _ := procGetKeyState.Call(VK_CONTROL)
	shift, _, _ := procGetKeyState.Call(VK_SHIFT)
	isCtrl := int16(ctrl) < 0
	isShift := int16(shift) < 0
	if msg.Hwnd == app.renameEdit {
		if key == VK_RETURN {
			commitRename()
			return true
		}
		if key == VK_ESCAPE {
			cancelRename()
			return true
		}
	}
	if key == VK_ESCAPE && app.contextVisible {
		closeContextMenu()
		return true
	}
	if key == VK_F1 {
		if app.shortcutsVisible {
			closeShortcuts()
		} else {
			openShortcuts()
		}
		return true
	}
	if app.shortcutsVisible {
		if key == VK_ESCAPE {
			closeShortcuts()
		}
		return true
	}
	if key == VK_ESCAPE && app.onboarding && app.tutorialRevisit {
		closeTutorial()
		return true
	}
	if key == VK_F2 && !app.onboarding {
		beginRename()
		return true
	}
	if !isCtrl {
		return false
	}
	switch key {
	case 'N':
		newDocument()
		return true
	case 'O':
		openDialog()
		return true
	case 'S':
		if isShift {
			saveAsCurrent()
		} else {
			resolveManualSave()
		}
		return true
	case 'W':
		closeTab(app.active)
		return true
	case VK_TAB:
		if len(app.docs) > 1 {
			n := app.active + 1
			if isShift {
				n = app.active - 1
				if n < 0 {
					n = len(app.docs) - 1
				}
			} else if n >= len(app.docs) {
				n = 0
			}
			switchTab(n)
		}
		return true
	case 'T':
		if isShift {
			reopenClosed()
			return true
		}
	}
	return false
}

func wndProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		return 0
	case WM_SIZE:
		layout()
		invalidate()
		return 0
	case WM_PAINT:
		paint()
		return 0
	case WM_ERASEBKGND:
		return 1
	case WM_SETFOCUS:
		if !app.onboarding && !app.shortcutsVisible && app.editor != 0 && activeDoc() != nil {
			procSetFocus.Call(uintptr(app.editor))
		}
		return 0
	case WM_MOUSEMOVE:
		x, y := signedWord(lparam), int32(int16((lparam>>16)&0xffff))
		handleTabDrag(x, y)
		handleSidebarDrag(x, y)
		handleMouseMove(x, y)
		return 0
	case WM_MOUSELEAVE:
		clearHover()
		return 0
	case WM_LBUTTONDOWN:
		x, y := signedWord(lparam), int32(int16((lparam>>16)&0xffff))
		if app.shortcutsVisible {
			handleClick(x, y)
			return 0
		}
		if startTabDrag(x, y) {
			return 0
		}
		if startSidebarDrag(x, y) {
			return 0
		}
		handleClick(x, y)
		return 0
	case WM_LBUTTONUP:
		finishTabDrag()
		finishSidebarDrag()
		return 0
	case WM_RBUTTONUP:
		handleRightClick(signedWord(lparam), int32(int16((lparam>>16)&0xffff)))
		return 0
	case WM_CAPTURECHANGED:
		app.dragTab = -1
		app.dragVisualX = 0
		app.dragging = false
		app.dragMoved = false
		app.sidebarDrag = -1
		app.sidebarDragVisualY = 0
		app.sidebarDragging = false
		app.sidebarDragMoved = false
		return 0
	case WM_EXITSIZEMOVE:
		rememberWindowPlacement()
		_ = saveConfig(app.cfg)
		return 0
	case WM_MBUTTONDOWN:
		handleMiddleClick(signedWord(lparam), int32(int16((lparam>>16)&0xffff)))
		return 0
	case WM_MOUSEWHEEL:
		pt := POINT{X: signedWord(lparam), Y: int32(int16((lparam >> 16) & 0xffff))}
		procScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pt)))
		handleWheel(int16((wparam>>16)&0xffff), pt.X, pt.Y)
		return 0
	case WM_COMMAND:
		id := int(loWord(wparam))
		code := hiWord(wparam)
		if id == IDC_EDITOR && code == EN_CHANGE && !app.suppress {
			if d := activeDoc(); d != nil {
				d.Revision++
			}
			app.saveStatus = "Saving…"
			scheduleRecovery()
			scheduleAutosave()
			schedulePreview()
			invalidateStatus()
			return 0
		}
		if id == IDC_RENAME && code == EN_KILLFOCUS {
			commitRename()
			return 0
		}
	case WM_NOTIFY:
		if lparam != 0 {
			hdr := (*NMHDR)(unsafe.Pointer(lparam))
			if hdr.HwndFrom == app.preview && hdr.Code == EN_LINK {
				el := (*ENLINK)(unsafe.Pointer(lparam))
				if el.Msg == WM_LBUTTONUP {
					openPreviewLinkAt(el.Chrg.CpMin, el.Chrg.CpMax)
					return 1
				}
			}
		}
		return 0
	case WM_TIMER:
		handleTimer(wparam)
		return 0
	case WM_APP_IO_RESULT:
		handleIOResults()
		return 0
	case WM_CTLCOLOREDIT:
		p := colors()
		hdc := syscall.Handle(wparam)
		procSetTextColor.Call(uintptr(hdc), uintptr(p.text))
		procSetBkMode.Call(uintptr(hdc), TRANSPARENT)
		if app.editorBrush != 0 {
			return uintptr(app.editorBrush)
		}
		b, _, _ := procCreateSolidBrush.Call(uintptr(p.editorBg))
		app.editorBrush = syscall.Handle(b)
		return b
	case WM_CLOSE:
		rememberWindowPlacement()
		_ = saveConfig(app.cfg)
		saveAll()
		procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
		return 0
	case WM_DESTROY:
		stopSidebarFolderWatcher()
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func startupFailure(stage string, err any) {
	f, _ := os.OpenFile(filepath.Join(os.TempDir(), "Typora-v4.20.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if f != nil {
		fmt.Fprintf(f, "%s FAILED %s: %v\n%s\n", time.Now().Format(time.RFC3339Nano), stage, err, debug.Stack())
		f.Close()
	}
	message(fmt.Sprintf("Typora could not start.\n\nStage: %s\n%v\n\nLog: %%TEMP%%\\Typora-v4.19.log", stage, err), "Typora startup error", MB_OK|MB_ICONWARNING)
}

func loadAppIcon(hInst uintptr, size int32) syscall.Handle {
	if h, _, _ := procLoadImage.Call(hInst, 1, IMAGE_ICON, uintptr(size), uintptr(size), 0); h != 0 {
		return syscall.Handle(h)
	}

	icoPath := filepath.Join(filepath.Dir(os.Args[0]), "Typora.ico")
	if h, _, _ := procLoadImage.Call(0, uintptr(unsafe.Pointer(wstr(icoPath))), IMAGE_ICON, uintptr(size), uintptr(size), LR_LOADFROMFILE); h != 0 {
		return syscall.Handle(h)
	}
	return 0
}

func main() {
	runtime.LockOSThread()
	f, _ := os.OpenFile(filepath.Join(os.TempDir(), "Typora-v4.20.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if f != nil {
		log.SetOutput(f)
		defer f.Close()
	}
	log.Printf("Typora %s native starting", appVersion)
	defer func() {
		if r := recover(); r != nil {
			startupFailure("panic", r)
		}
	}()
	procSetProcessDPIAware.Call()
	log.Printf("Loading native RichEdit")
	err := msftedit.Load()
	if err != nil {
		startupFailure("loading native RichEdit", err)
		return
	}
	log.Printf("RichEdit loaded")
	app.cfg = loadConfig()
	app.view = app.cfg.View
	log.Printf("Config loaded view=%s version=%d", app.view, app.cfg.Version)
	if app.view == "" {
		app.view = "edit"
	}
	app.closed = loadClosed()
	app.active = -1
	app.dragTab = -1
	app.sidebarDrag = -1
	ensureDefaultFolder()
	app.onboarding = !app.cfg.Onboarded
	hInst, _, _ := procGetModuleHandle.Call(0)
	cursor, _, _ := procLoadCursor.Call(0, IDC_ARROW)
	app.iconBig = loadAppIcon(hInst, 32)
	app.iconSmall = loadAppIcon(hInst, 16)
	app.iconHeader = loadAppIcon(hInst, 24)
	className := wstr("TyporaNativeV413Window")
	wc := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(wndProc), HInstance: syscall.Handle(hInst), HIcon: app.iconBig, HCursor: syscall.Handle(cursor), HbrBackground: syscall.Handle(COLOR_WINDOW + 1), LpszClassName: className, HIconSm: app.iconSmall}
	if r, _, e := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		startupFailure("registering window class", e)
		return
	}
	contextClassName := wstr("TyporaContextMenuV413")
	contextWC := WNDCLASSEX{CbSize: uint32(unsafe.Sizeof(WNDCLASSEX{})), LpfnWndProc: syscall.NewCallback(contextMenuWndProc), HInstance: syscall.Handle(hInst), HCursor: syscall.Handle(cursor), LpszClassName: contextClassName}
	if r, _, e := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&contextWC))); r == 0 {
		startupFailure("registering context menu class", e)
		return
	}
	px, py, pw, ph, startMaximized, havePlacement := startupWindowPlacement()
	xArg, yArg := uintptr(CW_USEDEFAULT), uintptr(CW_USEDEFAULT)
	if havePlacement {
		xArg = uintptr(int64(px))
		yArg = uintptr(int64(py))
		log.Printf("Restoring window placement x=%d y=%d w=%d h=%d maximized=%v", px, py, pw, ph, startMaximized)
	} else {
		pw, ph = 1360, 860
		log.Printf("No usable saved window placement; using Windows default")
	}
	hwnd, _, e := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(wstr(appName))), WS_OVERLAPPEDWINDOW|WS_VISIBLE|WS_CLIPCHILDREN, xArg, yArg, uintptr(pw), uintptr(ph), 0, 0, hInst, 0)
	if hwnd == 0 {
		startupFailure("creating main window", e)
		return
	}
	app.hwnd = syscall.Handle(hwnd)
	startIOWorkers()
	log.Printf("Main window created hwnd=0x%x", hwnd)
	app.fontUI = createFont(16, 400, "Segoe UI")
	app.fontUISemibold = createFont(18, 600, "Segoe UI Semibold")
	app.fontSmall = createFont(13, 400, "Segoe UI")
	app.fontMono = createFont(15, 400, "Cascadia Mono")
	app.editor = createChild("RICHEDIT50W", WS_CHILD|WS_VISIBLE|WS_VSCROLL|WS_TABSTOP|ES_MULTILINE|ES_AUTOVSCROLL|ES_NOHIDESEL|ES_WANTRETURN, IDC_EDITOR)
	app.preview = createChild("RICHEDIT50W", WS_CHILD|WS_VSCROLL|ES_MULTILINE|ES_AUTOVSCROLL|ES_READONLY, IDC_PREVIEW)
	if app.editor == 0 || app.preview == 0 {
		startupFailure("creating editor controls", errors.New("RichEdit child window creation failed"))
		return
	}
	send(app.editor, WM_SETFONT, uintptr(app.fontMono), 1)
	send(app.preview, WM_SETFONT, uintptr(app.fontUI), 1)
	setEditorStyle()
	installRichEditSubclasses()
	log.Printf("Editor controls created")
	if app.iconBig != 0 {
		send(app.hwnd, WM_SETICON, ICON_BIG, uintptr(app.iconBig))
	}
	if app.iconSmall != 0 {
		send(app.hwnd, WM_SETICON, ICON_SMALL, uintptr(app.iconSmall))
	}
	refreshSidebar()
	startSidebarFolderWatcher(app.cfg.DefaultFolder)
	layout()
	showCmd := uintptr(SW_SHOWNORMAL)
	if startMaximized && havePlacement {
		showCmd = SW_SHOWMAXIMIZED
	}
	procShowWindow.Call(hwnd, showCmd)
	procUpdateWindow.Call(hwnd)
	log.Printf("Window shown onboarding=%v", app.onboarding)
	if !app.onboarding {
		restoreSession()
		for _, d := range app.docs {
			queueDiskSave(d, false, false)
		}
		layout()
		if activeDoc() != nil {
			procSetFocus.Call(uintptr(app.editor))
		} else {
			procSetFocus.Call(uintptr(app.hwnd))
		}
	} else {
		app.saveStatus = "First-run setup"
		invalidate()
	}
	if len(os.Args) > 1 && !app.onboarding {
		if st, er := os.Stat(os.Args[1]); er == nil && !st.IsDir() {
			openFile(os.Args[1])
		}
	}
	log.Printf("Entering message loop docs=%d active=%d", len(app.docs), app.active)
	var m MSG
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		if handleHotkey(&m) {
			continue
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
	if app.editorBrush != 0 {
		procDeleteObject.Call(uintptr(app.editorBrush))
	}
	for _, h := range []syscall.Handle{app.fontUI, app.fontUISemibold, app.fontSmall, app.fontMono} {
		if h != 0 {
			procDeleteObject.Call(uintptr(h))
		}
	}
}
