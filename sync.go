//go:build windows

package main

import (
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
)

var (
	srcText        string
	previewText    []rune
	previewSrc     []int
	previewValid   bool
	previewSyncing bool
)

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// diffRange returns the changed span in the old text and the new text.
func diffRange(oldText, newText []rune) (start, endOld, endNew int) {
	start = 0
	for start < len(oldText) && start < len(newText) && oldText[start] == newText[start] {
		start++
	}
	endOld = len(oldText)
	endNew = len(newText)
	for endOld > start && endNew > start && oldText[endOld-1] == newText[endNew-1] {
		endOld--
		endNew--
	}
	return
}

// mapPreviewRange translates a rune range in the preview into byte offsets in
// the Markdown source. Synthetic runs (markers, bullets) map to the nearest
// real source character.
func mapPreviewRange(start, end int) (int, int, bool) {
	if len(previewSrc) == 0 {
		return 0, 0, false
	}
	if start < 0 {
		start = 0
	}
	if start > len(previewSrc) {
		start = len(previewSrc)
	}
	if end > len(previewSrc) {
		end = len(previewSrc)
	}
	if end < start {
		end = start
	}
	if start == end {
		for i := start; i < len(previewSrc); i++ {
			if previewSrc[i] >= 0 {
				return previewSrc[i], previewSrc[i], true
			}
		}
		for i := start - 1; i >= 0; i-- {
			if previewSrc[i] >= 0 {
				off := previewSrc[i]
				size := 1
				if off < len(srcText) {
					_, size = utf8.DecodeRuneInString(srcText[off:])
				}
				return off + size, off + size, true
			}
		}
		return 0, 0, false
	}
	first, last := -1, -1
	for i := start; i < end; i++ {
		if previewSrc[i] >= 0 {
			if first < 0 {
				first = previewSrc[i]
			}
			last = previewSrc[i]
		}
	}
	if first < 0 {
		return mapPreviewRange(start, start)
	}
	lastEnd := last + 1
	if last >= 0 && last < len(srcText) {
		_, size := utf8.DecodeRuneInString(srcText[last:])
		if size > 0 {
			lastEnd = last + size
		}
	}
	return first, lastEnd, true
}

func utf16LenOf(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// applySourceReplace splices newText into the source control and returns the
// updated source string.
func applySourceReplace(srcStart, srcEnd int, newText string) string {
	old := srcText
	if srcStart < 0 {
		srcStart = 0
	}
	if srcStart > len(old) {
		srcStart = len(old)
	}
	if srcEnd < srcStart {
		srcEnd = srcStart
	}
	if srcEnd > len(old) {
		srcEnd = len(old)
	}
	newSrc := old[:srcStart] + newText + old[srcEnd:]

	previewSyncing = true
	uStart := utf16LenOf(old[:srcStart])
	uEnd := utf16LenOf(old[:srcEnd])
	cr := CHARRANGE{CpMin: int32(uStart), CpMax: int32(uEnd)}
	sendMsg(hCode, EM_EXSETSEL, 0, uintptr(unsafe.Pointer(&cr)))
	text := strings.ReplaceAll(newText, "\n", "\r\n")
	p := syscall.StringToUTF16Ptr(text)
	sendMsg(hCode, EM_REPLACESEL, 0, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
	previewSyncing = false

	lastSeen = newSrc
	lastEdit = time.Now()
	isDirty = true
	updateTitle()
	return newSrc
}

func updatePreviewMap(start, endOld int, newRunes []rune, srcStart, srcEnd int, newText string) {
	newMap := make([]int, 0, len(previewSrc)-endOld+start+len(newRunes))
	newMap = append(newMap, previewSrc[:start]...)
	off := srcStart
	for _, r := range newRunes {
		newMap = append(newMap, off)
		off += utf8.RuneLen(r)
	}
	delta := len(newText) - (srcEnd - srcStart)
	for _, v := range previewSrc[endOld:] {
		if v >= 0 {
			v += delta
		}
		newMap = append(newMap, v)
	}
	previewSrc = newMap
}

// applyPreviewEdit copies an edit made in the preview pane into the source.
func applyPreviewEdit(now []rune) {
	start, endOld, endNew := diffRange(previewText, now)
	newRunes := now[start:endNew]
	srcStart, srcEnd, ok := mapPreviewRange(start, endOld)
	if !ok {
		previewText = now
		return
	}
	newText := string(newRunes)
	pushUndo(srcText)
	srcText = applySourceReplace(srcStart, srcEnd, newText)
	updatePreviewMap(start, endOld, newRunes, srcStart, srcEnd, newText)
	previewText = now
}

func pollPreview() {
	if previewSyncing || !previewValid || hPreview == 0 {
		return
	}
	now := []rune(getText(hPreview))
	if runesEqual(now, previewText) {
		return
	}
	applyPreviewEdit(now)
}
