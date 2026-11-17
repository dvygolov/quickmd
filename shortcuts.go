//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

var pendingChar uintptr
var replaceBuf []uint16

// handleFormatShortcut implements Telegram Desktop style formatting shortcuts.
func handleFormatShortcut(hwnd, key uintptr) bool {
	if !keyDown(VK_CONTROL) {
		return false
	}
	shift := keyDown(VK_SHIFT)

	var mark string
	var controlChar uintptr
	switch {
	case !shift && key == 'B':
		mark, controlChar = "**", 0x02
	case !shift && key == 'I':
		mark, controlChar = "_", 0x09
	case !shift && key == 'U':
		mark, controlChar = "__", 0x15
	case shift && key == 'M':
		mark, controlChar = "`", 0x0D
	case shift && key == 'X':
		mark, controlChar = "~~", 0x18
	case shift && key == 'P':
		mark, controlChar = "||", 0x10
	default:
		return false
	}

	pendingChar = controlChar
	toggleMark(hwnd, mark)
	return true
}

func keyDown(vk uintptr) bool {
	state, _, _ := pGetKeyState.Call(vk)
	return uint16(state)&0x8000 != 0
}

func toggleMark(h uintptr, mark string) {
	var cr CHARRANGE
	sendMsg(h, EM_EXGETSEL, 0, uintptr(unsafe.Pointer(&cr)))
	start, end := int(cr.CpMin), int(cr.CpMax)
	if end < start {
		start, end = end, start
	}
	text := getTextU16(h)
	if end > len(text) {
		end = len(text)
	}
	if start > end {
		start = end
	}
	ml := len(mark)

	if start == end {
		insertAt(h, start, mark+mark)
		setSelRange(int32(start+ml), int32(start+ml))
		applyStyles(h, getText(h))
		return
	}

	if end >= start+2*ml && syscall.UTF16ToString(text[start:start+ml]) == mark && syscall.UTF16ToString(text[end-ml:end]) == mark {
		deleteRange(h, end-ml, end)
		deleteRange(h, start, start+ml)
		setSelRange(int32(start), int32(end-2*ml))
		applyStyles(h, getText(h))
		return
	}

	before := start-ml >= 0 && syscall.UTF16ToString(text[start-ml:start]) == mark
	after := end+ml <= len(text) && syscall.UTF16ToString(text[end:end+ml]) == mark
	if before && after {
		deleteRange(h, end, end+ml)
		deleteRange(h, start-ml, start)
		setSelRange(int32(start-ml), int32(end-ml))
		applyStyles(h, getText(h))
		return
	}

	insertAt(h, end, mark)
	insertAt(h, start, mark)
	setSelRange(int32(start+ml), int32(end+ml))
	applyStyles(h, getText(h))
}

func insertAt(h uintptr, pos int, s string) {
	setSelRange(int32(pos), int32(pos))
	replaceBuf = syscall.StringToUTF16(s)
	sendMsg(h, EM_REPLACESEL, 1, uintptr(unsafe.Pointer(&replaceBuf[0])))
	runtime.KeepAlive(replaceBuf)
}

func deleteRange(h uintptr, start, end int) {
	setSelRange(int32(start), int32(end))
	replaceBuf = []uint16{0}
	sendMsg(h, EM_REPLACESEL, 1, uintptr(unsafe.Pointer(&replaceBuf[0])))
	runtime.KeepAlive(replaceBuf)
}

func getTextU16(h uintptr) []uint16 {
	lt := GETTEXTLENGTHEX{Flags: 8, Codepage: 1200}
	n, _, _ := pSendMessageW.Call(h, uintptr(EM_GETTEXTLENGTHEX), uintptr(unsafe.Pointer(&lt)), 0)
	if n <= 0 {
		return nil
	}
	buf := make([]uint16, int(n)+16)
	gt := GETTEXTEX{Cb: uint32(len(buf) * 2), Flags: 0, Codepage: 1200}
	copied, _, _ := pSendMessageW.Call(h, uintptr(EM_GETTEXTEX), uintptr(unsafe.Pointer(&gt)), uintptr(unsafe.Pointer(&buf[0])))
	if int(copied) > len(buf) {
		copied = uintptr(len(buf))
	}
	return buf[:copied]
}
