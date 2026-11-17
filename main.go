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
	VK_CONTROL = 0x11
	VK_SHIFT   = 0x10
	VK_MENU    = 0x12

	TIMER_POLL        = 9
	TIMER_FOCUS       = 6
	TIMER_CLIPRESTORE = 4
	CF_UNICODETEXT = 13
)

type snapshot struct {
	text string
	sel  CHARRANGE
}

type clipState struct {
	text string
	ok   bool
}

var (
	docDir string

	hwndMain uintptr
	hCode    uintptr
	hPreview uintptr

	wndProcPtr  uintptr
	editProcPtr uintptr
	oldEditProc uintptr

	filePath   string
	isDirty    bool
	lastSeen   string
	savedText  string
	dirtySince time.Time
	lastEdit   time.Time

	undoStack []snapshot
	redoStack []snapshot

	rtfFormat uintptr
	pendingClip clipState
	hasPendingClip bool

	classNamePtr *uint16
	hInstance    uintptr
	logFile      *os.File
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
		messageBox("Не удалось загрузить RichEdit:\n" + err.Error())
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
	brush, _, _ := pCreateSolidBrush.Call(rgb(30, 30, 32))

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
	if w > 1440 {
		w = 1440
	}
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

func setup() {
	dpi, _, _ := pGetDpiForWindow.Call(hwndMain)
	if dpi < 96 {
		dpi = 96
	}
	monoPt := int(11 * dpi / 72)
	uiPt := int(12 * dpi / 72)

	fontMono, _, _ := pCreateFontW.Call(uintptr(-monoPt), 0, 0, 0, FW_NORMAL, 0, 0, 0,
		DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, 0, uintptr(unsafe.Pointer(utf16Ptr("Consolas"))))
	fontUI, _, _ := pCreateFontW.Call(uintptr(-uiPt), 0, 0, 0, FW_NORMAL, 0, 0, 0,
		DEFAULT_CHARSET, 0, 0, CLEARTYPE_QUALITY, 0, uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))

	sendMsg(hCode, WM_SETFONT, fontMono, 1)
	sendMsg(hPreview, WM_SETFONT, fontUI, 1)
	setDefaultFormat(hCode, "Consolas", monoPt, rgb(212, 212, 212))
	setDefaultFormat(hPreview, "Segoe UI", uiPt, rgb(214, 214, 214))

	dark := rgb(30, 30, 32)
	sendMsg(hCode, EM_SETBKGNDCOLOR, 0, dark)
	sendMsg(hPreview, EM_SETBKGNDCOLOR, 0, dark)

	sendMsg(hCode, EM_AUTOURLDETECT, 0, 0)
	sendMsg(hCode, EM_SETOPTIONS, ECO_AUTOVSCROLL, ECO_AUTOVSCROLL)
	sendMsg(hCode, EM_SETEVENTMASK, ENM_CHANGE, 0)
	sendMsg(hCode, EM_SETUNDOLIMIT, 0, 0)
	sendMsg(hPreview, EM_SETEVENTMASK, 0, 0)

	oldEditProc = setWindowLongPtr(hCode, GWLP_WNDPROC, editProcPtr)

	name, _, _ := pRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(utf16Ptr("Rich Text Format"))))
	rtfFormat = name

	loadFile()
	renderPreview()
	pSetFocus.Call(hCode)
	pSetTimer.Call(hwndMain, TIMER_POLL, 300, 0)
}

func setDefaultFormat(h uintptr, face string, pt int, color uintptr) {
	var cf CHARFORMAT2W
	cf.CbSize = uint32(unsafe.Sizeof(cf))
	cf.DwMask = CFM_FACE | CFM_SIZE | CFM_COLOR | CFM_CHARSET | CFM_WEIGHT
	cf.YHeight = int32(pt * 20 * 72 / 96)
	cf.CrTextColor = uint32(color)
	cf.WWeight = FW_NORMAL
	cf.BCharSet = 204
	copy(cf.SzFaceName[:], syscall.StringToUTF16(face))
	sendMsg(h, EM_SETCHARFORMAT, SCF_ALL, uintptr(unsafe.Pointer(&cf)))
}

func createRichEdit(parent uintptr, id int) uintptr {
	style := uint32(WS_CHILD | WS_VISIBLE | WS_VSCROLL | ES_MULTILINE | ES_AUTOVSCROLL | ES_WANTRETURN | ES_NOHIDESEL)
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
		hCode = createRichEdit(hwnd, 1001)
		hPreview = createRichEdit(hwnd, 1002)
		layout(hwnd)
		return 0

	case WM_SIZE:
		layout(hwnd)
		return 0

	case WM_ACTIVATE:
		setTopmost(hwnd, wp&0xFFFF != 0)
		return 0

	case WM_SETFOCUS:
		pSetFocus.Call(hCode)
		return 0

	case WM_TIMER:
		switch wp {
		case TIMER_POLL:
			pollText()
		case TIMER_FOCUS:
			pKillTimer.Call(hwnd, TIMER_FOCUS)
			focusWindow(hwnd)
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
	if msg == WM_CHAR {
		if pendingChar != 0 {
			if wp == pendingChar {
				pendingChar = 0
				return 0
			}
			pendingChar = 0
		}
	}
	if msg == WM_KEYUP {
		pendingChar = 0
	}
	if msg == WM_KEYDOWN {
		if keyDown(VK_CONTROL) {
			switch wp {
			case 'Z':
				undoEdit()
				return 0
			case 'Y':
				redoEdit()
				return 0
			case 'S':
				saveFile()
				return 0
			}
		}
		if handleFormatShortcut(hwnd, wp) {
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
	pMoveWindow.Call(hCode, 0, 0, uintptr(left), uintptr(h), 1)
	pMoveWindow.Call(hPreview, uintptr(left+gap), 0, uintptr(w-left-gap), uintptr(h), 1)
	setMargins(hCode, left, h)
	setMargins(hPreview, w-left-gap, h)
}

func setMargins(hwnd uintptr, width, height int32) {
	rect := RECT{Left: 18, Top: 14, Right: width - 18, Bottom: height - 14}
	sendMsg(hwnd, EM_SETRECT, 0, uintptr(unsafe.Pointer(&rect)))
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
	setText(hCode, text)
	docDir = filepath.Dir(filePath)
	lastSeen = text
	savedText = text
	lastEdit = time.Now()
	isDirty = false
	undoStack = nil
	redoStack = nil
}

func setText(h uintptr, s string) {
	text := strings.ReplaceAll(s, "\n", "\r\n")
	p := syscall.StringToUTF16Ptr(text)
	sendMsg(h, WM_SETTEXT, 0, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
}

func getText(h uintptr) string {
	lt := GETTEXTLENGTHEX{Flags: 8, Codepage: 1200}
	n, _, _ := pSendMessageW.Call(h, uintptr(EM_GETTEXTLENGTHEX), uintptr(unsafe.Pointer(&lt)), 0)
	if n <= 0 {
		return ""
	}
	buf := make([]uint16, int(n)+16)
	gt := GETTEXTEX{Cb: uint32(len(buf) * 2), Flags: 0, Codepage: 1200}
	copied, _, _ := pSendMessageW.Call(h, uintptr(EM_GETTEXTEX), uintptr(unsafe.Pointer(&gt)), uintptr(unsafe.Pointer(&buf[0])))
	if int(copied) > len(buf) {
		copied = uintptr(len(buf))
	}
	s := syscall.UTF16ToString(buf[:copied])
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

func currentSel() CHARRANGE {
	var cr CHARRANGE
	sendMsg(hCode, EM_EXGETSEL, 0, uintptr(unsafe.Pointer(&cr)))
	return cr
}

func setSelRange(start, end int32) {
	cr := CHARRANGE{CpMin: start, CpMax: end}
	sendMsg(hCode, EM_EXSETSEL, 0, uintptr(unsafe.Pointer(&cr)))
}

func pushUndo(text string) {
	if len(undoStack) >= 100 {
		undoStack = undoStack[1:]
	}
	undoStack = append(undoStack, snapshot{text: text, sel: currentSel()})
	redoStack = nil
}

func undoEdit() {
	if len(undoStack) == 0 {
		return
	}
	cur := snapshot{text: getText(hCode), sel: currentSel()}
	s := undoStack[len(undoStack)-1]
	undoStack = undoStack[:len(undoStack)-1]

	setText(hCode, s.text)
	setSelRange(s.sel.CpMin, s.sel.CpMax)
	lastSeen = s.text
	isDirty = true
	updateTitle()
	renderPreview()
	redoStack = append(redoStack, cur)
}

func redoEdit() {
	if len(redoStack) == 0 {
		return
	}
	cur := snapshot{text: getText(hCode), sel: currentSel()}
	s := redoStack[len(redoStack)-1]
	redoStack = redoStack[:len(redoStack)-1]

	setText(hCode, s.text)
	setSelRange(s.sel.CpMin, s.sel.CpMax)
	lastSeen = s.text
	isDirty = true
	updateTitle()
	renderPreview()
	undoStack = append(undoStack, cur)
}

func saveFile() {
	if filePath == "" {
		return
	}
	text := getText(hCode)
	tmp := filePath + ".quickmd.tmp"
	if err := os.WriteFile(tmp, []byte(text), 0644); err != nil {
		dbg("write failed: %v", err)
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
	txt := getText(hCode)
	if txt != lastSeen {
		if time.Since(lastEdit) > 700*time.Millisecond {
			pushUndo(lastSeen)
		}
		lastEdit = time.Now()
		dirtySince = time.Now()
		lastSeen = txt
		if !isDirty {
			isDirty = true
			updateTitle()
		}
		renderPreview()
	}
	if txt != savedText && time.Since(lastEdit) > 700*time.Millisecond {
		saveFile()
	}
}

func renderPreview() {
	src := getText(hCode)
	rtf := renderRTF(parseMarkdown(src))
	setRTF(hPreview, [][]byte{rtf})
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

func clipboardSetRTF(data []byte) bool {
	if ok, _, _ := pOpenClipboard.Call(hwndMain); ok == 0 {
		return false
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()

	mem, _, _ := pGlobalAlloc.Call(GMEM_MOVEABLE, uintptr(len(data)+1))
	if mem == 0 {
		return false
	}
	ptr, _, _ := pGlobalLock.Call(mem)
	if ptr == 0 {
		pGlobalFree.Call(mem)
		return false
	}
	buf := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), len(data)+1)
	copy(buf, data)
	buf[len(data)] = 0
	pGlobalUnlock.Call(mem)
	if r, _, _ := pSetClipboardData.Call(rtfFormat, mem); r == 0 {
		pGlobalFree.Call(mem)
		return false
	}
	return true
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
	pSetWindowTitle(name + mark + " — QuickMD")
}

func pSetWindowTitle(s string) {
	pSetWindowTextW.Call(hwndMain, uintptr(unsafe.Pointer(utf16Ptr(s))))
}

func utf16PtrString(p *uint16) string {
	if p == nil {
		return ""
	}
	var out []uint16
	for i := 0; i < 1<<20; i++ {
		c := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(i*2)))
		if c == 0 {
			break
		}
		out = append(out, c)
	}
	return string(utf16Decode(out))
}

func utf16Decode(in []uint16) []rune {
	out := make([]rune, 0, len(in))
	for i := 0; i < len(in); i++ {
		c := rune(in[i])
		if c >= 0xD800 && c <= 0xDBFF && i+1 < len(in) {
			c2 := rune(in[i+1])
			if c2 >= 0xDC00 && c2 <= 0xDFFF {
				out = append(out, ((c-0xD800)<<10)+(c2-0xDC00)+0x10000)
				i++
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

func setDPIAware() {
	if r, _, _ := pSetProcessDpiAwarenessContext.Call(^uintptr(3)); r != 0 {
		return
	}
	pSetProcessDPIAware.Call()
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
	setTopmost(hwnd, true)
	if attached {
		pAttachThreadInput.Call(myThread, fgThread, 0)
	}
}

func setTopmost(hwnd uintptr, on bool) {
	const (
		swpNoSize     = 0x0001
		swpNoMove     = 0x0002
		swpShowWindow = 0x0040
	)
	after := ^uintptr(1)
	if on {
		after = ^uintptr(0)
	}
	pSetWindowPos.Call(hwnd, after, 0, 0, 0, 0, uintptr(swpNoSize|swpNoMove|swpShowWindow))
}

func setWindowLongPtr(h uintptr, index uintptr, value uintptr) uintptr {
	r, _, _ := pSetWindowLongPtrW.Call(h, index, value)
	return r
}
