package httpapi

import (
	"html"
	"html/template"
	"net/url"
	"regexp"
	"strings"
)

var portalInlinePattern = regexp.MustCompile("\\[[^\\]]+\\]\\([^)]*\\)|`[^`]+`|\\*\\*[^*]+\\*\\*")

// renderPortalMarkdown supports the small Markdown vocabulary used in client
// requests. Every source fragment is escaped before it enters generated HTML.
func renderPortalMarkdown(source string) template.HTML {
	var out strings.Builder
	inList, inCode := false, false
	closeList := func() {
		if inList {
			out.WriteString("</ul>")
			inList = false
		}
	}
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			closeList()
			if inCode {
				out.WriteString("</code></pre>")
			} else {
				out.WriteString("<pre><code>")
			}
			inCode = !inCode
			continue
		}
		if inCode {
			out.WriteString(html.EscapeString(line))
			out.WriteByte('\n')
			continue
		}
		if trimmed == "" {
			closeList()
			continue
		}
		if heading, ok := strings.CutPrefix(trimmed, "# "); ok {
			closeList()
			out.WriteString("<h2>")
			out.WriteString(renderPortalInline(heading))
			out.WriteString("</h2>")
			continue
		}
		if heading, ok := strings.CutPrefix(trimmed, "## "); ok {
			closeList()
			out.WriteString("<h3>")
			out.WriteString(renderPortalInline(heading))
			out.WriteString("</h3>")
			continue
		}
		if item, ok := strings.CutPrefix(trimmed, "- "); ok {
			if !inList {
				out.WriteString("<ul>")
				inList = true
			}
			if rest, ok := strings.CutPrefix(item, "[ ] "); ok {
				item = "☐ " + rest
			} else if rest, ok := strings.CutPrefix(item, "[x] "); ok {
				item = "☑ " + rest
			}
			out.WriteString("<li>")
			out.WriteString(renderPortalInline(item))
			out.WriteString("</li>")
			continue
		}
		closeList()
		if quote, ok := strings.CutPrefix(trimmed, "> "); ok {
			out.WriteString("<blockquote><p>")
			out.WriteString(renderPortalInline(quote))
			out.WriteString("</p></blockquote>")
			continue
		}
		out.WriteString("<p>")
		out.WriteString(renderPortalInline(trimmed))
		out.WriteString("</p>")
	}
	closeList()
	if inCode {
		out.WriteString("</code></pre>")
	}
	return template.HTML(out.String())
}

func renderPortalInline(source string) string {
	var out strings.Builder
	last := 0
	for _, loc := range portalInlinePattern.FindAllStringIndex(source, -1) {
		out.WriteString(html.EscapeString(source[last:loc[0]]))
		match := source[loc[0]:loc[1]]
		switch {
		case strings.HasPrefix(match, "`"):
			out.WriteString("<code>")
			out.WriteString(html.EscapeString(match[1 : len(match)-1]))
			out.WriteString("</code>")
		case strings.HasPrefix(match, "**"):
			out.WriteString("<strong>")
			out.WriteString(html.EscapeString(match[2 : len(match)-2]))
			out.WriteString("</strong>")
		default:
			open := strings.Index(match, "](")
			label, destination := match[1:open], match[open+2:len(match)-1]
			parsed, err := url.Parse(destination)
			if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
				out.WriteString(html.EscapeString(label))
				break
			}
			out.WriteString(`<a href="`)
			out.WriteString(html.EscapeString(destination))
			out.WriteString(`">`)
			out.WriteString(html.EscapeString(label))
			out.WriteString("</a>")
		}
		last = loc[1]
	}
	out.WriteString(html.EscapeString(source[last:]))
	return out.String()
}
