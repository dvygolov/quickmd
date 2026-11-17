package main

import (
	"regexp"
	"strings"
)

type KV struct {
	Key   string
	Value string
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
	Level int
	Text  string
}

type Block struct {
	Kind    BlockKind
	Level   int
	Text    string
	Lang    string
	Items   []ListItem
	Ordered bool
	Rows    [][]string
}

type Doc struct {
	Front  []KV
	Blocks []Block
}

var (
	headingRe = regexp.MustCompile(`^(#{1,6})[ \t]+(.*)$`)
	fenceRe   = regexp.MustCompile("^[ \t]*```[ \t]*([^`]*)$")
	hrRe      = regexp.MustCompile(`^[ \t]*((\*[ \t]*){3,}|(-[ \t]*){3,}|(_[ \t]*){3,})$`)
	listRe    = regexp.MustCompile(`^([ \t]*)([-*+]|[0-9]+[.)])[ \t]+(.*)$`)
	quoteRe   = regexp.MustCompile(`^[ \t]*>[ \t]?(.*)$`)
	tableSep  = regexp.MustCompile(`^[ \t]*\|?[ \t]*:?-{2,}:?[ \t]*(\|[ \t]*:?-{2,}:?[ \t]*)*\|?[ \t]*$`)
)

func parseMarkdown(src string) *Doc {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	lines := strings.Split(src, "\n")
	d := &Doc{}

	i := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for j := 1; j < len(lines); j++ {
			t := strings.TrimSpace(lines[j])
			if t == "---" || t == "..." {
				d.Front = parseFrontMatter(lines[1:j])
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
			d.Blocks = append(d.Blocks, Block{
				Kind:  BlockHeading,
				Level: len(m[1]),
				Text:  strings.TrimSpace(m[2]),
			})
			i++
			continue
		}

		if m := fenceRe.FindStringSubmatch(line); m != nil {
			lang := strings.TrimSpace(m[1])
			var body []string
			i++
			for i < len(lines) {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					i++
					break
				}
				body = append(body, lines[i])
				i++
			}
			d.Blocks = append(d.Blocks, Block{Kind: BlockCode, Lang: lang, Text: strings.Join(body, "\n")})
			continue
		}

		if hrRe.MatchString(line) {
			d.Blocks = append(d.Blocks, Block{Kind: BlockHR})
			i++
			continue
		}

		if m := quoteRe.FindStringSubmatch(line); m != nil {
			var parts []string
			for i < len(lines) {
				mm := quoteRe.FindStringSubmatch(lines[i])
				if mm == nil {
					break
				}
				parts = append(parts, strings.TrimSpace(mm[1]))
				i++
			}
			d.Blocks = append(d.Blocks, Block{Kind: BlockQuote, Text: strings.Join(parts, " ")})
			continue
		}

		if m := listRe.FindStringSubmatch(line); m != nil {
			b := Block{Kind: BlockList, Ordered: m[2] != "-" && m[2] != "*" && m[2] != "+"}
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
				b.Items = append(b.Items, ListItem{Level: level, Text: strings.TrimSpace(mm[3])})
				i++
			}
			d.Blocks = append(d.Blocks, b)
			continue
		}

		if strings.Contains(line, "|") && i+1 < len(lines) && tableSep.MatchString(lines[i+1]) {
			rows := [][]string{splitRow(line)}
			i += 2
			for i < len(lines) && strings.Contains(lines[i], "|") && strings.TrimSpace(lines[i]) != "" {
				rows = append(rows, splitRow(lines[i]))
				i++
			}
			d.Blocks = append(d.Blocks, Block{Kind: BlockTable, Rows: rows})
			continue
		}

		var parts []string
		for i < len(lines) {
			t := strings.TrimSpace(lines[i])
			if t == "" || headingRe.MatchString(lines[i]) || strings.HasPrefix(t, "```") ||
				hrRe.MatchString(lines[i]) || quoteRe.MatchString(lines[i]) || listRe.MatchString(lines[i]) {
				break
			}
			parts = append(parts, t)
			i++
		}
		d.Blocks = append(d.Blocks, Block{Kind: BlockParagraph, Text: strings.Join(parts, " ")})
	}

	return d
}

func parseFrontMatter(lines []string) []KV {
	var out []KV
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "- ") && len(out) > 0 {
			v := strings.TrimSpace(strings.TrimPrefix(t, "- "))
			if out[len(out)-1].Value != "" {
				out[len(out)-1].Value += ", " + v
			} else {
				out[len(out)-1].Value = v
			}
			continue
		}
		idx := strings.Index(t, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(t[:idx])
		val := strings.TrimSpace(t[idx+1:])
		val = strings.Trim(val, `"'`)
		out = append(out, KV{Key: key, Value: val})
	}
	return out
}

func splitRow(line string) []string {
	t := strings.TrimSpace(line)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
