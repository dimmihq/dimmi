/*
|--------------------------------------------------------------------------
| Addressing
|--------------------------------------------------------------------------
|
| Mail reaches the install at forwarding addresses on its inbound
| domain. A workspace's first address is its slug, further ones add a
| label after a dot; slugs never contain dots, so the two never clash.
| Names an operator or a mail system relies on, such as postmaster, are
| never handed out. A reply token rides after a plus sign, so a reply
| finds its conversation from the address alone.
|
| Tokens are 16 lowercase Crockford base32 characters, 80 random bits:
| unguessable, and safe in any mail system's local part.
|
*/

package email

import (
	"encoding/base32"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/support/str"
)

/*
Shapes of local parts: a reply token's length, the bytes of randomness
behind it, and the longest label of an additional address.
*/
const (
	tokenLength    = 16
	tokenBytes     = 10
	tokenAlphabet  = "0123456789abcdefghjkmnpqrstvwxyz"
	maxLabelLength = 30
	tokenSeed      = 32
)

/*
Recipient is a recipient address read against the inbound domain:
the forwarding address it reaches and the reply token it carries, if
any.
*/
type Recipient struct {
	Forwarding string
	Token      string
}

/*
ParseRecipient reads an address on the inbound domain. Anything after
the first plus sign of the local part is a tag; a tag shaped like a
reply token is kept as one. Addresses on other domains are not ours.
*/
func ParseRecipient(address, inboundDomain string) (Recipient, bool) {
	local, domain, ok := strings.Cut(strings.ToLower(strings.TrimSpace(address)), "@")
	if !ok || local == "" || inboundDomain == "" || domain != inboundDomain {
		return Recipient{}, false
	}

	base, tag, _ := strings.Cut(local, "+")
	if base == "" {
		return Recipient{}, false
	}

	recipient := Recipient{Forwarding: base + "@" + domain}
	if ValidToken(tag) {
		recipient.Token = tag
	}

	return recipient, true
}

/*
ReplyAddress returns the forwarding address with the reply token in its
local part.
*/
func ReplyAddress(forwarding, token string) string {
	local, domain, _ := strings.Cut(forwarding, "@")

	return local + "+" + token + "@" + domain
}

/*
NewReplyToken returns a fresh random reply token.
*/
func NewReplyToken() string {
	seed := hashing.SHA256([]byte(str.Random(tokenSeed)))

	return base32.NewEncoding(tokenAlphabet).WithPadding(base32.NoPadding).EncodeToString(seed[:tokenBytes])
}

/*
ValidToken reports whether the text has the shape of a reply token.
*/
func ValidToken(token string) bool {
	if len(token) != tokenLength {
		return false
	}

	return !strings.ContainsFunc(token, func(r rune) bool { return !strings.ContainsRune(tokenAlphabet, r) })
}

/*
DefaultLocal returns the local part of a workspace's first forwarding
address: its slug, or the slug with -team when the slug is a name the
install keeps for itself.
*/
func DefaultLocal(slug string) string {
	if Reserved(slug) {
		return slug + "-team"
	}

	return slug
}

/*
LabeledLocal returns the local part of a further forwarding address of
the workspace: its slug, a dot, and a label made from the public
address, such as acme.sales for sales@acme.com.
*/
func LabeledLocal(slug, publicAddress string) string {
	local, _, _ := strings.Cut(strings.ToLower(publicAddress), "@")

	label := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}

		return '-'
	}, local)
	label = strings.Trim(label, "-")

	if label == "" {
		label = "mail"
	}

	return slug + "." + strings.TrimRight(label[:min(len(label), maxLabelLength)], "-")
}

/*
WithSuffix returns the local part with a number after it, for when the
plain one is taken: acme-2, acme.sales-3.
*/
func WithSuffix(local string, n int) string {
	if n <= 1 {
		return local
	}

	return local + "-" + strconv.Itoa(n)
}

/*
Reserved reports whether a local part is one the install never hands
out, because people and mail systems expect it to reach the operator.
*/
func Reserved(local string) bool {
	switch local {
	case "postmaster", "abuse", "hostmaster", "webmaster", "admin", "administrator", "root", "mailer-daemon",
		"noreply", "no-reply", "bounces", "bounce", "support", "security", "dmarc", "info", "mail", "www", "dimmi":
		return true
	}

	return false
}
