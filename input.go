//go:build windows

package main

import (
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	idLinkOK     = 1
	idLinkCancel = 2
	idLinkEdit   = 100
)

var (
	inputProcPtr uintptr
	inputHwnd    uintptr
	inputEdit    uintptr
	inputDone    bool
	inputOK      bool
	inputText    string
)

func inputWndProc(hwnd, msg, wp, lp uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		inputHwnd = hwnd
		label, _, _ := pCreateWindowExW.Call(0,
			uintptr(unsafe.Pointer(utf16Ptr("STATIC"))),
			uintptr(unsafe.Pointer(utf16Ptr("URL ссылки:"))),
			uintptr(WS_CHILD|WS_VISIBLE),
			16, 14, 460, 20,
			hwnd, 0, hInstance, 0)
		edit, _, _ := pCreateWindowExW.Call(WS_EX_CLIENTEDGE,
			uintptr(unsafe.Pointer(utf16Ptr("EDIT"))),
			uintptr(unsafe.Pointer(utf16Ptr(""))),
			uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|ES_AUTOHSCROLL),
			16, 40, 460, 26,
			hwnd, idLinkEdit, hInstance, 0)
		inputEdit = edit
		okBtn, _, _ := pCreateWindowExW.Call(0,
			uintptr(unsafe.Pointer(utf16Ptr("BUTTON"))),
			uintptr(unsafe.Pointer(utf16Ptr("OK"))),
			uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_DEFPUSHBUTTON),
			270, 82, 100, 30,
			hwnd, idLinkOK, hInstance, 0)
		cancelBtn, _, _ := pCreateWindowExW.Call(0,
			uintptr(unsafe.Pointer(utf16Ptr("BUTTON"))),
			uintptr(unsafe.Pointer(utf16Ptr("Отмена"))),
			uintptr(WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON),
			376, 82, 100, 30,
			hwnd, idLinkCancel, hInstance, 0)
		if uiFont != 0 {
			for _, c := range []uintptr{label, edit, okBtn, cancelBtn} {
				sendMsg(c, WM_SETFONT, uiFont, 1)
			}
		}
		sendMsg(inputEdit, EM_SETSEL, 0, ^uintptr(0))
		pSetFocus.Call(inputEdit)
		return 0

	case WM_COMMAND:
		switch int(wp & 0xFFFF) {
		case idLinkOK:
			inputText = getWindowText(inputEdit)
			inputOK = true
			pDestroyWindow.Call(hwnd)
		case idLinkCancel:
			pDestroyWindow.Call(hwnd)
		}
		return 0

	case WM_CLOSE:
		pDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		inputDone = true
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func getWindowText(h uintptr) string {
	n, _, _ := pGetWindowTextLengthW.Call(h)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, int(n)+1)
	pGetWindowTextW.Call(h, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	runtime.KeepAlive(buf)
	return syscall.UTF16ToString(buf)
}

func setWindowText(h uintptr, s string) {
	p := utf16Ptr(s)
	sendMsg(h, WM_SETTEXT, 0, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
}

// promptText shows a small modal input box and returns the entered text.
func promptText(title, label, initial string) (string, bool) {
	if inputProcPtr == 0 {
		inputProcPtr = syscall.NewCallback(inputWndProc)
		className := utf16Ptr("QuickMDInput")
		var wc WNDCLASSEX
		wc.CbSize = uint32(unsafe.Sizeof(wc))
		wc.LpfnWndProc = inputProcPtr
		wc.HInstance = hInstance
		wc.HbrBackground = 16 // COLOR_BTNFACE + 1
		wc.LpszClassName = className
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		runtime.KeepAlive(className)
	}

	var rc RECT
	pGetWindowRect.Call(hwndMain, uintptr(unsafe.Pointer(&rc)))
	w, h := 520, 170
	x := int(rc.Left) + (int(rc.Right-rc.Left)-w)/2
	y := int(rc.Top) + (int(rc.Bottom-rc.Top)-h)/2

	inputDone = false
	inputOK = false
	inputText = ""

	titlePtr := utf16Ptr(title)
	classPtr := utf16Ptr("QuickMDInput")
	dlg, _, _ := pCreateWindowExW.Call(
		WS_EX_DLGMODALFRAME|WS_EX_CONTROLPARENT,
		uintptr(unsafe.Pointer(classPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(WS_POPUP|WS_CAPTION|WS_SYSMENU|WS_VISIBLE),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		hwndMain, 0, hInstance, 0,
	)
	runtime.KeepAlive(titlePtr)
	runtime.KeepAlive(classPtr)
	if dlg == 0 {
		return "", false
	}
	if initial != "" && inputEdit != 0 {
		setWindowText(inputEdit, initial)
		sendMsg(inputEdit, EM_SETSEL, 0, ^uintptr(0))
	}

	pEnableWindow.Call(hwndMain, 0)
	pShowWindow.Call(dlg, SW_SHOW)
	pUpdateWindow.Call(dlg)
	pSetForegroundWindow.Call(dlg)
	pSetFocus.Call(inputEdit)

	var msg MSG
	for !inputDone {
		for {
			r, _, _ := pPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, PM_REMOVE)
			if r == 0 {
				break
			}
			if ok, _, _ := pIsDialogMessageW.Call(dlg, uintptr(unsafe.Pointer(&msg))); ok != 0 {
				continue
			}
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
			pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
		}
		if inputDone {
			break
		}
		pWaitMessage.Call()
	}

	pEnableWindow.Call(hwndMain, 1)
	pSetForegroundWindow.Call(hwndMain)
	return inputText, inputOK
}

// insertLink wraps the current selection into a Markdown link after asking for
// the URL.
func insertLink(h uintptr) {
	var cr CHARRANGE
	sendMsg(h, EM_EXGETSEL, 0, uintptr(unsafe.Pointer(&cr)))
	start, end := int(cr.CpMin), int(cr.CpMax)
	if start > end {
		start, end = end, start
	}
	text := getTextU16(h)
	selected := ""
	if start < end && end <= len(text) {
		selected = syscall.UTF16ToString(text[start:end])
		selected = strings.ReplaceAll(selected, "\r\n", " ")
		selected = strings.ReplaceAll(selected, "\r", " ")
		selected = strings.ReplaceAll(selected, "\n", " ")
		selected = strings.TrimSpace(selected)
	}

	url, ok := promptText("Вставить ссылку", "URL:", "")
	if !ok {
		return
	}
	url = strings.TrimSpace(url)
	if url == "" {
		return
	}
	if !strings.Contains(url, "://") && !strings.HasPrefix(url, "/") &&
		!strings.HasPrefix(url, "#") && !strings.HasPrefix(url, "mailto:") {
		url = "https://" + url
	}

	var insert string
	var caret int
	if selected == "" {
		insert = "[](" + url + ")"
		caret = start + 1
	} else {
		insert = "[" + selected + "](" + url + ")"
		caret = start + utf16LenOf(insert)
	}

	setSelRange(int32(start), int32(end))
	replaceBuf = syscall.StringToUTF16(insert)
	sendMsg(h, EM_REPLACESEL, 1, uintptr(unsafe.Pointer(&replaceBuf[0])))
	runtime.KeepAlive(replaceBuf)
	setSelRange(int32(caret), int32(caret))
	renderPreview()
}
