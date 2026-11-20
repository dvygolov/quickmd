//go:build windows

package main

import (
	"runtime"
	"syscall"
	"unsafe"
)

const (
	ID_FILE_OPEN    = 40001
	ID_FILE_SAVE    = 40002
	ID_FILE_SAVEAS  = 40003
	ID_FILE_EXIT    = 40004
	ID_EDIT_UNDO    = 40010
	ID_EDIT_REDO    = 40011
	ID_EDIT_CUT     = 40012
	ID_EDIT_COPY    = 40013
	ID_EDIT_PASTE   = 40014
	ID_EDIT_SELECT  = 40015
	ID_HELP_ABOUT   = 40020
)

func appendMenuItem(menu uintptr, flags uintptr, id uintptr, text string) {
	pAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

// buildMenu creates the application main menu.
func buildMenu() uintptr {
	file, _, _ := pCreatePopupMenu.Call()
	appendMenuItem(file, MF_STRING, ID_FILE_OPEN, "&Открыть...\tCtrl+O")
	appendMenuItem(file, MF_STRING, ID_FILE_SAVE, "&Сохранить\tCtrl+S")
	appendMenuItem(file, MF_STRING, ID_FILE_SAVEAS, "Сохранить &как...\tCtrl+Shift+S")
	appendMenuItem(file, MF_SEPARATOR, 0, "")
	appendMenuItem(file, MF_STRING, ID_FILE_EXIT, "В&ыход\tAlt+F4")

	edit, _, _ := pCreatePopupMenu.Call()
	appendMenuItem(edit, MF_STRING, ID_EDIT_UNDO, "&Отменить\tCtrl+Z")
	appendMenuItem(edit, MF_STRING, ID_EDIT_REDO, "&Повторить\tCtrl+Y")
	appendMenuItem(edit, MF_SEPARATOR, 0, "")
	appendMenuItem(edit, MF_STRING, ID_EDIT_CUT, "Вы&резать\tCtrl+X")
	appendMenuItem(edit, MF_STRING, ID_EDIT_COPY, "&Копировать\tCtrl+C")
	appendMenuItem(edit, MF_STRING, ID_EDIT_PASTE, "&Вставить\tCtrl+V")
	appendMenuItem(edit, MF_STRING, ID_EDIT_SELECT, "Выделить &всё\tCtrl+A")

	help, _, _ := pCreatePopupMenu.Call()
	appendMenuItem(help, MF_STRING, ID_HELP_ABOUT, "&О программе")

	main, _, _ := pCreateMenu.Call()
	appendMenuItem(main, MF_POPUP, file, "&Файл")
	appendMenuItem(main, MF_POPUP, edit, "&Правка")
	appendMenuItem(main, MF_POPUP, help, "&Справка")
	return main
}

func handleCommand(id int) {
	switch id {
	case ID_FILE_OPEN:
		openFileDialog()
	case ID_FILE_SAVE:
		saveFile()
	case ID_FILE_SAVEAS:
		saveFileAs()
	case ID_FILE_EXIT:
		closeApp()
	case ID_EDIT_UNDO:
		undoEdit()
	case ID_EDIT_REDO:
		redoEdit()
	case ID_EDIT_CUT:
		sendMsg(hCode, WM_CUT, 0, 0)
	case ID_EDIT_COPY:
		sendMsg(hCode, WM_COPY, 0, 0)
	case ID_EDIT_PASTE:
		sendMsg(hCode, WM_PASTE, 0, 0)
	case ID_EDIT_SELECT:
		sendMsg(hCode, EM_SETSEL, 0, ^uintptr(0))
	case ID_HELP_ABOUT:
		messageBox(appName+" "+appVersion+" — быстрый редактор Markdown.\n\n" +
			"Слева исходник, справа живой предпросмотр.\n" +
			"Правки в предпросмотре попадают в исходник.\n\n" +
			"Ctrl+B — жирный, Ctrl+I — курсив, Ctrl+U — подчёркнутый,\n" +
			"Ctrl+Shift+M — код, Ctrl+Shift+X — зачёркнутый.")
	}
}

// confirmDiscard asks what to do with unsaved changes.
func confirmDiscard() bool {
	if !isDirty {
		return true
	}
	r, _, _ := pMessageBoxW.Call(
		hwndMain,
		uintptr(unsafe.Pointer(utf16Ptr("Сохранить изменения перед продолжением?"))),
		uintptr(unsafe.Pointer(utf16Ptr("QuickMD"))),
		uintptr(MB_YESNOCANCEL|MB_ICONQUESTION),
	)
	switch r {
	case IDYES:
		saveFile()
		return !isDirty
	case IDNO:
		return true
	}
	return false
}

func closeApp() {
	if !confirmDiscard() {
		return
	}
	pDestroyWindow.Call(hwndMain)
}

func markdownFilter() *uint16 {
	return utf16Ptr("Markdown (*.md;*.markdown;*.mdx)\x00*.md;*.markdown;*.mdx\x00Все файлы (*.*)\x00*.*\x00\x00")
}

func openFileDialog() {
	if !confirmDiscard() {
		return
	}
	buf := make([]uint16, 4096)
	filter := markdownFilter()
	title := utf16Ptr("Открыть Markdown")
	ofn := OPENFILENAMEW{
		LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})),
		HwndOwner:   hwndMain,
		LpstrFilter: filter,
		LpstrFile:   &buf[0],
		NMaxFile:    uint32(len(buf)),
		LpstrTitle:  title,
		Flags:       OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST | OFN_NOCHANGEDIR | OFN_EXPLORER,
	}
	r, _, _ := pGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	runtime.KeepAlive(buf)
	runtime.KeepAlive(filter)
	runtime.KeepAlive(title)
	if r == 0 {
		return
	}
	openFile(syscall.UTF16ToString(buf))
}

func saveFileAs() {
	buf := make([]uint16, 4096)
	if filePath != "" {
		copy(buf, syscall.StringToUTF16(filePath))
	}
	filter := markdownFilter()
	title := utf16Ptr("Сохранить как")
	defExt := utf16Ptr("md")
	ofn := OPENFILENAMEW{
		LStructSize: uint32(unsafe.Sizeof(OPENFILENAMEW{})),
		HwndOwner:   hwndMain,
		LpstrFilter: filter,
		LpstrFile:   &buf[0],
		NMaxFile:    uint32(len(buf)),
		LpstrTitle:  title,
		LpstrDefExt: defExt,
		Flags:       OFN_OVERWRITEPROMPT | OFN_PATHMUSTEXIST | OFN_NOCHANGEDIR | OFN_EXPLORER,
	}
	r, _, _ := pGetSaveFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	runtime.KeepAlive(buf)
	runtime.KeepAlive(filter)
	runtime.KeepAlive(title)
	runtime.KeepAlive(defExt)
	if r == 0 {
		return
	}
	path := syscall.UTF16ToString(buf)
	if path == "" {
		return
	}
	filePath = path
	docDir = dirOf(path)
	saveFile()
	updateTitle()
}

func openFile(path string) {
	if path == "" {
		return
	}
	filePath = path
	loadFile()
	renderPreview()
	updateTitle()
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '\\' || path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
