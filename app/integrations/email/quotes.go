/*
|--------------------------------------------------------------------------
| Quotes And Signatures
|--------------------------------------------------------------------------
|
| A reply carries the whole thread below it and a signature above that.
| Teammates and the agent should see only what the customer wrote this
| time, so the quoted thread and the signature are cut from the visible
| body, while the full message stays available.
|
| The text pass follows GitHub's email_reply_parser (MIT): the quoted
| thread starts at a header line such as "On Mon, Bob wrote:", in a
| dozen languages and also when a client wrapped it over two lines, at
| an Outlook header block, at "Original Message", or at a final run of
| quoted lines. A signature starts at the "-- " delimiter when only a
| few lines follow it, or at a "Sent from my iPhone" line near the end.
|
| Stripping is careful. A customer who answers between quoted lines
| keeps the whole message, and a cut that would leave nothing keeps
| everything instead.
|
*/

package email

import (
	"regexp"
	"slices"
	"strings"
)

/*
Bounds of the text pass: how many lines may follow a signature
delimiter, how close to the end a mobile signature line must be, and
how far below a From line Outlook puts its Sent line.
*/
const (
	maxSignatureLines  = 12
	mobileSignatureEnd = 3
	outlookBlockLines  = 4
)

var (
	quoteHeader = regexp.MustCompile(`(?i)^(` +
		`on\b.{4,}\bwrote:?|le\b.{4,}\ba écrit ?:|am\b.{4,}\bschrieb\b.*:|el\b.{4,}\bescribió:|` +
		`il\b.{4,}\bha scritto:|op\b.{4,}\bschreef\b.*:|em\b.{4,}\bescreveu:|w dniu\b.{4,}\bpisze:|` +
		`.{4,}написал(а)?:|.{4,}写道[：:]|.{4,}のメッセージ[：:]|` +
		`-{2,}\s*(original message|ursprüngliche nachricht|message d'origine|mensaje original|` +
		`messaggio originale|oorspronkelijk bericht)\s*-{2,}|_{20,})$`)
	wrappedHeader   = regexp.MustCompile(`(?i)^(on|le|am|el|il|op|em|w dniu)\s`)
	outlookFrom     = regexp.MustCompile(`(?i)^\**(from|von|de|da|van|från)\s*:\**\s`)
	outlookSent     = regexp.MustCompile(`(?i)^\**(sent|date|gesendet|envoyé|enviado|inviato|verzonden|skickat)\s*:`)
	mobileSignature = regexp.MustCompile(`(?i)^(sent from my .+|sent from (outlook|mail) for .+|` +
		`get outlook for .+|envoyé de mon .+|von meinem .+ gesendet|enviado desde mi .+|inviato da .+)$`)
)

/*
Stripped is a body with the quoted thread and signature cut off, and
what was cut. Fallback means a cut would have left nothing, so nothing
was cut.
*/
type Stripped struct {
	Visible   string
	Quote     bool
	Signature bool
	Fallback  bool
}

/*
StripText cuts the quoted thread and the signature from a plain text or
Markdown body.
*/
func StripText(text string) Stripped {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	cut, quote := quoteStart(lines)
	if quote && interleaved(lines[cut:]) {
		cut, quote = len(lines), false
	}

	visible := lines[:cut]
	sigCut, signature := signatureStart(visible)
	visible = visible[:sigCut]

	out := strings.TrimSpace(strings.Join(visible, "\n"))
	if out == "" && strings.TrimSpace(text) != "" {
		return Stripped{Visible: strings.TrimSpace(text), Fallback: quote || signature}
	}

	return Stripped{Visible: out, Quote: quote, Signature: signature}
}

/*
quoteStart returns the line the quoted thread starts at, and whether
there is one.
*/
func quoteStart(lines []string) (int, bool) {
	for i := range lines {
		if introducesQuote(lines, i) || outlookBlock(lines, i) {
			return i, true
		}
	}

	start := len(lines)
	for i, raw := range slices.Backward(lines) {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, ">") {
			break
		}

		start = i
	}

	return start, start < len(lines)
}

/*
introducesQuote reports whether line i introduces a quote, alone or
joined with the next line when a client wrapped it.
*/
func introducesQuote(lines []string, i int) bool {
	line := plainLine(lines[i])
	if quoteHeader.MatchString(line) {
		return true
	}

	if i+1 >= len(lines) || !wrappedHeader.MatchString(line) {
		return false
	}

	return quoteHeader.MatchString(line + " " + plainLine(lines[i+1]))
}

/*
outlookBlock reports whether line i starts Outlook's header block: a
From line followed closely by a Sent or Date line.
*/
func outlookBlock(lines []string, i int) bool {
	if !outlookFrom.MatchString(plainLine(lines[i])) {
		return false
	}

	for j := i + 1; j < len(lines) && j <= i+outlookBlockLines; j++ {
		if outlookSent.MatchString(plainLine(lines[j])) {
			return true
		}
	}

	return false
}

/*
interleaved reports whether the customer answered between quoted
lines: text of their own after a quoted line.
*/
func interleaved(lines []string) bool {
	seenQuote := false

	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)

		switch {
		case line == "":
		case strings.HasPrefix(line, ">"):
			seenQuote = true
		case seenQuote:
			return true
		}
	}

	return false
}

/*
signatureStart returns the line the signature starts at, and whether
there is one.
*/
func signatureStart(lines []string) (int, bool) {
	for i, raw := range slices.Backward(lines) {
		line := strings.TrimRight(raw, " \t")
		if (line == "--" || line == `\--`) && len(lines)-i-1 < maxSignatureLines {
			return i, true
		}
	}

	seen := 0

	for i, raw := range slices.Backward(lines) {
		line := plainLine(raw)
		if line == "" {
			continue
		}

		if mobileSignature.MatchString(line) {
			return i, true
		}

		seen++
		if seen == mobileSignatureEnd {
			break
		}
	}

	return len(lines), false
}

/*
plainLine returns a line without surrounding spaces or the Markdown
emphasis and escapes HTML conversion adds.
*/
func plainLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.ReplaceAll(line, `\`, "")

	return strings.TrimSpace(strings.Trim(line, "*_"))
}
