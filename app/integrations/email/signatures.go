/*
|--------------------------------------------------------------------------
| Signatures And Footers
|--------------------------------------------------------------------------
|
| What people and their mail systems add below a message. A signature
| starts at the "-- " delimiter when only a few lines follow it, at the
| same two dashes set apart when the space was lost, or at a "Sent from
| my iPhone" line that ends the mail. Below a quote, these and short
| closings or legal notices are never mistaken for the customer's answer.
|
*/

package email

import (
	"regexp"
	"slices"
	"strings"
)

/*
Bounds of what passes for a signature: how many lines may follow its
delimiter, with and without the delimiter's space, and how many lines a
closing below a quote may take.
*/
const (
	maxSignatureLines     = 12
	maxBareSignatureLines = 4
	maxClosingLines       = 4
)

var (
	mobileSignature = regexp.MustCompile(`(?i)^(sent from my .+|sent from (outlook|mail) for .+|` +
		`get outlook for .+|envoyé de mon .+|von meinem .+ gesendet|enviado desde mi .+|` +
		`inviato da( il)?( mio)? (iphone|ipad|android|samsung|galaxy|outlook)\b.*)$`)
	closingLine = regexp.MustCompile(`(?i)^(thanks|thank you|many thanks|regards|kind regards|best regards|best|` +
		`cheers|sincerely|merci|cordialement|danke|grüße|gracias|saludos|grazie|obrigado)[,.!]?$`)
	legalFooter = regexp.MustCompile(`(?i)(intended recipient|confidentiality notice|disclaimer|unsubscribe|` +
		`scanned (by|for)|virus-free|checked for viruses|` +
		`this (e-?mail|message|communication)[^.]{0,80}(confidential|privileged|intended))`)
)

/*
isFooter reports whether the lines are nothing, or only a signature, a
short closing or a legal notice.
*/
func isFooter(lines []string) bool {
	var written []string

	for _, raw := range lines {
		if line := plainLine(raw); line != "" {
			written = append(written, line)
		}
	}

	if len(written) == 0 {
		return true
	}

	first := written[0]
	if strings.HasPrefix(first, "--") || strings.HasPrefix(first, "__") || strings.HasPrefix(first, "—") ||
		mobileSignature.MatchString(first) {
		return true
	}

	if closingLine.MatchString(first) && len(written) <= maxClosingLines {
		return true
	}

	return legalFooter.MatchString(strings.Join(written, " "))
}

/*
signatureStart returns the line the signature starts at, and whether
there is one.
*/
func signatureStart(lines []string) (int, bool) {
	for i, raw := range slices.Backward(lines) {
		if (raw == "-- " || strings.TrimRight(raw, " \t") == `\--`) && len(lines)-i-1 < maxSignatureLines {
			return i, true
		}

		if bareDelimiter(lines, i) {
			return i, true
		}
	}

	for i, raw := range slices.Backward(lines) {
		line := plainLine(raw)
		if line == "" {
			continue
		}

		if mobileSignature.MatchString(line) {
			return i, true
		}

		break
	}

	return len(lines), false
}

/*
bareDelimiter reports whether line i is "--" with its space lost, set
apart like a signature: a blank line above it, no colon leading into
it, and only a few lines below. People also write "--" between lines of
their own.
*/
func bareDelimiter(lines []string, i int) bool {
	if strings.TrimSpace(lines[i]) != "--" || i < 2 || len(lines)-i-1 > maxBareSignatureLines {
		return false
	}

	return strings.TrimSpace(lines[i-1]) == "" && !endsInColon(strings.TrimSpace(lines[i-2]))
}
