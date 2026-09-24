package main

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	markdownImage          = regexp.MustCompile(`!\[([^\]]*)\]\(([^()\s]+)(?:\s+"[^"]*")?\)`)
	markdownLink           = regexp.MustCompile(`\[([^\]]+)\]\(([^()\s]+)(?:\s+"[^"]*")?\)`)
	markdownBoldStar       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	markdownBoldUnderscore = regexp.MustCompile(`__(.+?)__`)
	markdownStar           = regexp.MustCompile(`(^|[\s\p{P}])\*([^*\n]+?)\*([\s\p{P}]|$)`)
	markdownUnderscore     = regexp.MustCompile(`(^|[\s\p{P}])_([^_\n]+?)_([\s\p{P}]|$)`)
	markdownStrike         = regexp.MustCompile(`~~(.+?)~~`)
)

// renderMarkdown makes the common Markdown constructs in tickets readable in
// a terminal. Content remains plain text: it never emits terminal controls
// received from the API.
func renderMarkdown(source string) string {
	lines := strings.Split(sanitizeTerminalText(source), "\n")
	out := make([]string, 0, len(lines))
	inCodeBlock := false
	fence := ""

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if marker := markdownFence(trimmed); marker != "" {
			if !inCodeBlock {
				inCodeBlock, fence = true, marker
			} else if marker == fence {
				inCodeBlock, fence = false, ""
			}
			continue
		}
		if inCodeBlock {
			out = append(out, styledMuted("  "+line))
			continue
		}
		if heading, ok := markdownHeading(trimmed); ok {
			out = append(out, styledHeading(renderMarkdownInline(heading)))
			continue
		}
		if isMarkdownRule(trimmed) {
			out = append(out, styledMuted(strings.Repeat("─", 24)))
			continue
		}
		if quote, ok := strings.CutPrefix(trimmed, ">"); ok {
			out = append(out, styledMuted("│ ")+renderMarkdownInline(strings.TrimLeft(quote, " ")))
			continue
		}
		out = append(out, renderMarkdownList(line))
	}

	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func markdownFence(line string) string {
	if len(line) < 3 {
		return ""
	}
	if strings.HasPrefix(line, "```") {
		return "```"
	}
	if strings.HasPrefix(line, "~~~") {
		return "~~~"
	}
	return ""
}

func markdownHeading(line string) (string, bool) {
	count := 0
	for count < len(line) && line[count] == '#' {
		count++
	}
	if count == 0 || count > 6 || count == len(line) || (line[count] != ' ' && line[count] != '\t') {
		return "", false
	}
	return strings.TrimRight(strings.TrimSpace(line[count:]), "# "), true
}

func isMarkdownRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	marker := line[0]
	if marker != '-' && marker != '*' && marker != '_' {
		return false
	}
	for _, char := range line {
		if char != rune(marker) && char != ' ' && char != '\t' {
			return false
		}
	}
	return true
}

func renderMarkdownList(line string) string {
	indentLength := len(line) - len(strings.TrimLeft(line, " \t"))
	indent, content := line[:indentLength], line[indentLength:]
	if len(content) >= 2 && (content[0] == '-' || content[0] == '*' || content[0] == '+') && content[1] == ' ' {
		item := content[2:]
		if len(item) >= 4 && item[0] == '[' && item[2] == ']' && item[3] == ' ' {
			switch item[1] {
			case ' ', 'x', 'X':
				checkbox := "☐"
				if item[1] == 'x' || item[1] == 'X' {
					checkbox = "☑"
				}
				return indent + checkbox + " " + renderMarkdownInline(item[4:])
			}
		}
		return indent + "• " + renderMarkdownInline(item)
	}

	digits := 0
	for digits < len(content) && content[digits] >= '0' && content[digits] <= '9' {
		digits++
	}
	if digits > 0 && len(content) > digits+1 && (content[digits] == '.' || content[digits] == ')') && content[digits+1] == ' ' {
		return indent + content[:digits+2] + renderMarkdownInline(content[digits+2:])
	}
	return renderMarkdownInline(line)
}

func renderMarkdownInline(line string) string {
	var out strings.Builder
	for len(line) > 0 {
		start := strings.IndexByte(line, '`')
		if start == -1 {
			out.WriteString(renderMarkdownText(line))
			break
		}
		out.WriteString(renderMarkdownText(line[:start]))
		line = line[start+1:]
		end := strings.IndexByte(line, '`')
		if end == -1 {
			out.WriteByte('`')
			out.WriteString(renderMarkdownText(line))
			break
		}
		out.WriteString(styledAccent(line[:end]))
		line = line[end+1:]
	}
	return out.String()
}

func renderMarkdownText(text string) string {
	text = markdownImage.ReplaceAllString(text, "$1 <$2>")
	text = markdownLink.ReplaceAllString(text, "$1 <$2>")
	text = markdownStrike.ReplaceAllString(text, "$1")
	text = markdownBoldStar.ReplaceAllStringFunc(text, func(match string) string {
		return styledHeading(match[2 : len(match)-2])
	})
	text = markdownBoldUnderscore.ReplaceAllStringFunc(text, func(match string) string {
		return styledHeading(match[2 : len(match)-2])
	})
	text = renderMarkdownEmphasis(text, markdownStar)
	text = renderMarkdownEmphasis(text, markdownUnderscore)
	return strings.NewReplacer(`\\*`, "*", `\\_`, "_", `\\[`, "[", `\\]`, "]", `\\`+"`", "`").Replace(text)
}

func renderMarkdownEmphasis(text string, pattern *regexp.Regexp) string {
	return pattern.ReplaceAllStringFunc(text, func(match string) string {
		marker := "*"
		if !strings.Contains(match, marker) {
			marker = "_"
		}
		open := strings.Index(match, marker)
		close := strings.LastIndex(match, marker)
		return match[:open] + styledHeading(match[open+1:close]) + match[close+1:]
	})
}

// sanitizeTerminalText removes control sequences supplied by a ticket author.
// Newlines and tabs are retained because they are part of ordinary Markdown.
func sanitizeTerminalText(text string) string {
	var out strings.Builder
	for len(text) > 0 {
		r, size := utf8.DecodeRuneInString(text)
		text = text[size:]
		if r == 0x1b {
			if len(text) > 0 && text[0] == '[' {
				text = skipANSICSI(text[1:])
			}
			continue
		}
		if r == 0x9b {
			text = skipANSICSI(text)
			continue
		}
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

func skipANSICSI(text string) string {
	for len(text) > 0 {
		char := text[0]
		text = text[1:]
		if char >= 0x40 && char <= 0x7e {
			return text
		}
	}
	return text
}
