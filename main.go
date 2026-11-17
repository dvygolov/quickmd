//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	ID_SRC         = 1001
	ID_PREV        = 1002
	TIMER_AUTOSAVE = 2
	TIMER_CLIPRESTORE = 4
	TIMER_PREVIEW  = 3
	VK_CONTROL     = 0x11
	VK_MENU        = 0x12
	CF_UNICODETEXT = 13
)

var (
	pendingClip    clipState
	hasPendingClip bool

	hwndMain uintptr
	hSrc     uintptr
	hPrev    uintptr

	wndProcPtr  uintptr
	editProcPtr uintptr
	oldEditProc uintptr

	filePath string
	isDirty  bool
	lastSeen   string
	savedText  string
	dirtySince time.Time

	classNamePtr *uint16
	hInstance    uintptr
	logFile      *os.File
	rtfFormat    uintptr
)

func messageBox(text string) {
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(text))), uintptr(unsafe.Pointer(utf16Ptr("QuickMD"))), 0x10)
}

func dbg(format string, args ...any) {
	if logFile != nil {
		fmt.Fprintf(logFile, format+"\n", args...)
	}
}

func main() {
	runtime.LockOSThread()
	t0 := time.Now()

	exePath, _ := os.Executable()
	if f, err := os.Create(filepath.Join(filepath.Dir(exePath), "run.log")); err == nil {
		logFile = f
		defer f.Close()
	}

	if len(os.Args) > 1 {
		filePath = os.Args[1]
	}

	if err := msftedit.Load(); err != nil {
		messageBox("Не удалось загрузить RichEdit (Msftedit.dll).\n" + err.Error())
		return
	}
	setDPIAware()

	hInstance = getModuleHandle()
	classNamePtr = utf16Ptr("QuickMD")

	wndProcPtr = syscall.NewCallback(wndProc)
	editProcPtr = syscall.NewCallback(editWndProc)

	cursor, _, _ := pLoadCursorW.Call(0, IDC_IBEAM)
	icon, _, _ := pLoadIconW.Call(hInstance, 1)
	if icon == 0 {
		icon, _, _ = pLoadIconW.Call(0, IDI_APPLICATION)
	}
	brush, _, _ := pCreateSolidBrush.Call(rgb(24, 24, 26))

	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   wndProcPtr,
		HInstance:     hInstance,
		HCursor:       cursor,
		HIcon:         icon,
		HbrBackground: brush,
		LpszClassName: classNamePtr,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		messageBox("RegisterClassExW: " + err.Error())
		return
	}

	sw, _, _ := pGetSystemMetrics.Call(0)
	sh, _, _ := pGetSystemMetrics.Call(1)
	w, h := sw*3/4, sh*4/5
	x, y := (sw-w)/2, (sh-h)/3

	hwnd, _, err := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(utf16Ptr("QuickMD"))),
		uintptr(WS_OVERLAPPEDWINDOW),
		x, y, w, h,
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		messageBox("CreateWindowExW: " + err.Error())
		return
	}
	hwndMain = hwnd

	sendMsg(hwnd, WM_SETICON, 1, icon)
	sendMsg(hwnd, WM_SETICON, 0, icon)
	pShowWindow.Call(hwnd, SW_SHOW)
	pUpdateWindow.Call(hwnd)
	focusWindow(hwnd)

	setup()
	updateTitle()
	dbg("window ready in %d ms", time.Since(t0).Milliseconds())

	var msg MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func focusWindow(hwnd uintptr) {
	fg, _, _ := pGetForegroundWindow.Call()
	myThread, _, _ := pGetCurrentThreadId.Call()
	var fgThread uintptr
	if fg != 0 {
		t, _, _ := pGetWindowThreadProcessId.Call(fg, 0)
		fgThread = t
	}
	attached := false
	if fgThread != 0 && fgThread != myThread {
		if r, _, _ := pAttachThreadInput.Call(myThread, fgThread, 1); r != 0 {
			attached = true
		}
	}
	pKeybdEvent.Call(VK_MENU, 0, 0, 0)
	pSetForegroundWindow.Call(hwnd)
	pBringWindowToTop.Call(hwnd)
	pSwitchToThisWindow.Call(hwnd, 1)
	pKeybdEvent.Call(VK_MENU, 0, 2, 0)
	raiseWindow(hwnd)
	if attached {
		pAttachThreadInput.Call(myThread, fgThread, 0)
	}
}

func raiseWindow(hwnd uintptr) {
	setTopmost(hwnd, true)
}

func setTopmost(hwnd uintptr, on bool) {
	const (
		swpNoSize     = 0x0001
		swpNoMove     = 0x0002
		swpShowWindow = 0x0040
	)
	after := ^uintptr(1) // HWND_NOTOPMOST
	if on {
		after = ^uintptr(0) // HWND_TOPMOST
	}
	pSetWindowPos.Call(hwnd, after, 0, 0, 0, 0, uintptr(swpNoSize|swpNoMove|swpShowWindow))
}

func setDPIAware() {
	if r, _, _ := pSetProcessDpiAwarenessContext.Call(^uintptr(3)); r != 0 { // PER_MONITOR_AWARE_V2 = -4
		return
	}
	pSetProcessDPIAware.Call()
}

func setup() {
	dpi, _, _ := pGetDpiForWindow.Call(hwndMain)
	if dpi < 96 {
		dpi = 96
	}
	pt := int(11 * dpi / 72)

	fontMono, _, _ := pCreateFontW.Call(uintptr(-pt), 0, 0, 0, FW_NORMAL, 0, 0, 0,
		DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, 0, uintptr(unsafe.Pointer(utf16Ptr("Consolas"))))
	fontUI, _, _ := pCreateFontW.Call(uintptr(-pt), 0, 0, 0, FW_NORMAL, 0, 0, 0,
		DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, 0, uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))

	sendMsg(hSrc, WM_SETFONT, fontMono, 1)
	sendMsg(hPrev, WM_SETFONT, fontUI, 1)

	setDefaultFormat(hSrc, "Consolas", pt, rgb(212, 212, 212))
	setDefaultFormat(hPrev, "Segoe UI", pt, rgb(214, 214, 214))

	dark := rgb(24, 24, 26)
	sendMsg(hSrc, EM_SETBKGNDCOLOR, 0, dark)
	sendMsg(hPrev, EM_SETBKGNDCOLOR, 0, dark)

	sendMsg(hSrc, EM_AUTOURLDETECT, 0, 0)
	sendMsg(hSrc, EM_SETOPTIONS, ECO_AUTOVSCROLL, ECO_AUTOVSCROLL)
	sendMsg(hSrc, EM_SETEVENTMASK, ENM_CHANGE, 0)
	sendMsg(hPrev, EM_SETEVENTMASK, 0, 0)

	oldEditProc = setWindowLongPtr(hSrc, GWLP_WNDPROC, editProcPtr)
	pSetTimer.Call(hwndMain, 9, 300, 0)

	name, _, _ := pRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(utf16Ptr("Rich Text Format"))))
	rtfFormat = name

	loadFile()
	renderPreview()

	pSetFocus.Call(hSrc)
	focusWindow(hwndMain)
	pSetTimer.Call(hwndMain, 6, 250, 0)
}

func setDefaultFormat(h uintptr, face string, pt int, color uintptr) {
	var cf CHARFORMAT2W
	cf.CbSize = uint32(unsafe.Sizeof(cf))
	cf.DwMask = CFM_FACE | CFM_SIZE | CFM_COLOR | CFM_CHARSET | CFM_WEIGHT
	cf.YHeight = int32(pt * 20 * 72 / 96)
	cf.CrTextColor = uint32(color)
	cf.WWeight = FW_NORMAL
	cf.BCharSet = 204
	face16 := syscall.StringToUTF16(face)
	copy(cf.SzFaceName[:], face16)
	sendMsg(h, EM_SETCHARFORMAT, SCF_ALL, uintptr(unsafe.Pointer(&cf)))
}

func createRichEdit(parent uintptr, id int, readonly bool) uintptr {
	style := uint32(WS_CHILD | WS_VISIBLE | WS_VSCROLL | ES_MULTILINE | ES_AUTOVSCROLL | ES_WANTRETURN | ES_NOHIDESEL)
	if readonly {
		style |= ES_READONLY
	}
	h, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16Ptr("RICHEDIT50W"))),
		0,
		uintptr(style),
		0, 0, 10, 10,
		parent, uintptr(id), hInstance, 0,
	)
	return h
}

func wndProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		hwndMain = hwnd
		hSrc = createRichEdit(hwnd, ID_SRC, false)
		hPrev = createRichEdit(hwnd, ID_PREV, false)
		layout(hwnd)
		return 0

	case WM_SIZE:
		layout(hwnd)
		return 0

	case WM_ACTIVATE:
		if wp&0xFFFF == 0 {
			setTopmost(hwnd, false)
		} else {
			setTopmost(hwnd, true)
		}
		return 0

	case WM_SETFOCUS:
		pSetFocus.Call(hSrc)
		return 0

	case WM_TIMER:
		switch wp {
		case 6:
			pKillTimer.Call(hwnd, 6)
			focusWindow(hwnd)
		case 9:
			pollText()
		case TIMER_CLIPRESTORE:
			pKillTimer.Call(hwnd, TIMER_CLIPRESTORE)
			if hasPendingClip {
				clipboardRestoreText(pendingClip)
				hasPendingClip = false
			}
		}
		return 0

	case WM_CLOSE:
		if isDirty {
			saveFile()
		}
		pDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		pPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func editWndProc(hwnd, msg, wp, lp uintptr) uintptr {
	if msg == WM_KEYDOWN && wp == 'S' {
		state, _, _ := pGetKeyState.Call(VK_CONTROL)
		if uint16(state)&0x8000 != 0 {
			saveFile()
			return 0
		}
	}
	r, _, _ := pCallWindowProcW.Call(oldEditProc, hwnd, msg, wp, lp)
	return r
}

func layout(hwnd uintptr) {
	var rc RECT
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	w := int32(rc.Right - rc.Left)
	h := int32(rc.Bottom - rc.Top)
	if w <= 0 || h <= 0 {
		return
	}
	gap := int32(6)
	left := (w - gap) / 2
	pMoveWindow.Call(hSrc, 0, 0, uintptr(left), uintptr(h), 1)
	pMoveWindow.Call(hPrev, uintptr(left+gap), 0, uintptr(w-left-gap), uintptr(h), 1)
	setMargins(hSrc, left, h)
	setMargins(hPrev, w-left-gap, h)
}

func setMargins(hwnd uintptr, width, height int32) {
	rc := RECT{Left: 16, Top: 12, Right: width - 16, Bottom: height - 12}
	sendMsg(hwnd, EM_SETRECT, 0, uintptr(unsafe.Pointer(&rc)))
}

func loadFile() {
	if filePath == "" {
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		messageBox("Не удалось открыть файл:\n" + err.Error())
		return
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	setText(hSrc, text)
	lastSeen = text
	savedText = text
	isDirty = false
}

func setText(h uintptr, s string) {
	text := strings.ReplaceAll(s, "\n", "\r\n")
	p := syscall.StringToUTF16Ptr(text)
	sendMsg(h, WM_SETTEXT, 0, uintptr(unsafe.Pointer(p)))
}

func getText(h uintptr) string {
	lt := GETTEXTLENGTHEX{Flags: 8, Codepage: 1200} // GTL_NUMCHARS
	n, _, _ := pSendMessageW.Call(h, uintptr(EM_GETTEXTLENGTHEX), uintptr(unsafe.Pointer(&lt)), 0)
	if n < 0 {
		n = 0
	}
	buf := make([]uint16, n+4)
	gt := GETTEXTEX{Cb: uint32(len(buf) * 2), Flags: 0, Codepage: 1200}
	sendMsg(h, EM_GETTEXTEX, uintptr(unsafe.Pointer(&gt)), uintptr(unsafe.Pointer(&buf[0])))
	s := syscall.UTF16ToString(buf)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func saveFile() {
	if filePath == "" {
		return
	}
	text := getText(hSrc)
	tmp := filePath + ".quickmd.tmp"
	if err := os.WriteFile(tmp, []byte(text), 0644); err != nil {
		dbg("write tmp failed: %v", err)
		messageBox("Не удалось сохранить файл:\n" + err.Error())
		return
	}
	if err := os.Rename(tmp, filePath); err != nil {
		dbg("rename failed: %v", err)
		messageBox("Не удалось сохранить файл:\n" + err.Error())
		os.Remove(tmp)
		return
	}
	isDirty = false
	savedText = text
	updateTitle()
}

func pollText() {
	txt := getText(hSrc)
	if txt != lastSeen {
		lastSeen = txt
		dirtySince = time.Now()
		if !isDirty {
			isDirty = true
			updateTitle()
		}
		renderPreview()
	}
	if txt != savedText && time.Since(dirtySince) > 700*time.Millisecond {
		saveFile()
	}
}

func updateTitle() {
	name := "Без имени"
	if filePath != "" {
		name = filepath.Base(filePath)
	}
	mark := ""
	if isDirty {
		mark = " *"
	}
	pSetWindowTextW.Call(hwndMain, uintptr(unsafe.Pointer(utf16Ptr(name+mark+" — QuickMD"))))
}

func renderPreview() {
	src := getText(hSrc)
	rtf := renderRTF(parseMarkdown(src))
	setRTF(hPrev, [][]byte{rtf})
}

func setRTF(h uintptr, chunks [][]byte) {
	first, _, _ := pSendMessageW.Call(h, uintptr(EM_GETFIRSTVISIBLELINE), 0, 0)
	pSendMessageW.Call(h, WM_SETREDRAW, 0, 0)
	restore := clipboardTakeText()

	sendMsg(h, EM_SETREADONLY, 0, 0)
	sendMsg(h, EM_SETSEL, 0, 0xFFFFFFFF)
	for i, data := range chunks {
		if !clipboardSetRTF(data) {
			break
		}
		if i > 0 {
			sendMsg(h, EM_SETSEL, ^uintptr(0), ^uintptr(0))
		}
		pSendMessageW.Call(h, WM_PASTE, 0, 0)
	}
	sendMsg(h, EM_SETREADONLY, 1, 0)
	if restore.ok {
		pendingClip = restore
		hasPendingClip = true
		pSetTimer.Call(hwndMain, TIMER_CLIPRESTORE, 600, 0)
	}

	pSendMessageW.Call(h, WM_SETREDRAW, 1, 0)

	cur, _, _ := pSendMessageW.Call(h, uintptr(EM_GETFIRSTVISIBLELINE), 0, 0)
	if delta := int32(first) - int32(cur); delta != 0 {
		pSendMessageW.Call(h, uintptr(EM_LINESCROLL), 0, uintptr(int64(delta)))
	}
	pInvalidateRect.Call(h, 0, 1)
}

type clipState struct {
	text string
	ok   bool
}

func clipboardTakeText() clipState {
	if ok, _, _ := pIsClipboardFormatAvailable.Call(CF_UNICODETEXT); ok == 0 {
		return clipState{}
	}
	if ok, _, _ := pOpenClipboard.Call(hwndMain); ok == 0 {
		return clipState{}
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(CF_UNICODETEXT)
	if h == 0 {
		return clipState{}
	}
	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return clipState{}
	}
	defer pGlobalUnlock.Call(h)
	return clipState{text: utf16PtrString((*uint16)(unsafe.Pointer(ptr))), ok: true}
}

func clipboardRestoreText(state clipState) {
	if !state.ok {
		return
	}
	utf := syscall.StringToUTF16(state.text)
	if ok, _, _ := pOpenClipboard.Call(hwndMain); ok == 0 {
		return
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	mem, _, _ := pGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(len(utf)*2))
	if mem == 0 {
		return
	}
	ptr, _, _ := pGlobalLock.Call(mem)
	if ptr == 0 {
		pGlobalFree.Call(mem)
		return
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(ptr)), len(utf)), utf)
	pGlobalUnlock.Call(mem)
	if r, _, _ := pSetClipboardData.Call(CF_UNICODETEXT, mem); r == 0 {
		pGlobalFree.Call(mem)
	}
}

func clipboardSetRTF(rtf []byte) bool {
	if ok, _, _ := pOpenClipboard.Call(hwndMain); ok == 0 {
		return false
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()

	const pad = 8192
	mem, _, _ := pGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(len(rtf)+1+pad))
	if mem == 0 {
		return false
	}
	ptr, _, _ := pGlobalLock.Call(mem)
	if ptr == 0 {
		pGlobalFree.Call(mem)
		return false
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(rtf)+1+pad)
	for i := range dst {
		dst[i] = 0
	}
	copy(dst, rtf)
	pGlobalUnlock.Call(mem)

	if r, _, _ := pSetClipboardData.Call(rtfFormat, mem); r == 0 {
		pGlobalFree.Call(mem)
		return false
	}
	return true
}

func clipboardRTFSize() int {
	if ok, _, _ := pOpenClipboard.Call(hwndMain); ok == 0 {
		return -1
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(rtfFormat)
	if h == 0 {
		return -2
	}
	n, _, _ := pGlobalSize.Call(h)
	return int(n)
}

func utf16PtrString(p *uint16) string {
	if p == nil {
		return ""
	}
	var buf []uint16
	for i := 0; i < 1<<20; i++ {
		v := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(i*2)))
		if v == 0 {
			break
		}
		buf = append(buf, v)
	}
	return syscall.UTF16ToString(buf)
}

func setWindowLongPtr(h uintptr, index uintptr, value uintptr) uintptr {
	r, _, _ := pSetWindowLongPtrW.Call(h, index, value)
	return r
}
