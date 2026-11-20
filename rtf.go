package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	docDir string
	docDPI = 96
)

const (
	colText    = 1
	colDim     = 2
	colCode    = 3
	colLink    = 4
	colHeading = 5
	colAccent  = 6
)

// Style describes how a run of preview text is drawn.
type Style struct {
	Bold      bool
	Italic    bool
	Strike    bool
	Underline bool
	Code      bool
	Size      int
	Color     int
}

// Sink receives the flattened inline content of a Markdown block.
type Sink interface {
	Text(text string, src []int, st Style)
	Image(url string, src int) bool
}

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

// rtfSink writes inline runs as RTF.
type rtfSink struct {
	b      *bytes.Buffer
	cur    Style
	inited bool
}

func (s *rtfSink) Text(text string, src []int, st Style) {
	if !s.inited || st != s.cur {
		s.emitStyle(st)
		s.cur = st
		s.inited = true
	}
	writeText(s.b, text)
}

func (s *rtfSink) Image(url string, src int) bool {
	return writeImage(s.b, url)
}

func (s *rtfSink) emitStyle(st Style) {
	b := s.b
	if st.Code {
		b.WriteString(`\f1`)
	} else {
		b.WriteString(`\f0`)
	}
	b.WriteString(`\fs`)
	b.WriteString(strconv.Itoa(st.Size))
	b.WriteString(`\cf`)
	b.WriteString(strconv.Itoa(st.Color))
	if st.Bold {
		b.WriteString(`\b`)
	} else {
		b.WriteString(`\b0`)
	}
	if st.Italic {
		b.WriteString(`\i`)
	} else {
		b.WriteString(`\i0`)
	}
	if st.Strike {
		b.WriteString(`\strike`)
	} else {
		b.WriteString(`\strike0`)
	}
	if st.Underline {
		b.WriteString(`\ul`)
	} else {
		b.WriteString(`\ul0`)
	}
	b.WriteString(` `)
}

// plainSink records the visible text and its source mapping.
type plainSink struct {
	p *previewBuilder
}

func (s *plainSink) Text(text string, src []int, st Style) {
	s.p.text = append(s.p.text, []rune(text)...)
	s.p.src = append(s.p.src, src...)
}

func (s *plainSink) Image(url string, src int) bool {
	s.p.text = append(s.p.text, '\uFFFC')
	s.p.src = append(s.p.src, src)
	return true
}

type multiSink struct {
	rtf   *rtfSink
	plain *plainSink
}

func (m *multiSink) Text(text string, src []int, st Style) {
	m.rtf.Text(text, src, st)
	m.plain.Text(text, src, st)
}

func (m *multiSink) Image(url string, src int) bool {
	if !m.rtf.Image(url, src) {
		return false
	}
	m.plain.Image(url, src)
	return true
}

// previewBuilder accumulates the RTF output, the plain text shown in the
// preview and the source offset of every visible rune.
type previewBuilder struct {
	text []rune
	src  []int
	rtf  bytes.Buffer
}

func (p *previewBuilder) newline(srcByte int) {
	p.text = append(p.text, '\n')
	p.src = append(p.src, srcByte)
	p.rtf.WriteString(`\par`)
}

func (p *previewBuilder) emit(text string, src []int, st Style) {
	sink := &multiSink{rtf: &rtfSink{b: &p.rtf}, plain: &plainSink{p: p}}
	walkInline([]rune(text), src, st, sink)
}

func (p *previewBuilder) emitSynthetic(text string, st Style) {
	p.emit(text, syntheticSrc(len([]rune(text))), st)
}

func seqOffsets(text string, base int) []int {
	if base < 0 {
		return syntheticSrc(len([]rune(text)))
	}
	out := make([]int, 0, len([]rune(text)))
	for i := range text {
		out = append(out, base+i)
	}
	return out
}

type lineSeg struct {
	text string
	src  []int
	end  int
}

func splitLines(text string, src []int) []lineSeg {
	r := []rune(text)
	var out []lineSeg
	start := 0
	for i := 0; i <= len(r); i++ {
		if i == len(r) || r[i] == '\n' {
			end := 0
			if i < len(r) {
				end = src[i]
			} else if len(src) > 0 {
				end = src[len(src)-1] + utf8.RuneLen(r[len(r)-1])
			}
			out = append(out, lineSeg{text: string(r[start:i]), src: src[start:i], end: end})
			start = i + 1
		}
	}
	return out
}

func renderFrontMatter(p *previewBuilder, d *Doc) {
	if len(d.Front) == 0 {
		return
	}
	p.rtf.WriteString(`\pard\sa40 `)
	p.emitSynthetic("YAML front matter", Style{Size: 16, Color: colDim, Code: true})
	p.newline(d.FrontStart)
	for _, kv := range d.Front {
		p.rtf.WriteString(`\pard\sa20 `)
		p.emit(kv.Key, seqOffsets(kv.Key, kv.KeySrc), Style{Size: 18, Color: colText, Code: true, Bold: true})
		p.emitSynthetic("  ", Style{Size: 18, Color: colText, Code: true})
		p.emit(kv.Value, seqOffsets(kv.Value, kv.ValSrc), Style{Size: 18, Color: colDim, Code: true})
		p.newline(kv.SrcEnd)
	}
	p.rtf.WriteString(`\pard\sa160 `)
	p.emitSynthetic(strings.Repeat("─", 42), Style{Size: 14, Color: colDim})
	p.newline(d.FrontEnd)
}

func renderBlock(p *previewBuilder, blk Block) {
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
		p.rtf.WriteString(`\pard\sb220\sa140 `)
		p.emit(blk.Text, blk.Src, Style{Size: size, Color: colHeading, Bold: true})
		p.newline(blk.SrcEnd)

	case BlockParagraph:
		if strings.Contains(blk.Text, "![") {
			p.rtf.WriteString(`\pard\sa150\slmult0 `)
		} else {
			p.rtf.WriteString(`\pard\sa150\sl276\slmult1 `)
		}
		p.emit(blk.Text, blk.Src, Style{Size: 22, Color: colText})
		p.newline(blk.SrcEnd)

	case BlockCode:
		for _, seg := range splitLines(blk.Text, blk.Src) {
			p.rtf.WriteString(`\pard\li280\sa0\sl240\slmult0 `)
			p.emit(seg.text, seg.src, Style{Size: 18, Color: colCode, Code: true})
			p.newline(seg.end)
		}
		p.rtf.WriteString(`\pard\sa160 `)
		p.newline(blk.SrcEnd)

	case BlockList:
		for idx, it := range blk.Items {
			indent := 340 + it.Level*280
			bullet := "• "
			if blk.Ordered {
				bullet = strconv.Itoa(idx+1) + ". "
			}
			p.rtf.WriteString(fmt.Sprintf(`\pard\li%d\fi-240\sa60\sl264\slmult1 `, indent))
			p.emitSynthetic(bullet, Style{Size: 22, Color: colText})
			text := it.Text
			if strings.HasPrefix(text, "[ ] ") {
				text = "☐ " + text[4:]
			} else if strings.HasPrefix(strings.ToLower(text), "[x] ") {
				text = "☑ " + text[4:]
			}
			p.emit(text, it.Src, Style{Size: 22, Color: colText})
			p.newline(it.SrcEnd)
		}

	case BlockQuote:
		p.rtf.WriteString(`\pard\li360\sa100\sl264\slmult1 `)
		p.emitSynthetic("│ ", Style{Size: 22, Color: colDim, Italic: true})
		p.emit(blk.Text, blk.Src, Style{Size: 22, Color: colDim, Italic: true})
		p.newline(blk.SrcEnd)

	case BlockTable:
		for ri, row := range blk.Rows {
			p.rtf.WriteString(`\pard\sa40\sl240\slmult1 `)
			st := Style{Size: 20, Color: colText}
			if ri == 0 {
				st.Bold = true
			}
			for ci, cell := range row.Cells {
				if ci > 0 {
					p.emitSynthetic("  │  ", Style{Size: 20, Color: colDim})
				}
				var src []int
				if ci < len(row.Srcs) {
					src = row.Srcs[ci]
				}
				p.emit(cell, src, st)
			}
			p.newline(row.SrcEnd)
		}
		p.rtf.WriteString(`\pard\sa120 `)
		p.newline(blk.SrcEnd)

	case BlockHR:
		p.rtf.WriteString(`\pard\sa140 `)
		p.emitSynthetic(strings.Repeat("─", 48), Style{Size: 16, Color: colDim})
		p.newline(blk.SrcEnd)
	}
}

// renderPreviewDoc renders the document once and returns the RTF, the visible
// text and the source offset of every visible rune.
func renderPreviewDoc(d *Doc) ([]byte, []rune, []int) {
	p := &previewBuilder{}
	p.rtf.WriteString(rtfHeader())
	renderFrontMatter(p, d)
	for _, blk := range d.Blocks {
		renderBlock(p, blk)
	}
	p.rtf.WriteString(`\pard\f0\fs2\cf1 `)
	p.newline(len(d.Source))
	p.rtf.WriteString(`}`)
	return p.rtf.Bytes(), p.text, p.src
}

func walkInline(r []rune, src []int, st Style, sink Sink) {
	i := 0
	runStart := 0
	flush := func(end int) {
		if end > runStart {
			sink.Text(string(r[runStart:end]), src[runStart:end], st)
		}
	}
	for i < len(r) {
		c := r[i]
		switch {
		case c == '\\' && i+1 < len(r):
			flush(i)
			sink.Text(string(r[i+1]), src[i+1:i+2], st)
			i += 2
			runStart = i

		case c == '`':
			if j := indexRune(r, i+1, '`'); j >= 0 {
				flush(i)
				cs := st
				cs.Code = true
				cs.Size = st.Size - 2
				sink.Text(string(r[i+1:j]), src[i+1:j], cs)
				i = j + 1
				runStart = i
			} else {
				i++
			}

		case c == '*' && i+1 < len(r) && r[i+1] == '*':
			if j := indexSeq(r, i+2, "**"); j > i+2 {
				flush(i)
				bs := st
				bs.Bold = true
				walkInline(r[i+2:j], src[i+2:j], bs, sink)
				i = j + 2
				runStart = i
			} else {
				i += 2
			}

		case c == '_' && i+1 < len(r) && r[i+1] == '_':
			if j := indexSeq(r, i+2, "__"); j > i+2 {
				flush(i)
				bs := st
				bs.Bold = true
				walkInline(r[i+2:j], src[i+2:j], bs, sink)
				i = j + 2
				runStart = i
			} else {
				i += 2
			}

		case c == '~' && i+1 < len(r) && r[i+1] == '~':
			if j := indexSeq(r, i+2, "~~"); j > i+2 {
				flush(i)
				ss := st
				ss.Strike = true
				walkInline(r[i+2:j], src[i+2:j], ss, sink)
				i = j + 2
				runStart = i
			} else {
				i += 2
			}

		case c == '!' && i+1 < len(r) && r[i+1] == '[':
			if j := indexSeq(r, i+2, "]("); j >= 0 {
				k := indexRune(r, j+2, ')')
				if k >= 0 {
					alt := string(r[i+2 : j])
					url := strings.TrimSpace(string(r[j+2 : k]))
					flush(i)
					if sink.Image(url, src[i]) {
						i = k + 1
						runStart = i
						continue
					}
					sink.Text("[image: "+alt+"]", syntheticSrc(len([]rune("[image: "+alt+"]"))), st)
					i = k + 1
					runStart = i
					continue
				}
			}
			i++

		case c == '[':
			if j := indexSeq(r, i+1, "]("); j >= 0 {
				k := indexRune(r, j+2, ')')
				if k >= 0 {
					flush(i)
					ls := st
					ls.Underline = true
					ls.Color = colLink
					walkInline(r[i+1:j], src[i+1:j], ls, sink)
					i = k + 1
					runStart = i
					continue
				}
			}
			i++

		case c == '<':
			if j := indexRune(r, i+1, '>'); j >= 0 {
				flush(i)
				cs := st
				cs.Code = true
				cs.Size = st.Size - 2
				sink.Text(string(r[i:j+1]), src[i:j+1], cs)
				i = j + 1
				runStart = i
			} else {
				i++
			}

		default:
			i++
		}
	}
	flush(len(r))
}

func indexRune(r []rune, from int, want rune) int {
	for i := from; i < len(r); i++ {
		if r[i] == want {
			return i
		}
	}
	return -1
}

func indexSeq(r []rune, from int, seq string) int {
	s := []rune(seq)
	for i := from; i+len(s) <= len(r); i++ {
		ok := true
		for j := range s {
			if r[i+j] != s[j] {
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
		} else if r <= 0xFFFF {
			fmt.Fprintf(b, `\u%d?`, int16(r))
		} else {
			r -= 0x10000
			hi := 0xD800 + (r >> 10)
			lo := 0xDC00 + (r & 0x3FF)
			fmt.Fprintf(b, `\u%d?\u%d?`, int16(hi), int16(lo))
		}
	}
}

type imgCacheEntry struct {
	rtf string
	mod int64
}

var imgCache = map[string]imgCacheEntry{}

func writeImage(b *bytes.Buffer, url string) bool {
	if url == "" {
		return false
	}
	path := url
	if unescaped, err := neturl.PathUnescape(url); err == nil {
		path = unescaped
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(docDir, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if hit, ok := imgCache[path]; ok && hit.mod == info.ModTime().UnixNano() {
		b.WriteString(hit.rtf)
		return true
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".png" {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return false
		}
		var out bytes.Buffer
		if err := png.Encode(&out, img); err != nil {
			return false
		}
		data = out.Bytes()
	}
	const maxWidth = 640
	w, h := cfg.Width, cfg.Height
	if w > maxWidth {
		h = h * maxWidth / w
		w = maxWidth
	}
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	tw := w * 1440 / docDPI
	th := h * 1440 / docDPI
	var pic bytes.Buffer
	fmt.Fprintf(&pic, `{\pict\pngblip\picw%d\pich%d\picwgoal%d\pichgoal%d `, w, h, tw, th)
	pic.WriteString(hex.EncodeToString(data))
	pic.WriteString(`}`)
	imgCache[path] = imgCacheEntry{rtf: pic.String(), mod: info.ModTime().UnixNano()}
	b.WriteString(pic.String())
	return true
}
