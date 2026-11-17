package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

const (
	colText    = 1
	colDim     = 2
	colCode    = 3
	colLink    = 4
	colHeading = 5
	colAccent  = 6
)

func rtfHeader() string {
	return `{\rtf1\ansi\ansicpg1252\deff0\uc1\viewkind4` +
		`{\fonttbl{\f0\fswiss\fcharset204 Segoe UI;}{\f1\fmodern\fcharset204 Consolas;}}` +
		`{\colortbl ;` +
		`\red214\green214\blue214;` +
		`\red138\green138\blue138;` +
		`\red206\green145\blue120;` +
		`\red106\green156\blue214;` +
		`\red126\green197\blue150;` +
		`\red224\green177\blue105;}` +
		`\pard\f0\fs22\cf1`
}

func renderFrontMatter(b *bytes.Buffer, d *Doc) {
	if len(d.Front) == 0 {
		return
	}
	b.WriteString(`\pard\f1\fs16\cf2\sa40 YAML front matter\par`)
	for _, kv := range d.Front {
		b.WriteString(`\pard\f1\fs18\sa20\cf1\b `)
		writeText(b, kv.Key)
		b.WriteString(`\b0  \cf2 `)
		writeText(b, kv.Value)
		b.WriteString(`\par`)
	}
	b.WriteString(`\pard\f0\fs14\cf2\sa160 `)
	writeText(b, strings.Repeat("─", 42))
	b.WriteString(`\par`)
}

func renderBlock(b *bytes.Buffer, blk Block) {
	switch blk.Kind {
	case BlockHeading:
		size := 22
		switch blk.Level {
		case 1:
			size = 40
		case 2:
			size = 34
		case 3:
			size = 28
		case 4:
			size = 24
		}
		fmt.Fprintf(b, `\pard\sb220\sa140\f0\fs%d\b\cf%d `, size, colHeading)
		writeInline(b, blk.Text, size)
		b.WriteString(`\b0\par`)

	case BlockParagraph:
		b.WriteString(`\pard\sa150\sl276\slmult1\f0\fs22\cf1 `)
		writeInline(b, blk.Text, 22)
		b.WriteString(`\par`)

	case BlockCode:
		for _, ln := range strings.Split(blk.Text, "\n") {
			b.WriteString(`\pard\li280\sa0\sl240\slmult0\f1\fs18\cf3 `)
			writeText(b, ln)
			b.WriteString(`\par`)
		}
		b.WriteString(`\pard\sa160\par`)

	case BlockList:
		for idx, it := range blk.Items {
			indent := 340 + it.Level*280
			bullet := "• "
			if blk.Ordered {
				bullet = strconv.Itoa(idx+1) + ". "
			}
			fmt.Fprintf(b, `\pard\li%d\fi-240\sa60\sl264\slmult1\f0\fs22\cf1 `, indent)
			writeText(b, bullet)
			text := it.Text
			if strings.HasPrefix(text, "[ ] ") {
				text = "☐ " + text[4:]
			} else if strings.HasPrefix(strings.ToLower(text), "[x] ") {
				text = "☑ " + text[4:]
			}
			writeInline(b, text, 22)
			b.WriteString(`\par`)
		}

	case BlockQuote:
		b.WriteString(`\pard\li360\sa100\sl264\slmult1\f0\fs22\i\cf2 `)
		writeText(b, "│ ")
		writeInline(b, blk.Text, 22)
		b.WriteString(`\i0\par`)

	case BlockTable:
		for ri, row := range blk.Rows {
			b.WriteString(`\pard\sa40\sl240\slmult1\f0\fs20\cf1 `)
			if ri == 0 {
				b.WriteString(`\b `)
			}
			for ci, cell := range row {
				if ci > 0 {
					b.WriteString(`  \cf2`)
					writeText(b, "│")
					b.WriteString(`\cf1  `)
				}
				writeInline(b, cell, 20)
			}
			if ri == 0 {
				b.WriteString(`\b0 `)
			}
			b.WriteString(`\par`)
		}
		b.WriteString(`\pard\sa120\par`)

	case BlockHR:
		b.WriteString(`\pard\sa140\f0\fs16\cf2 `)
		writeText(b, strings.Repeat("─", 48))
		b.WriteString(`\par`)
	}
}

func renderRTF(d *Doc) []byte {
	var b bytes.Buffer
	b.WriteString(rtfHeader())
	renderFrontMatter(&b, d)
	for _, blk := range d.Blocks {
		renderBlock(&b, blk)
	}
	b.WriteString(`\pard\f0\fs2\cf1 \par}`)
	return b.Bytes()
}

// renderRTFChunks splits a document into RTF documents below max bytes because
// RichEdit imports only about a kilobyte of RTF from the clipboard per paste.
func renderRTFChunks(d *Doc, max int) [][]byte {
	head := rtfHeader()
	var chunks [][]byte
	var cur bytes.Buffer
	cur.WriteString(head)
	flush := func() {
		if cur.Len() > len(head) {
			chunks = append(chunks, []byte(cur.String()+`}`))
			cur.Reset()
			cur.WriteString(head)
		}
	}
	renderFrontMatter(&cur, d)
	for _, blk := range d.Blocks {
		var tb bytes.Buffer
		renderBlock(&tb, blk)
		if cur.Len()+tb.Len()+1 > max {
			flush()
		}
		cur.Write(tb.Bytes())
	}
	flush()
	if len(chunks) == 0 {
		chunks = append(chunks, []byte(rtfHeader()+`}`))
	}
	return chunks
}

func writeInline(b *bytes.Buffer, s string, base int) {
	r := []rune(s)
	i := 0
	for i < len(r) {
		c := r[i]
		switch {
		case c == '\\' && i+1 < len(r):
			writeRuneText(b, r[i+1])
			i += 2

		case c == '`':
			if j := indexRune(r, i+1, '`'); j >= 0 {
				fmt.Fprintf(b, `\f1\fs%d\cf%d `, base-2, colCode)
				writeText(b, string(r[i+1:j]))
				fmt.Fprintf(b, `\f0\fs%d\cf%d `, base, colText)
				i = j + 1
			} else {
				writeRuneText(b, c)
				i++
			}

		case c == '*' && i+1 < len(r) && r[i+1] == '*':
			if j := indexSeq(r, i+2, "**"); j > i+2 {
				b.WriteString(`\b `)
				writeInline(b, string(r[i+2:j]), base)
				b.WriteString(`\b0 `)
				i = j + 2
			} else {
				writeText(b, "**")
				i += 2
			}

		case c == '_' && i+1 < len(r) && r[i+1] == '_':
			if j := indexSeq(r, i+2, "__"); j > i+2 {
				b.WriteString(`\b `)
				writeInline(b, string(r[i+2:j]), base)
				b.WriteString(`\b0 `)
				i = j + 2
			} else {
				writeText(b, "__")
				i += 2
			}

		case c == '*' || c == '_':
			if j := indexRune(r, i+1, c); j > i+1 {
				b.WriteString(`\i `)
				writeInline(b, string(r[i+1:j]), base)
				b.WriteString(`\i0 `)
				i = j + 1
			} else {
				writeRuneText(b, c)
				i++
			}

		case c == '~' && i+1 < len(r) && r[i+1] == '~':
			if j := indexSeq(r, i+2, "~~"); j > i+2 {
				b.WriteString(`\strike `)
				writeInline(b, string(r[i+2:j]), base)
				b.WriteString(`\strike0 `)
				i = j + 2
			} else {
				writeText(b, "~~")
				i += 2
			}

		case c == '!' && i+1 < len(r) && r[i+1] == '[':
			if j := indexSeq(r, i+2, "]("); j >= 0 {
				k := indexRune(r, j+2, ')')
				if k >= 0 {
					b.WriteString(`\cf2 [image: `)
					writeInline(b, string(r[i+2:j]), base)
					b.WriteString(`] \cf1 `)
					i = k + 1
					continue
				}
			}
			writeRuneText(b, c)
			i++

		case c == '[':
			if j := indexSeq(r, i+1, "]("); j >= 0 {
				k := indexRune(r, j+2, ')')
				if k >= 0 {
					b.WriteString(`\ul\cf4 `)
					writeInline(b, string(r[i+1:j]), base)
					b.WriteString(`\ul0\cf1 `)
					i = k + 1
					continue
				}
			}
			writeRuneText(b, c)
			i++

		case c == '<':
			if j := indexRune(r, i+1, '>'); j >= 0 {
				b.WriteString(`\f1\fs` + strconv.Itoa(base-2) + `\cf2 `)
				writeText(b, string(r[i:j+1]))
				fmt.Fprintf(b, `\f0\fs%d\cf%d `, base, colText)
				i = j + 1
			} else {
				writeRuneText(b, c)
				i++
			}

		default:
			writeRuneText(b, c)
			i++
		}
	}
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

func writeText(b *bytes.Buffer, s string) {
	for _, r := range s {
		writeRuneText(b, r)
	}
}

func writeRuneText(b *bytes.Buffer, r rune) {
	switch r {
	case '\\':
		b.WriteString(`\\`)
	case '{':
		b.WriteString(`\{`)
	case '}':
		b.WriteString(`\}`)
	case '\n':
		b.WriteString(`\line `)
	case '\t':
		b.WriteString(`\tab `)
	default:
		if r < 128 {
			b.WriteByte(byte(r))
			return
		}
		if r <= 0xFFFF {
			fmt.Fprintf(b, `\u%d?`, int16(r))
			return
		}
		r -= 0x10000
		hi := 0xD800 + (r >> 10)
		lo := 0xDC00 + (r & 0x3FF)
		fmt.Fprintf(b, `\u%d?\u%d?`, int16(hi), int16(lo))
	}
}
