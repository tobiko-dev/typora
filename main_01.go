//go:build windows

package main

import (
	"errors"
	"syscall"
	"unsafe"
)

const (
	appName    = "Typora"
	appVersion = "4.18.0"

	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_POPUP            = 0x80000000
	WS_VISIBLE          = 0x10000000
	WS_EX_TOOLWINDOW    = 0x00000080
	WS_CHILD            = 0x40000000
	WS_VSCROLL          = 0x00200000
	WS_HSCROLL          = 0x00100000
	WS_TABSTOP          = 0x00010000
	WS_CLIPCHILDREN     = 0x02000000
	ES_MULTILINE        = 0x0004
	ES_AUTOVSCROLL      = 0x0040
	ES_AUTOHSCROLL      = 0x0080
	ES_NOHIDESEL        = 0x0100
	ES_WANTRETURN       = 0x1000
	ES_READONLY         = 0x0800

	CW_USEDEFAULT    = 0x80000000
	SW_SHOW          = 5
	SW_HIDE          = 0
	SW_SHOWNORMAL    = 1
	SW_SHOWMAXIMIZED = 3

	WM_CREATE         = 0x0001
	WM_DESTROY        = 0x0002
	WM_SETREDRAW      = 0x000B
	WM_SIZE           = 0x0005
	WM_SETFOCUS       = 0x0007
	WM_KILLFOCUS      = 0x0008
	WM_CLOSE          = 0x0010
	WM_PAINT          = 0x000F
	WM_ERASEBKGND     = 0x0014
	WM_COMMAND        = 0x0111
	WM_NOTIFY         = 0x004E
	WM_TIMER          = 0x0113
	WM_MOUSEMOVE      = 0x0200
	WM_LBUTTONDOWN    = 0x0201
	WM_LBUTTONUP      = 0x0202
	WM_RBUTTONUP      = 0x0205
	WM_MBUTTONDOWN    = 0x0207
	WM_CAPTURECHANGED = 0x0215
	WM_EXITSIZEMOVE   = 0x0232
	WM_MOUSEWHEEL     = 0x020A
	WM_MOUSELEAVE     = 0x02A3
	WM_THEMECHANGED   = 0x031A
	WM_SETFONT        = 0x0030
	WM_SETICON        = 0x0080
	WM_CTLCOLOREDIT   = 0x0133
	WM_APP_IO_RESULT  = 0x8001

	WM_SETTEXT       = 0x000C
	WM_GETTEXT       = 0x000D
	WM_GETTEXTLENGTH = 0x000E

	WM_KEYDOWN = 0x0100
	VK_LBUTTON = 0x01
	VK_CONTROL = 0x11
	VK_SHIFT   = 0x10
	VK_RETURN  = 0x0D
	VK_ESCAPE  = 0x1B
	VK_TAB     = 0x09
	VK_F1      = 0x70
	VK_F2      = 0x71

	EN_CHANGE    = 0x0300
	EN_KILLFOCUS = 0x0200
	EN_LINK      = 0x070B

	// RichEdit event masks / link formatting.
	ENM_LINK = 0x04000000
	CFM_LINK = 0x00000020
	CFE_LINK = 0x00000020

	EM_POSFROMCHAR        = 0x0426
	EM_SETBKGNDCOLOR      = 0x0443
	EM_SETEVENTMASK       = 0x0445
	EM_SETEDITSTYLE       = 0x04CC
	EM_SETMARGINS         = 0x00D3
	EM_SETSEL             = 0x00B1
	EM_LINESCROLL         = 0x00B6
	EM_STREAMIN           = 0x0449
	EM_SETREADONLY        = 0x00CF
	EM_SETCHARFORMAT      = 0x0444
	ENM_CHANGE            = 0x00000001
	SES_HYPERLINKTOOLTIPS = 0x00000008
	SES_NOFOCUSLINKNOTIFY = 0x00000020
	TME_LEAVE             = 0x00000002
	EC_LEFTMARGIN         = 0x0001
	EC_RIGHTMARGIN        = 0x0002
	SF_RTF                = 0x0002
	SCF_SELECTION         = 0x0001
	SCF_ALL               = 0x0004
	CFM_BOLD              = 0x00000001
	CFM_ITALIC            = 0x00000002
	CFM_UNDERLINE         = 0x00000004
	CFM_STRIKEOUT         = 0x00000008
	CFM_BACKCOLOR         = 0x04000000
	CFE_BOLD              = 0x00000001
	CFE_ITALIC            = 0x00000002
	CFE_UNDERLINE         = 0x00000004
	CFE_STRIKEOUT         = 0x00000008
	CFM_SIZE              = 0x80000000
	CFM_COLOR             = 0x40000000
	CFM_FACE              = 0x20000000

	ICON_SMALL      = 0
	ICON_BIG        = 1
	IMAGE_ICON      = 1
	LR_LOADFROMFILE = 0x00000010
	LR_DEFAULTSIZE  = 0x00000040
	DI_NORMAL       = 0x0003

	IDC_ARROW    = 32512
	COLOR_WINDOW = 5

	DT_LEFT         = 0x00000000
	DT_CENTER       = 0x00000001
	DT_RIGHT        = 0x00000002
	DT_VCENTER      = 0x00000004
	DT_SINGLELINE   = 0x00000020
	DT_END_ELLIPSIS = 0x00008000
	DT_WORDBREAK    = 0x00000010
	DT_NOPREFIX     = 0x00000800
	TRANSPARENT     = 1
	PS_SOLID        = 0
	SRCCOPY         = 0x00CC0020

	MB_OK           = 0x00000000
	MB_OKCANCEL     = 0x00000001
	MB_YESNOCANCEL  = 0x00000003
	MB_YESNO        = 0x00000004
	MB_ICONQUESTION = 0x00000020
	MB_ICONWARNING  = 0x00000030
	IDOK            = 1
	IDCANCEL        = 2
	IDYES           = 6
	IDNO            = 7

	MF_STRING       = 0x00000000
	MF_SEPARATOR    = 0x00000800
	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100

	TIMER_AUTOSAVE      = 1
	TIMER_PREVIEW       = 2
	TIMER_RECOVERY      = 3
	TIMER_TOOLTIP       = 4
	TIMER_SMOOTH_SCROLL = 5

	IDC_EDITOR  = 2001
	IDC_PREVIEW = 2002
	IDC_RENAME  = 2003

	TOP_H     = 54
	TAB_H     = 42
	STATUS_H  = 28
	SIDEBAR_W = 238
)

type POINT struct{ X, Y int32 }

type SIZE struct{ Cx, Cy int32 }

type MSG struct {
	Hwnd           syscall.Handle
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Pt             POINT
}

type RECT struct{ Left, Top, Right, Bottom int32 }

type WINDOWPLACEMENT struct {
	Length           uint32
	Flags            uint32
	ShowCmd          uint32
	PtMinPosition    POINT
	PtMaxPosition    POINT
	RcNormalPosition RECT
}

type MONITORINFO struct {
	CbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
}

type TRACKMOUSEEVENT struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   syscall.Handle
	DwHoverTime uint32
}

type NMHDR struct {
	HwndFrom syscall.Handle
	IDFrom   uintptr
	Code     uint32
}

type CHARRANGE struct{ CpMin, CpMax int32 }

type ENLINK struct {
	Nmhdr  NMHDR
	Msg    uint32
	_      uint32 // align WPARAM on 64-bit Windows
	WParam uintptr
	LParam uintptr
	Chrg   CHARRANGE
}

type WNDCLASSEX struct {
	CbSize                                   uint32
	Style                                    uint32
	LpfnWndProc                              uintptr
	CbClsExtra, CbWndExtra                   int32
	HInstance, HIcon, HCursor, HbrBackground syscall.Handle
	LpszMenuName, LpszClassName              *uint16
	HIconSm                                  syscall.Handle
}

type PAINTSTRUCT struct {
	Hdc                  syscall.Handle
	FErase               int32
	RcPaint              RECT
	FRestore, FIncUpdate int32
	RgbReserved          [32]byte
}

type CHARFORMAT2 struct {
	CbSize          uint32
	DwMask          uint32
	DwEffects       uint32
	YHeight         int32
	YOffset         int32
	CrTextColor     uint32
	BCharSet        byte
	BPitchAndFamily byte
	SzFaceName      [32]uint16
	WWeight         uint16
	SSpacing        int16
	CrBackColor     uint32
	Lcid            uint32
	DwReserved      uint32
	SStyle          int16
	WKerning        uint16
	BUnderlineType  byte
	BAnimation      byte
	BRevAuthor      byte
	BReserved1      byte
}

type Document struct {
	ID          string
	Title       string
	Path        string
	BufferPath  string
	Content     string
	AutoCreated bool
	LastDiskMod int64
	Conflict    bool
	DiskMissing bool
	Revision    uint64
}

type palette struct {
	bg, panel, panel2, line, text, muted, accent, accentSoft, editorBg, statusGood, danger uint32
}

type App struct {
	hwnd, editor, preview, renameEdit           syscall.Handle
	iconBig, iconSmall, iconHeader              syscall.Handle
	fontUI, fontUISemibold, fontSmall, fontMono syscall.Handle
	editorBrush                                 syscall.Handle
	cfg                                         Config
	docs                                        []*Document
	active                                      int
	closed                                      []ClosedDoc
	sidebarFiles                                []string
	sidebarScroll                               int
	view                                        string
	suppress                                    bool
	saveStatus                                  string
	onboarding                                  bool
	tutorialStep                                int
	tutorialRevisit                             bool
	previewLinks                                []previewLink
	previewCodeBlocks                           []previewCodeBlock
	clientW, clientH                            int32
	renameOriginal                              string
	renameActive                                bool
	hover                                       string
	mouseTracking                               bool
	tabScroll                                   int32
	dragTab                                     int
	dragStartX, dragStartY                      int32
	dragVisualX                                 int32
	dragging, dragMoved                         bool
	sidebarDrag                                 int
	sidebarDragStartY, sidebarDragVisualY       int32
	sidebarDragging, sidebarDragMoved           bool
	contextVisible                              bool
	contextHwnd                                 syscall.Handle
	contextKind                                 string
	contextPath                                 string
	contextTab                                  int
	contextX, contextY                          int32
	contextHover                                int
	tooltipVisible                              bool
	tooltipTarget                               string
	mouseX, mouseY                              int32
	shortcutsVisible                            bool
	scrollTarget                                syscall.Handle
	smoothScrollRemaining                       int
	wheelRemainder                              int
}

var app App

var oldEditorWndProc uintptr

var oldPreviewWndProc uintptr

var richEditSubclassCallback uintptr

var errExternalChange = errors.New("file changed outside Typora")

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	msftedit = syscall.NewLazyDLL("Msftedit.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	uxtheme  = syscall.NewLazyDLL("uxtheme.dll")

	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessageW")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procPostMessage         = user32.NewProc("PostMessageW")
	procLoadCursor          = user32.NewProc("LoadCursorW")
	procSendMessage         = user32.NewProc("SendMessageW")
	procSetWindowText       = user32.NewProc("SetWindowTextW")
	procSetWindowLongPtr    = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProc      = user32.NewProc("CallWindowProcW")
	procGetWindowPlacement  = user32.NewProc("GetWindowPlacement")
	procMonitorFromRect     = user32.NewProc("MonitorFromRect")
	procGetMonitorInfo      = user32.NewProc("GetMonitorInfoW")
	procGetWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	procGetWindowText       = user32.NewProc("GetWindowTextW")
	procMoveWindow          = user32.NewProc("MoveWindow")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procScreenToClient      = user32.NewProc("ScreenToClient")
	procClientToScreen      = user32.NewProc("ClientToScreen")
	procInvalidateRect      = user32.NewProc("InvalidateRect")
	procTrackMouseEvent     = user32.NewProc("TrackMouseEvent")
	procSetCapture          = user32.NewProc("SetCapture")
	procReleaseCapture      = user32.NewProc("ReleaseCapture")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procRedrawWindow        = user32.NewProc("RedrawWindow")
	procBeginPaint          = user32.NewProc("BeginPaint")
	procEndPaint            = user32.NewProc("EndPaint")
	procFillRect            = user32.NewProc("FillRect")
	procDrawText            = user32.NewProc("DrawTextW")
	procSetFocus            = user32.NewProc("SetFocus")
	procSetTimer            = user32.NewProc("SetTimer")
	procKillTimer           = user32.NewProc("KillTimer")
	procGetKeyState         = user32.NewProc("GetKeyState")
	procMessageBox          = user32.NewProc("MessageBoxW")
	procLoadImage           = user32.NewProc("LoadImageW")
	procDrawIconEx          = user32.NewProc("DrawIconEx")
	procSetProcessDPIAware  = user32.NewProc("SetProcessDPIAware")

	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procRectangle              = gdi32.NewProc("Rectangle")
	procCreateFont             = gdi32.NewProc("CreateFontW")
	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procGetTextExtentPoint32   = gdi32.NewProc("GetTextExtentPoint32W")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procSaveDC                 = gdi32.NewProc("SaveDC")
	procRestoreDC              = gdi32.NewProc("RestoreDC")
	procIntersectClipRect      = gdi32.NewProc("IntersectClipRect")

	procGetModuleHandle       = kernel32.NewProc("GetModuleHandleW")
	procShellExecuteW         = shell32.NewProc("ShellExecuteW")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
	procSetWindowTheme        = uxtheme.NewProc("SetWindowTheme")
)

func wstr(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

func rgb(r, g, b byte) uint32 { return uint32(r) | uint32(g)<<8 | uint32(b)<<16 }

func loWord(v uintptr) uint16 { return uint16(v & 0xffff) }

func hiWord(v uintptr) uint16 { return uint16((v >> 16) & 0xffff) }

func signedWord(v uintptr) int32 { return int32(int16(v & 0xffff)) }

func send(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := procSendMessage.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func setText(hwnd syscall.Handle, s string) {
	procSetWindowText.Call(uintptr(hwnd), uintptr(unsafe.Pointer(wstr(s))))
}
