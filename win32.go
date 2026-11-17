//go:build windows

package main

import (
	"syscall"
)

// DLLs ---------------------------------------------------------------------

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	msftedit = syscall.NewLazyDLL("Msftedit.dll")
)

var (
	pRegisterClassExW   = user32.NewProc("RegisterClassExW")
	pCreateWindowExW    = user32.NewProc("CreateWindowExW")
	pDefWindowProcW     = user32.NewProc("DefWindowProcW")
	pDestroyWindow      = user32.NewProc("DestroyWindow")
	pShowWindow         = user32.NewProc("ShowWindow")
	pUpdateWindow       = user32.NewProc("UpdateWindow")
	pGetMessageW        = user32.NewProc("GetMessageW")
	pTranslateMessage   = user32.NewProc("TranslateMessage")
	pDispatchMessageW   = user32.NewProc("DispatchMessageW")
	pPostQuitMessage    = user32.NewProc("PostQuitMessage")
	pSendMessageW       = user32.NewProc("SendMessageW")
	pSetWindowLongPtrW  = user32.NewProc("SetWindowLongPtrW")
	pCallWindowProcW    = user32.NewProc("CallWindowProcW")
	pSetTimer           = user32.NewProc("SetTimer")
	pKillTimer          = user32.NewProc("KillTimer")
	pGetClientRect      = user32.NewProc("GetClientRect")
	pGetWindowRect      = user32.NewProc("GetWindowRect")
	pGetScrollInfo      = user32.NewProc("GetScrollInfo")
	pMoveWindow         = user32.NewProc("MoveWindow")
	pGetKeyState        = user32.NewProc("GetKeyState")
	pSetFocus           = user32.NewProc("SetFocus")
	pPostMessage        = user32.NewProc("PostMessageW")
	pLoadCursorW        = user32.NewProc("LoadCursorW")
	pLoadIconW          = user32.NewProc("LoadIconW")
	pMessageBoxW        = user32.NewProc("MessageBoxW")
	pSetProcessDPIAware = user32.NewProc("SetProcessDPIAware")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pBringWindowToTop  = user32.NewProc("BringWindowToTop")
	pGetClipboardData  = user32.NewProc("GetClipboardData")
	pGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	pGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	pAttachThreadInput = user32.NewProc("AttachThreadInput")
	pSwitchToThisWindow = user32.NewProc("SwitchToThisWindow")
	pGetCurrentThreadId = kernel32.NewProc("GetCurrentThreadId")
	pKeybdEvent        = user32.NewProc("keybd_event")
	pSetWindowPos      = user32.NewProc("SetWindowPos")
	pSetCursorPos      = user32.NewProc("SetCursorPos")
	pMouseEvent        = user32.NewProc("mouse_event")
	pGlobalFree        = kernel32.NewProc("GlobalFree")
	pGlobalSize        = kernel32.NewProc("GlobalSize")
	pSetWindowTextW    = user32.NewProc("SetWindowTextW")
	pGetSystemMetrics  = user32.NewProc("GetSystemMetrics")
	pGetDpiForWindow   = user32.NewProc("GetDpiForWindow")
	pInvalidateRect    = user32.NewProc("InvalidateRect")
	pEnumWindows       = user32.NewProc("EnumWindows")
	pEnumPropsExW      = user32.NewProc("EnumPropsExW")
	pOpenClipboard     = user32.NewProc("OpenClipboard")
	pEmptyClipboard    = user32.NewProc("EmptyClipboard")
	pCloseClipboard    = user32.NewProc("CloseClipboard")
	pSetClipboardData  = user32.NewProc("SetClipboardData")
	pRegisterClipboardFormatW = user32.NewProc("RegisterClipboardFormatW")
	pIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	pGlobalAlloc       = kernel32.NewProc("GlobalAlloc")
	pGlobalLock        = kernel32.NewProc("GlobalLock")
	pGlobalUnlock      = kernel32.NewProc("GlobalUnlock")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	pCreateFontW      = gdi32.NewProc("CreateFontW")
	pCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
)

// Constants ----------------------------------------------------------------

const (
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_CHILD            = 0x40000000
	WS_VISIBLE          = 0x10000000
	WS_VSCROLL          = 0x00200000
	WS_TABSTOP          = 0x00010000

	ES_MULTILINE   = 0x0004
	ES_AUTOVSCROLL = 0x0040
	ES_READONLY    = 0x0800
	ES_WANTRETURN  = 0x1000
	ES_NOHIDESEL   = 0x0100

	WM_CREATE    = 0x0001
	WM_DESTROY   = 0x0002
	WM_SIZE      = 0x0005
	WM_SETFOCUS  = 0x0007
	WM_ACTIVATE   = 0x0006
	WM_NOTIFY     = 0x004E
	WM_CHAR       = 0x0102
	WM_VSCROLL    = 0x0115
	WM_MOUSEWHEEL = 0x020A
	WM_KEYUP      = 0x0101
	WM_SETICON    = 0x0080
	WM_CLOSE     = 0x0010
	WM_SETTEXT   = 0x000C
	WM_GETTEXT   = 0x000D
	WM_KEYDOWN   = 0x0100
	WM_TIMER     = 0x0113
	WM_COMMAND   = 0x0111
	WM_SETFONT   = 0x0030
	WM_SETREDRAW = 0x000B
	WM_GETDLGCODE = 0x0087

	EN_CHANGE = 0x0300
	ENM_CHANGE = 0x00000001
	EN_UPDATE = 0x0400

	EM_GETSEL             = 0x00B0
	EM_LINESCROLL         = 0x00B6
	EM_SETREADONLY        = 0x0400 + 31
	EM_SETCHARFORMAT      = 0x0400 + 68
	EM_SETBKGNDCOLOR      = 0x0400 + 67
	EM_SETEVENTMASK       = 0x0400 + 69
	EM_GETEVENTMASK       = 0x0400 + 70
	EM_GETTEXTEX          = 0x0400 + 94
	EM_GETTEXTLENGTHEX    = 0x0400 + 95
	EM_STREAMIN           = 0x0400 + 73
	EM_SETTARGETDEVICE    = 0x0400 + 72
	EM_SETOPTIONS         = 0x0400 + 77
	EM_GETOPTIONS         = 0x0400 + 78
	EM_GETFIRSTVISIBLELINE = 0x00CE
	EM_SETTEXTEX          = 0x0400 + 97
	EM_AUTOURLDETECT      = 0x0400 + 91
	EM_EXSETSEL           = 0x0400 + 55
	EM_SETSEL             = 0x00B1
	EM_SETRECT            = 0x00B3
	EM_SETUNDOLIMIT       = 0x0400 + 82
	EM_SETPARAFORMAT      = 0x0400 + 71
	EM_LINEFROMCHAR       = 0x00C9
	EM_GETLINECOUNT       = 0x00BA
	EM_EXGETSEL           = 0x0400 + 52
	EM_REPLACESEL         = 0x00C2
	WM_PASTE              = 0x0302
	GMEM_MOVEABLE         = 0x0042

	SCF_SELECTION = 1
	SCF_WORD      = 2
	SCF_ALL       = 4

	CFM_BOLD     = 0x00000001
	CFM_ITALIC   = 0x00000002
	CFM_UNDERLINE = 0x00000004
	CFM_STRIKEOUT = 0x00000008
	CFM_FACE     = 0x20000000
	CFM_SIZE     = 0x80000000
	CFM_COLOR    = 0x40000000
	CFM_CHARSET  = 0x08000000
	CFM_WEIGHT   = 0x00400000
	CFM_BACKCOLOR = 0x04000000

	CFE_BOLD      = 0x0001
	CFE_ITALIC    = 0x0002
	CFE_UNDERLINE = 0x0004
	CFE_STRIKEOUT = 0x0008

	ECO_AUTOVSCROLL       = 0x00000040
	ECO_AUTOWORDSELECTION = 0x00000001

	GWLP_WNDPROC = ^uintptr(3) // -4

	SW_SHOW = 5

	IDC_IBEAM       = 32513
	IDI_APPLICATION = 32512

	FW_NORMAL       = 400
	DEFAULT_CHARSET = 1
	CLEARTYPE_QUALITY = 5
	FIXED_PITCH     = 1
	FF_MODERN       = 48
	FF_SWISS        = 32

	ERROR_SUCCESS = 0
)

// Structs ------------------------------------------------------------------

type RECT struct {
	Left, Top, Right, Bottom int32
}

type POINT struct {
	X, Y int32
}

type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type MSG struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       POINT
	LPrivate uint32
}

type CHARFORMAT2W struct {
	CbSize          uint32
	DwMask          uint32
	DwEffects       uint32
	YHeight         int32
	YOffset         int32
	CrTextColor     uint32
	BCharSet        uint8
	BPitchAndFamily uint8
	SzFaceName      [32]uint16
	WWeight         uint16
	SSpacing        int16
	CrBackColor     uint32
	Lcid            uint32
	DwReserved      uint32
	SStyle          int16
	WKerning        uint16
	BUnderlineType  uint8
	BAnimation      uint8
	BRevAuthor      uint8
	BReserved1      uint8
}

type SCROLLINFO struct {
	CbSize    uint32
	FMask     uint32
	NMin      int32
	NMax      int32
	NPage     uint32
	NPos      int32
	NTrackPos int32
}

type PARAFORMAT2 struct {
	CbSize          uint32
	DwMask          uint32
	WNumbering      uint16
	WEffects        uint16
	DxStartIndent   int32
	DxRightIndent   int32
	DxOffset        int32
	WAlignment      uint16
	CTabCount       int16
	RgxTabs         [32]int32
	DySpaceBefore   int32
	DySpaceAfter    int32
	DyLineSpacing   int32
	SStyle          int16
	BLineSpacingRule uint8
	BOutlineLevel   uint8
	WShadingWeight  uint16
	WShadingStyle   uint16
	WNumberingStart uint16
	WNumberingStyle uint16
	WNumberingTab   uint16
	WBorderSpace    uint16
	WBorderWidth    uint16
	WBorders        uint16
}

type CHARRANGE struct {
	CpMin int32
	CpMax int32
}

type GETTEXTLENGTHEX struct {
	Flags    uint32
	Codepage uint32
}

type GETTEXTEX struct {
	Cb            uint32
	Flags         uint32
	Codepage      uint32
	LpDefaultChar *byte
	LpUsedDefChar *byte
}

type EDITSTREAM struct {
	DwCookie    uintptr
	DwError     uint32
	PfnCallback uintptr
}

// Helpers ------------------------------------------------------------------

func rgb(r, g, b byte) uintptr {
	return uintptr(r) | uintptr(g)<<8 | uintptr(b)<<16
}

func sendMsg(h uintptr, msg uint32, w, l uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(h, uintptr(msg), w, l)
	return r
}

func getModuleHandle() uintptr {
	h, _, _ := pGetModuleHandleW.Call(0)
	return h
}

func loadLibrary(name string) error {
	return syscall.NewLazyDLL(name).Load()
}

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}
