//go:build windows

package main

import (
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

type styleDef struct {
	face      string
	size      int
	color     uintptr
	bold      bool
	italic    bool
	underline bool
}

const (
	stBody    = 1
	stH1      = 2
	stH2      = 3
	stH3      = 4
	stH4      = 5
	stCode    = 6
	stBold    = 7
	stItalic  = 8
	stUnder   = 9
	stLink    = 10
	stDim     = 11
	stQuote   = 12
	stKey     = 13
	stValue   = 14
	stFmDim   = 15
	stSpoiler = 16
	stStrike  = 17
	stList    = 18
	stMark    = 19
)

var styleDefs = map[int]styleDef{
	stBody:    {"Segoe UI", 240, rgb(0xD4, 0xD4, 0xD4), false, false, false},
	stH1:      {"Segoe UI", 520, rgb(0xF0, 0xF0, 0xF0), true, false, false},
	stH2:      {"Segoe UI", 400, rgb(0xF0, 0xF0, 0xF0), true, false, false},
	stH3:      {"Segoe UI", 320, rgb(0xF0, 0xF0, 0xF0), true, false, false},
	stH4:      {"Segoe UI", 280, rgb(0xF0, 0xF0, 0xF0), true, false, false},
	stCode:    {"Consolas", 200, rgb(0xCE, 0x91, 0x78), false, false, false},
	stBold:    {"Segoe UI", 240, rgb(0xD4, 0xD4, 0xD4), true, false, false},
	stItalic:  {"Segoe UI", 240, rgb(0xD4, 0xD4, 0xD4), false, true, false},
	stUnder:   {"Segoe UI", 240, rgb(0xD4, 0xD4, 0xD4), false, false, true},
	stLink:    {"Segoe UI", 240, rgb(0x6E, 0xA8, 0xFE), false, false, true},
	stDim:     {"Segoe UI", 200, rgb(0x70, 0x70, 0x78), false, false, false},
	stQuote:   {"Segoe UI", 240, rgb(0x8A, 0x8A, 0x90), false, true, false},
	stKey:     {"Consolas", 200, rgb(0x7E, 0xC5, 0x96), false, false, false},
	stValue:   {"Consolas", 200, rgb(0xE6, 0xDB, 0x9A), false, false, false},
	stFmDim:   {"Consolas", 200, rgb(0x50, 0x50, 0x56), false, false, false},
	stSpoiler: {"Segoe UI", 240, rgb(0x90, 0x90, 0x98), false, true, false},
	stStrike:  {"Segoe UI", 240, rgb(0x90, 0x90, 0x98), false, false, false},
	stList:    {"Segoe UI", 240, rgb(0xE0, 0xB1, 0x69), true, false, false},
	stMark:    {"Segoe UI", 140, rgb(0x60, 0x60, 0x68), false, false, false},
}

type styleSpan struct {
	start, end int
	style      int
}

func applyStyles(h uintptr, src string) {
	var sel CHARRANGE
	sendMsg(h, EM_EXGETSEL, 0, uintptr(unsafe.Pointer(&sel)))
	first, _, _ := pSendMessageW.Call(h, uintptr(EM_GETFIRSTVISIBLELINE), 0, 0)
	pSendMessageW.Call(h, WM_SETREDRAW, 0, 0)

	applyCharFormat(h, 0, 1<<30, stBody)
	for _, sp := range buildStyleSpans(src) {
		applyCharFormat(h, sp.start, sp.end, sp.style)
	}

	cr := CHARRANGE{CpMin: sel.CpMin, CpMax: sel.CpMax}
	sendMsg(h, EM_EXSETSEL, 0, uintptr(unsafe.Pointer(&cr)))
	pSendMessageW.Call(h, WM_SETREDRAW, 1, 0)

	cur, _, _ := pSendMessageW.Call(h, uintptr(EM_GETFIRSTVISIBLELINE), 0, 0)
	if delta := int32(first) - int32(cur); delta != 0 {
		pSendMessageW.Call(h, uintptr(EM_LINESCROLL), 0, uintptr(int64(delta)))
	}
	pInvalidateRect.Call(h, 0, 1)
}

func applyCharFormat(h uintptr, start, end int, st int) {
	if end <= start {
		return
	}
	cr := CHARRANGE{CpMin: int32(start), CpMax: int32(end)}
	sendMsg(h, EM_EXSETSEL, 0, uintptr(unsafe.Pointer(&cr)))

	def := styleDefs[st]
	var cf CHARFORMAT2W
	cf.CbSize = uint32(unsafe.Sizeof(cf))
	cf.DwMask = CFM_BOLD | CFM_ITALIC | CFM_UNDERLINE | CFM_COLOR | CFM_FACE | CFM_SIZE
	cf.YHeight = int32(def.size)
	cf.CrTextColor = uint32(def.color)
	cf.BCharSet = 204
	copy(cf.SzFaceName[:], syscall.StringToUTF16(def.face))
	if def.bold {
		cf.DwEffects |= CFE_BOLD
	}
	if def.italic {
		cf.DwEffects |= CFE_ITALIC
	}
	if def.underline {
		cf.DwEffects |= CFE_UNDERLINE
	}
	sendMsg(h, EM_SETCHARFORMAT, SCF_SELECTION, uintptr(unsafe.Pointer(&cf)))
}

func buildStyleSpans(src string) []styleSpan {
	lines := strings.Split(src, "\n")
	var out []styleSpan
	pos := 0
	inFront := false
	inCode := false

	for i, line := range lines {
		ll := u16len(line)
		start, end := pos, pos+ll
		trim := strings.TrimSpace(line)

		switch {
		case i == 0 && trim == "---":
			inFront = true
			out = append(out, styleSpan{start, end, stFmDim})
		case inFront && (trim == "---" || trim == "..."):
			inFront = false
			out = append(out, styleSpan{start, end, stFmDim})
		case inFront:
			out = append(out, frontStyleSpans(line, pos)...)
		case strings.HasPrefix(trim, "```"):
			inCode = !inCode
			out = append(out, styleSpan{start, end, stFmDim})
		case inCode:
			out = append(out, styleSpan{start, start + u16len(line), stCode})
		default:
			out = append(out, lineStyleSpans(line, pos)...)
		}

		pos += u16len(line) + 1
	}
	return out
}

func frontStyleSpans(line string, base int) []styleSpan {
	var out []styleSpan
	end := base + u16len(line)
	out = append(out, styleSpan{base, end, stFmDim})
	idx := strings.Index(line, ":")
	if idx > 0 {
		out = append(out, styleSpan{base, base + u16len(line[:idx]), stKey})
		out = append(out, styleSpan{base + u16len(line[:idx+1]), base + u16len(line), stValue})
	}
	return out
}

func lineStyleSpans(line string, base int) []styleSpan {
	var out []styleSpan
	end := base + u16len(line)

	if m := headingRe.FindStringSubmatch(line); m != nil {
		mw := u16len(m[1])
		st := stH4
		switch len(m[1]) {
		case 1:
			st = stH1
		case 2:
			st = stH2
		case 3:
			st = stH3
		}
		out = append(out, styleSpan{base, base + mw, stMark})
		out = append(out, styleSpan{base + mw, base + u16len(line), st})
		return out
	}

	if hrRe.MatchString(line) {
		return []styleSpan{{base, end, stDim}}
	}

	if quoteRe.MatchString(line) {
		out = append(out, styleSpan{base, base + 1, stDim})
		out = append(out, styleSpan{base + 1, end, stQuote})
	}

	if m := listRe.FindStringSubmatch(line); m != nil {
		if idx := strings.Index(line, m[2]); idx >= 0 {
			s := base + u16len(line[:idx])
			out = append(out, styleSpan{s, s + u16len(m[2]), stList})
		}
	}

	out = append(out, inlineStyleSpans(line, base)...)
	return out
}

func inlineStyleSpans(line string, base int) []styleSpan {
	r := []rune(line)
	pre := make([]int, len(r)+1)
	for i, ch := range r {
		pre[i+1] = pre[i] + utf16.RuneLen(ch)
	}
	var out []styleSpan
	add := func(from, to int, st int) {
		if to > from {
			out = append(out, styleSpan{base + pre[from], base + pre[to], st})
		}
	}

	for i := 0; i < len(r); i++ {
		switch {
		case r[i] == '`':
			if j := indexRune(r, i+1, '`'); j > i {
				add(i, i+1, stDim)
				add(i+1, j, stCode)
				add(j, j+1, stDim)
				i = j
			}
		case r[i] == '*' && i+1 < len(r) && r[i+1] == '*':
			if j := indexSeq(r, i+2, "**"); j > i+2 {
				add(i, i+2, stDim)
				add(i+2, j, stBold)
				add(j, j+2, stDim)
				i = j + 1
			}
		case r[i] == '_' && i+1 < len(r) && r[i+1] == '_':
			if j := indexSeq(r, i+2, "__"); j > i+2 {
				add(i, i+2, stDim)
				add(i+2, j, stUnder)
				add(j, j+2, stDim)
				i = j + 1
			}
		case r[i] == '*' || r[i] == '_':
			if j := indexRune(r, i+1, r[i]); j > i+1 {
				add(i, i+1, stDim)
				add(i+1, j, stItalic)
				add(j, j+1, stDim)
				i = j
			}
		case r[i] == '~' && i+1 < len(r) && r[i+1] == '~':
			if j := indexSeq(r, i+2, "~~"); j > i+2 {
				add(i, i+2, stDim)
				add(i+2, j, stStrike)
				add(j, j+2, stDim)
				i = j + 1
			}
		case r[i] == '|' && i+1 < len(r) && r[i+1] == '|':
			if j := indexSeq(r, i+2, "||"); j > i+2 {
				add(i, i+2, stDim)
				add(i+2, j, stSpoiler)
				add(j, j+2, stDim)
				i = j + 1
			}
		case r[i] == '[':
			if j := indexSeq(r, i+1, "]("); j > 0 {
				if k := indexRune(r, j+2, ')'); k > j {
					add(i, i+1, stDim)
					add(i+1, j, stLink)
					add(j, k+1, stDim)
					i = k
				}
			}
		case r[i] == '<':
			if j := indexRune(r, i+1, '>'); j > i {
				add(i, j+1, stDim)
				i = j
			}
		}
	}
	return out
}

func u16len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

func indexRune(r []rune, from int, ch rune) int {
	for i := from; i < len(r); i++ {
		if r[i] == ch {
			return i
		}
	}
	return -1
}

func indexSeq(r []rune, from int, s string) int {
	seq := []rune(s)
	for i := from; i+len(seq) <= len(r); i++ {
		ok := true
		for j := range seq {
			if r[i+j] != seq[j] {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}
