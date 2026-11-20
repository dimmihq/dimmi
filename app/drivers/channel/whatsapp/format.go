/*
|--------------------------------------------------------------------------
| Formatting
|--------------------------------------------------------------------------
|
| WhatsApp has its own small markup: *bold*, _italic_, ~strike~ and
| ```monospace```, no links and no headings. A reply written in Markdown
| is turned into it: emphasis mapped, headings made bold, a link shown
| as its words and its address, an image as its address, and a
| character Markdown escaped sent as itself, never with its backslash. A customer's
| message is read back the other way, so the thread shows what they
| meant. Code is left as it was, and an asterisk that is not around a
| word, as in 2*3*4, stays literal.
|
*/

package whatsapp

import (
	"regexp"
	"strings"
)

/*
MaxBodyChars is the longest text message WhatsApp sends, and
MaxCaptionChars the longest caption on a file.
*/
const (
	MaxBodyChars    = 4096
	MaxCaptionChars = 1024
)

var (
	imagePattern   = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	linkPattern    = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	boldPattern    = regexp.MustCompile(`(\*\*|__)([^\s*_](?:[^*_]*[^\s*_])?)(\*\*|__)`)
	italicStar     = regexp.MustCompile(`(^|[^\w*])\*([^\s*](?:[^*]*[^\s*])?)\*($|[^\w*])`)
	strikePattern  = regexp.MustCompile(`~~([^\s~](?:[^~]*[^\s~])?)~~`)
	headingPattern = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*#*\s*$`)
	inlineCode     = regexp.MustCompile("`[^`\n]+`")
	waBold         = regexp.MustCompile(`(^|[\s(\[{"'])\*([^\s*](?:[^*\n]*[^\s*])?)\*($|[\s.,!?;:)\]}"'])`)
	waItalic       = regexp.MustCompile(`(^|[\s(\[{"'])_([^\s_](?:[^_\n]*[^\s_])?)_($|[\s.,!?;:)\]}"'])`)
	waStrike       = regexp.MustCompile(`(^|[\s(\[{"'])~([^\s~](?:[^~\n]*[^\s~])?)~($|[\s.,!?;:)\]}"'])`)
	escaped        = regexp.MustCompile(`\\([\\` + "`" + `*_{}\[\]()#+\-.!~|>])`)
)

/*
Markers stand in for parts of a line while it is converted: bold, and
the prefix of inline code kept as it was.
*/
const (
	boldMarker       = "\x00b\x00"
	codeMarkerPrefix = "\x00c"
)

/*
ToWhatsApp turns Markdown into WhatsApp's markup.
*/
func ToWhatsApp(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	fenced := false

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			out = append(out, "```")

			continue
		}

		if fenced {
			out = append(out, line)

			continue
		}

		out = append(out, lineToWhatsApp(line))
	}

	return strings.TrimSpace(strings.Join(out, "\n"))
}

/*
lineToWhatsApp converts one line outside a code block, leaving its
inline code untouched.
*/
func lineToWhatsApp(line string) string {
	codes := inlineCode.FindAllString(line, -1)
	for i, code := range codes {
		line = strings.Replace(line, code, codeMarker(i), 1)
	}

	literals := escaped.FindAllStringSubmatch(line, -1)
	for i, m := range literals {
		line = strings.Replace(line, m[0], literalMarker(i), 1)
	}

	if m := headingPattern.FindStringSubmatch(line); m != nil {
		line = "**" + m[1] + "**"
	}

	line = imagePattern.ReplaceAllString(line, "$2")
	line = linkPattern.ReplaceAllStringFunc(line, func(match string) string {
		m := linkPattern.FindStringSubmatch(match)
		if m[1] == m[2] || strings.TrimPrefix(m[2], "mailto:") == m[1] {
			return m[2]
		}

		return m[1] + " (" + m[2] + ")"
	})
	line = boldPattern.ReplaceAllString(line, boldMarker+"$2"+boldMarker)
	line = italicStar.ReplaceAllString(line, "${1}_${2}_${3}")
	line = strings.ReplaceAll(line, boldMarker, "*")
	line = strikePattern.ReplaceAllString(line, "~$1~")

	if strings.HasPrefix(strings.TrimLeft(line, " "), "* ") {
		line = strings.Replace(line, "* ", "- ", 1)
	}

	for i, m := range literals {
		line = strings.Replace(line, literalMarker(i), m[1], 1)
	}

	for i, code := range codes {
		line = strings.Replace(line, codeMarker(i), code, 1)
	}

	return line
}

/*
literalMarker stands in for the i-th character a line escaped, so it is
sent as itself and never read as markup.
*/
func literalMarker(i int) string {
	return "\x00l" + string(rune('a'+i%26)) + "\x00"
}

/*
codeMarker stands in for the i-th inline code of a line.
*/
func codeMarker(i int) string {
	return codeMarkerPrefix + string(rune('a'+i%26)) + "\x00"
}

/*
ToMarkdown reads a customer's WhatsApp markup as Markdown.
*/
func ToMarkdown(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	fenced := false

	for i, line := range lines {
		if strings.Count(line, "```") == 1 {
			fenced = !fenced

			continue
		}

		if fenced || strings.Contains(line, "```") {
			continue
		}

		line = waBold.ReplaceAllString(line, "$1**$2**$3")
		line = waItalic.ReplaceAllString(line, "$1*$2*$3")
		lines[i] = waStrike.ReplaceAllString(line, "$1~~$2~~$3")
	}

	return strings.Join(lines, "\n")
}

/*
Split cuts text longer than the limit into parts that fit, at a blank
line, a line end or a space when one is near the limit, never within a
character.
*/
func Split(text string, limit int) []string {
	runes := []rune(text)
	if len(runes) <= limit {
		return []string{text}
	}

	var parts []string

	for len(runes) > limit {
		cut := limit
		window := string(runes[:limit])

		for _, sep := range []string{"\n\n", "\n", " "} {
			if at := strings.LastIndex(window, sep); at > limit/2 {
				cut = len([]rune(window[:at]))

				break
			}
		}

		parts = append(parts, strings.TrimSpace(string(runes[:cut])))
		runes = []rune(strings.TrimSpace(string(runes[cut:])))
	}

	if len(runes) > 0 {
		parts = append(parts, string(runes))
	}

	return parts
}

/*
Truncate cuts text to at most limit characters, marking the cut.
*/
func Truncate(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}

	return string(runes[:limit-1]) + "…"
}
