/*
|--------------------------------------------------------------------------
| Quotes And Signatures
|--------------------------------------------------------------------------
|
| A reply carries the whole thread below it and a signature above that.
| Teammates and the agent should see only what the customer wrote this
| time, so the quoted thread and the signature are cut from the visible
| body. The full text stays beside it, so a wrong cut never loses words.
|
| Showing too much is harmless and hiding a customer's words is not, so
| every cut needs corroboration, and that comes from the shape of the
| mail rather than from a list of languages. A line introduces a quote
| when it ends in a colon, names an address or a time, or a date beside
| a known verb, and quoted lines follow. A header block is at least four
| labeled lines in a row, naming a sender and a subject. A rule of
| underscores counts only above one of those. A final run of quoted
| lines is cut unless its writer introduced it in their own words, as in
| "Run this:".
|
| Forwarded mail is content, never a quote: nothing from a forward's
| marker down is cut. A customer who answers between quoted lines keeps
| the whole message, while a signature or legal footer below a quote
| does not count as an answer. A cut that would leave nothing keeps
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
Bounds of the text pass: how many lines a client may wrap a quote's
first line over, how many labeled lines make a header block, and how
far such a block is read.
*/
const (
	maxAttributionLines = 3
	minHeaderFields     = 4
	maxHeaderFields     = 8
)

/*
subjectLabels names the subject line of a mail header in the languages
mail clients print it in.
*/
const subjectLabels = `subject|betreff|objet|sujet|asunto|assunto|oggetto|onderwerp|ämne|emne|aihe|temat|předmět|` +
	`predmet|предмет|тема|konu|tárgy|主题|主題|件名|제목|الموضوع|נושא|subiect|θέμα|subjek|perihal`

/*
forwardPrefixes are the subject prefixes that mean a forward.
*/
const forwardPrefixes = `fwd?|wg|tr|rv|enc|doorst|pd|i|転送|转发|轉寄|전달`

var (
	originalMessage = regexp.MustCompile(`(?i)^-{2,}\s*(original message|ursprüngliche nachricht|` +
		`message d'origine|mensaje original|messaggio originale|oorspronkelijk bericht)\s*-{2,}$`)
	underscoreRule = regexp.MustCompile(`^_{20,}$`)
	headerField    = regexp.MustCompile(`^([\p{L}\p{M}][\p{L}\p{M} .'-]{0,23}?)\s*[:：]\s*(.*)$`)
	senderLabel    = regexp.MustCompile(`(?i)^(from|von|absender|de|expéditeur|remitente|da|mittente|van|afzender|` +
		`från|avsändare|fra|avsender|afsender|od|nadawca|odesílatel|от|отправитель|від|kimden|gönderen|lähettäjä|` +
		`feladó|发件人|寄件人|寄件者|差出人|送信者|送信元|보낸 사람|보낸사람|من|מאת|από|dari|pengirim)$`)
	subjectLabel     = regexp.MustCompile(`(?i)^(` + subjectLabels + `)$`)
	forwardedSubject = regexp.MustCompile(`(?i)^(` + forwardPrefixes + `)\s*[:：]`)
	forwardedHead    = regexp.MustCompile(`(?i)(-{2,}\s*(forwarded message|weitergeleitete nachricht|` +
		`message transféré|mensaje reenviado|messaggio inoltrato|doorgestuurd bericht)|begin forwarded message|` +
		`(` + subjectLabels + `)\s*[:：]\s*(` + forwardPrefixes + `)\s*[:：])`)
	mailAddress     = regexp.MustCompile(`[^\s<>()\[\]]+@[^\s<>()\[\]]+\.[^\s<>()\[\]]+`)
	clockTime       = regexp.MustCompile(`(^|\D)\d{1,2}[:.]\d{2}(\D|$)`)
	attributionVerb = regexp.MustCompile(`(?i)(wrote|a écrit|schrieb|escribió|ha scritto|schreef|escreveu|pisze|` +
		`написал|написала|写道|のメッセージ|skrev|kirjoitti|napisał|napsal|yazdı|έγραψε|menulis|a scris)`)
	endsInVerb = regexp.MustCompile(`(?i)\bwrote$`)
	anyDigit   = regexp.MustCompile(`\d`)
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
Markdown body. A forward at the end of the body stays, and only what is
written above it is looked at.
*/
func StripText(text string) Stripped {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	forward := forwardStart(lines)

	cut, quote := quoteStart(lines, forward)
	if quote && interleaved(lines[cut:]) {
		cut, quote = len(lines), false
	}

	if !quote {
		cut = forward
	}

	sigCut, signature := signatureStart(lines[:cut])

	visible := slices.Clone(lines[:sigCut])
	if !quote {
		visible = append(visible, lines[forward:]...)
	}

	out := strings.TrimSpace(strings.Join(visible, "\n"))
	if out == "" && strings.TrimSpace(text) != "" {
		return Stripped{Visible: strings.TrimSpace(text), Fallback: quote || signature}
	}

	return Stripped{Visible: out, Quote: quote, Signature: signature}
}

/*
forwardStart returns the line a forwarded message starts at, quoted or
not, or the number of lines when there is none.
*/
func forwardStart(lines []string) int {
	for i, raw := range lines {
		line := unquotedLine(raw)
		if underscoreRule.MatchString(line) {
			continue
		}

		if forwardMarker.MatchString(line) || forwardedBlock(lines, i) {
			return i
		}
	}

	return len(lines)
}

/*
forwardedBlock reports whether line i starts a header block whose
subject says it was forwarded.
*/
func forwardedBlock(lines []string, i int) bool {
	fields := fieldRun(lines, i)
	if !isHeaderBlock(fields) {
		return false
	}

	return slices.ContainsFunc(fields, func(f field) bool {
		return subjectLabel.MatchString(f.label) && forwardedSubject.MatchString(f.value)
	})
}

/*
quoteStart returns the line the quoted thread starts at, and whether
there is one. Nothing at or below a forward counts, other than a line
that introduces the forward as an older message's quote.
*/
func quoteStart(lines []string, forward int) (int, bool) {
	for i := range lines {
		if i > forward {
			break
		}

		if start, ok := quoteAt(lines, i, forward); ok {
			return start, true
		}
	}

	if forward < len(lines) {
		return 0, false
	}

	return finalQuoteRun(lines)
}

/*
quoteAt reports where the quote that line i belongs to starts, when
line i proves one.
*/
func quoteAt(lines []string, i, forward int) (int, bool) {
	line := plainLine(lines[i])

	if i < forward {
		if originalMessage.MatchString(line) && hasTextBelow(lines, i) {
			return i, true
		}

		if underscoreRule.MatchString(line) && nextText(lines, i) != forward && quoteBelow(lines, i) {
			return i, true
		}

		if isHeaderBlock(fieldRun(lines, i)) {
			return i, true
		}
	}

	if isQuoted(lines[i]) && (i == 0 || !isQuoted(lines[i-1])) {
		return attributionAbove(lines, i)
	}

	return 0, false
}

/*
hasTextBelow reports whether any text follows line i.
*/
func hasTextBelow(lines []string, i int) bool {
	return nextText(lines, i) < len(lines)
}

/*
quoteBelow reports whether quoted lines or a header block come right
after line i.
*/
func quoteBelow(lines []string, i int) bool {
	next := nextText(lines, i)
	if next == len(lines) {
		return false
	}

	return isQuoted(lines[next]) || isHeaderBlock(fieldRun(lines, next))
}

/*
nextText returns the first line with text after line i, or the number
of lines when there is none.
*/
func nextText(lines []string, i int) int {
	for j := i + 1; j < len(lines); j++ {
		if strings.TrimSpace(lines[j]) != "" {
			return j
		}
	}

	return len(lines)
}

/*
attributionAbove finds the line a client wrote above the quoted line q,
trying more of the lines above when it wrapped them, and returns where
it starts.
*/
func attributionAbove(lines []string, q int) (int, bool) {
	end := q - 1
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}

	joined := ""

	for n := 0; n < maxAttributionLines && end-n >= 0; n++ {
		line := plainLine(lines[end-n])
		if line == "" || isQuoted(line) {
			return 0, false
		}

		joined = strings.TrimSpace(line + " " + joined)
		if isAttribution(joined) {
			return wrappedStart(lines, end-n, joined), true
		}
	}

	return 0, false
}

/*
wrappedStart walks up from a found attribution over lines a client
wrapped it from: one that opens the address it closes, or one that
holds the date and does not end a sentence of its own.
*/
func wrappedStart(lines []string, start int, joined string) int {
	for n := 1; n < maxAttributionLines && start > 0; n++ {
		line := plainLine(lines[start-1])
		if line == "" || isQuoted(line) {
			return start
		}

		opensAddress := strings.Count(joined, ">") > strings.Count(joined, "<")
		holdsDate := clockTime.MatchString(line) && !strings.ContainsAny(line[len(line)-1:], ".!?:")

		if !opensAddress && !holdsDate {
			return start
		}

		start--
		joined = line + " " + joined
	}

	return start
}

/*
isAttribution reports whether text reads as the line that introduces a
quote rather than a sentence of the customer's own.
*/
func isAttribution(text string) bool {
	address := mailAddress.MatchString(text)
	if !endsInColon(text) && (!address || !endsInVerb.MatchString(text)) {
		return false
	}

	return address || clockTime.MatchString(text) ||
		(anyDigit.MatchString(text) && attributionVerb.MatchString(text))
}

/*
endsInColon reports whether text ends in a colon of any width.
*/
func endsInColon(text string) bool {
	return strings.HasSuffix(text, ":") || strings.HasSuffix(text, "：")
}

/*
finalQuoteRun returns the start of the quoted lines that end the
message. Quoted lines earlier in the message mean inline answers, and a
line ending in a colon above them means the writer is showing them.
*/
func finalQuoteRun(lines []string) (int, bool) {
	start := len(lines)

	for i, raw := range slices.Backward(lines) {
		if strings.TrimSpace(raw) == "" {
			continue
		}

		if !isQuoted(raw) {
			break
		}

		start = i
	}

	if start == len(lines) || slices.ContainsFunc(lines[:start], isQuoted) {
		return 0, false
	}

	for _, raw := range slices.Backward(lines[:start]) {
		if line := plainLine(raw); line != "" {
			return start, !endsInColon(line)
		}
	}

	return start, true
}

/*
isQuoted reports whether a line is a quoted line.
*/
func isQuoted(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), ">")
}

/*
field is one labeled line of a header block.
*/
type field struct {
	label string
	value string
}

/*
fieldRun reads the labeled lines starting at line i. One blank line
may separate them, as when converted HTML gives each its own paragraph.
*/
func fieldRun(lines []string, i int) []field {
	var fields []field

	for j := i; j < len(lines) && len(fields) < maxHeaderFields; j++ {
		line := strings.ReplaceAll(plainLine(lines[j]), "*", "")
		match := headerField.FindStringSubmatch(line)

		switch {
		case match != nil:
			fields = append(fields, field{label: strings.TrimSpace(match[1]), value: strings.TrimSpace(match[2])})
		case line != "" || j == i || strings.TrimSpace(lines[j-1]) == "":
			return fields
		}
	}

	return fields
}

/*
isHeaderBlock reports whether labeled lines are a mail's header: enough
of them, starting with the sender or subject, and naming both.
*/
func isHeaderBlock(fields []field) bool {
	if len(fields) < minHeaderFields {
		return false
	}

	if !senderLabel.MatchString(fields[0].label) && !subjectLabel.MatchString(fields[0].label) {
		return false
	}

	has := func(label *regexp.Regexp) bool {
		return slices.ContainsFunc(fields, func(f field) bool { return label.MatchString(f.label) })
	}

	return has(senderLabel) && has(subjectLabel)
}

/*
interleaved reports whether the customer answered between or below
quoted lines: text of their own after a quoted line, other than a
footer that ends the mail.
*/
func interleaved(lines []string) bool {
	last := -1

	for i, raw := range lines {
		if i > 0 && isQuoted(raw) {
			last = i
		}
	}

	if last < 0 {
		return false
	}

	seenQuote := false

	for _, raw := range lines[1:last] {
		switch {
		case isQuoted(raw):
			seenQuote = true
		case seenQuote && strings.TrimSpace(raw) != "":
			return true
		}
	}

	return !isFooter(lines[last+1:])
}

/*
plainLine returns a line without surrounding spaces or the Markdown
emphasis and escapes HTML conversion adds, with non-breaking spaces as
ordinary ones.
*/
func plainLine(line string) string {
	line = strings.ReplaceAll(line, " ", " ")
	line = strings.TrimSpace(line)
	line = strings.ReplaceAll(line, `\`, "")

	return strings.TrimSpace(strings.Trim(line, "*"))
}

/*
unquotedLine returns a line as plainLine does, its quote marks removed.
*/
func unquotedLine(line string) string {
	return plainLine(strings.TrimLeft(strings.TrimSpace(line), "> "))
}
