package main

import (
	"regexp"
	"strings"
	"unicode"
)

type KV struct {
	Key    string
	Value  string
	KeySrc int
	ValSrc int
	SrcEnd int
}

type BlockKind int

const (
	BlockParagraph BlockKind = iota
	BlockHeading
	BlockCode
	BlockList
	BlockQuote
	BlockTable
	BlockHR
)

type ListItem struct {
	Level  int
	Text   string
	Src    []int
	SrcEnd int
}

type TableRow struct {
	Cells    []string
	Srcs     [][]int
	SrcStart int
	SrcEnd   int
}

type Block struct {
	Kind     BlockKind
	Level    int
	Text     string
	Src      []int
	Lang     string
	Items    []ListItem
	Ordered  bool
	Rows     []TableRow
	SrcStart int
	SrcEnd   int
}

type Doc struct {
	Front      []KV
	FrontStart int
	FrontEnd   int
	Blocks     []Block
	Source     string
}

var (
	headingRe = regexp.MustCompile(`^(#{1,6})[ \t]+(.*)$`)
	fenceRe   = regexp.MustCompile("^[ \t]*```[ \t]*([^`]*)$")
	hrRe      = regexp.MustCompile(`^[ \t]*((\*[ \t]*){3,}|(-[ \t]*){3,}|(_[ \t]*){3,})$`)
	listRe    = regexp.MustCompile(`^([ \t]*)([-*+]|[0-9]+[.)])[ \t]+(.*)$`)
	quoteRe   = regexp.MustCompile(`^[ \t]*>[ \t]?(.*)$`)
	tableSep  = regexp.MustCompile(`^[ \t]*\|?[ \t]*:?-{2,}:?[ \t]*(\|[ \t]*:?-{2,}:?[ \t]*)*\|?[ \t]*$`)
)

// runeOffsets returns, for every rune of raw, its byte offset in the source.
// base is the byte offset of raw[0] in the source.
func runeOffsets(raw string, base int) []int {
	out := make([]int, 0, len(raw))
	for i := range raw {
		out = append(out, base+i)
	}
	return out
}

// trimOffsets trims whitespace and returns the trimmed text with the source
// byte offset of every rune in it.
func trimOffsets(raw string, base int) (string, []int) {
	left := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace))
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	return trimmed, runeOffsets(trimmed, base+left)
}

func syntheticSrc(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = -1
	}
	return out
}

func parseMarkdown(src string) *Doc {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	lines := strings.Split(src, "\n")
	lineStart := make([]int, len(lines))
	off := 0
	for i, ln := range lines {
		lineStart[i] = off
		off += len(ln) + 1
	}
	d := &Doc{Source: src}

	i := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for j := 1; j < len(lines); j++ {
			t := strings.TrimSpace(lines[j])
			if t == "---" || t == "..." {
				d.Front = parseFrontMatter(lines[1:j], lineStart[1:])
				d.FrontStart = lineStart[0]
				d.FrontEnd = lineStart[j] + len(lines[j])
				i = j + 1
				break
			}
		}
	}

	for i < len(lines) {
		line := lines[i]
		trim := strings.TrimSpace(line)
		if trim == "" {
			i++
			continue
		}

		if m := headingRe.FindStringSubmatch(line); m != nil {
			base := lineStart[i] + len(line) - len(m[2])
			text, textSrc := trimOffsets(m[2], base)
			d.Blocks = append(d.Blocks, Block{
				Kind:     BlockHeading,
				Level:    len(m[1]),
				Text:     text,
				Src:      textSrc,
				SrcStart: lineStart[i],
				SrcEnd:   lineStart[i] + len(line),
			})
			i++
			continue
		}

		if m := fenceRe.FindStringSubmatch(line); m != nil {
			lang := strings.TrimSpace(m[1])
			start := i
			var body strings.Builder
			var bodySrc []int
			lineNo := 0
			i++
			for i < len(lines) {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					i++
					break
				}
				if lineNo > 0 {
					body.WriteByte('\n')
					bodySrc = append(bodySrc, lineStart[i]-1)
				}
				body.WriteString(lines[i])
				bodySrc = append(bodySrc, runeOffsets(lines[i], lineStart[i])...)
				lineNo++
				i++
			}
			end := lineStart[start]
			if i > start {
				end = lineStart[i-1] + len(lines[i-1])
			}
			d.Blocks = append(d.Blocks, Block{
				Kind:     BlockCode,
				Lang:     lang,
				Text:     body.String(),
				Src:      bodySrc,
				SrcStart: lineStart[start],
				SrcEnd:   end,
			})
			continue
		}

		if hrRe.MatchString(line) {
			d.Blocks = append(d.Blocks, Block{
				Kind:     BlockHR,
				SrcStart: lineStart[i],
				SrcEnd:   lineStart[i] + len(line),
			})
			i++
			continue
		}

		if m := quoteRe.FindStringSubmatch(line); m != nil {
			start := i
			var text strings.Builder
			var textSrc []int
			lineNo := 0
			for i < len(lines) {
				mm := quoteRe.FindStringSubmatch(lines[i])
				if mm == nil {
					break
				}
				base := lineStart[i] + len(lines[i]) - len(mm[1])
				part, partSrc := trimOffsets(mm[1], base)
				if lineNo > 0 {
					text.WriteByte(' ')
					textSrc = append(textSrc, lineStart[i]-1)
				}
				text.WriteString(part)
				textSrc = append(textSrc, partSrc...)
				lineNo++
				i++
			}
			d.Blocks = append(d.Blocks, Block{
				Kind:     BlockQuote,
				Text:     text.String(),
				Src:      textSrc,
				SrcStart: lineStart[start],
				SrcEnd:   lineStart[i-1] + len(lines[i-1]),
			})
			continue
		}

		if m := listRe.FindStringSubmatch(line); m != nil {
			start := i
			b := Block{Kind: BlockList, Ordered: m[2] != "-" && m[2] != "*" && m[2] != "+", SrcStart: lineStart[start]}
			for i < len(lines) {
				mm := listRe.FindStringSubmatch(lines[i])
				if mm == nil {
					break
				}
				indent := len(strings.ReplaceAll(mm[1], "\t", "  "))
				level := indent / 2
				if level > 4 {
					level = 4
				}
				base := lineStart[i] + len(lines[i]) - len(mm[3])
				text, textSrc := trimOffsets(mm[3], base)
				b.Items = append(b.Items, ListItem{
					Level:  level,
					Text:   text,
					Src:    textSrc,
					SrcEnd: lineStart[i] + len(lines[i]),
				})
				i++
			}
			b.SrcEnd = lineStart[i-1] + len(lines[i-1])
			d.Blocks = append(d.Blocks, b)
			continue
		}

		if strings.Contains(line, "|") && i+1 < len(lines) && tableSep.MatchString(lines[i+1]) {
			start := i
			var rows []TableRow
			cells, srcs := splitRowWithOffsets(line, lineStart[i])
			rows = append(rows, TableRow{Cells: cells, Srcs: srcs, SrcStart: lineStart[i], SrcEnd: lineStart[i] + len(line)})
			i += 2
			for i < len(lines) && strings.Contains(lines[i], "|") && strings.TrimSpace(lines[i]) != "" {
				cells, srcs := splitRowWithOffsets(lines[i], lineStart[i])
				rows = append(rows, TableRow{Cells: cells, Srcs: srcs, SrcStart: lineStart[i], SrcEnd: lineStart[i] + len(lines[i])})
				i++
			}
			d.Blocks = append(d.Blocks, Block{
				Kind:     BlockTable,
				Rows:     rows,
				SrcStart: lineStart[start],
				SrcEnd:   lineStart[i-1] + len(lines[i-1]),
			})
			continue
		}

		start := i
		var text strings.Builder
		var textSrc []int
		lineNo := 0
		for i < len(lines) {
			t := strings.TrimSpace(lines[i])
			if t == "" || headingRe.MatchString(lines[i]) || strings.HasPrefix(t, "```") ||
				hrRe.MatchString(lines[i]) || quoteRe.MatchString(lines[i]) || listRe.MatchString(lines[i]) {
				break
			}
			trimmed, trimmedSrc := trimOffsets(lines[i], lineStart[i])
			if lineNo > 0 {
				text.WriteByte(' ')
				textSrc = append(textSrc, lineStart[i]-1)
			}
			text.WriteString(trimmed)
			textSrc = append(textSrc, trimmedSrc...)
			lineNo++
			i++
		}
		d.Blocks = append(d.Blocks, Block{
			Kind:     BlockParagraph,
			Text:     text.String(),
			Src:      textSrc,
			SrcStart: lineStart[start],
			SrcEnd:   lineStart[i-1] + len(lines[i-1]),
		})
	}

	return d
}

func parseFrontMatter(lines []string, starts []int) []KV {
	var out []KV
	for idx, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		base := starts[idx]
		lead := len(line) - len(strings.TrimLeftFunc(line, unicode.IsSpace))
		if strings.HasPrefix(t, "- ") && len(out) > 0 {
			v := strings.TrimSpace(strings.TrimPrefix(t, "- "))
			if out[len(out)-1].Value != "" {
				out[len(out)-1].Value += ", " + v
			} else {
				out[len(out)-1].Value = v
			}
			continue
		}
		idx2 := strings.Index(t, ":")
		if idx2 <= 0 {
			continue
		}
		key := strings.TrimSpace(t[:idx2])
		val := strings.TrimSpace(t[idx2+1:])
		val = strings.Trim(val, `"'`)
		keySrc := base + lead
		valSrc := base + lead + idx2 + 1
		for valSrc < base+len(line) && (line[valSrc-base] == ' ' || line[valSrc-base] == '\t') {
			valSrc++
		}
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') {
			valSrc++
		}
		out = append(out, KV{Key: key, Value: val, KeySrc: keySrc, ValSrc: valSrc, SrcEnd: base + len(line)})
	}
	return out
}

func splitRowWithOffsets(line string, base int) ([]string, [][]int) {
	var cells []string
	var srcs [][]int
	pos := 0
	for {
		idx := strings.IndexByte(line[pos:], '|')
		if idx < 0 {
			cell, src := trimOffsets(line[pos:], base+pos)
			cells = append(cells, cell)
			srcs = append(srcs, src)
			break
		}
		cell, src := trimOffsets(line[pos:pos+idx], base+pos)
		cells = append(cells, cell)
		srcs = append(srcs, src)
		pos += idx + 1
	}
	trimmed := strings.TrimSpace(line)
	if len(cells) > 0 && cells[0] == "" && strings.HasPrefix(trimmed, "|") {
		cells = cells[1:]
		srcs = srcs[1:]
	}
	if len(cells) > 0 && cells[len(cells)-1] == "" && strings.HasSuffix(trimmed, "|") {
		cells = cells[:len(cells)-1]
		srcs = srcs[:len(srcs)-1]
	}
	return cells, srcs
}
