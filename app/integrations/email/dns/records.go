/*
|--------------------------------------------------------------------------
| Expected Records
|--------------------------------------------------------------------------
|
| What a customer publishes so mail may leave from their own domain. The
| ownership record is always needed: it is what stops one workspace of a
| shared install from sending as another's domain. Beyond it the signing
| record depends on who sends. SES and a relay sign with keys we can
| check, so they are required. A provider the operator set up by hand
| keeps its keys to itself, so its records are only guidance and the
| operator's word stands in for them.
|
| Return path and DMARC never block verification, since mail flows without
| them; they only make it land better, and the page says so. Existing SPF
| is always merged into, never doubled, because two SPF records break all
| mail on the domain.
|
*/

package dns

import (
	"fmt"
	"strings"
)

/*
The drivers a sending domain is set up for.
*/
const (
	DriverSES      = "ses"
	DriverSMTP     = "smtp"
	DriverPostmark = "postmark"
	DriverResend   = "resend"
	DriverMailgun  = "mailgun"
	DriverSendGrid = "sendgrid"
)

/*
What a record is for.
*/
const (
	PurposeOwnership     = "ownership"
	PurposeDKIM          = "dkim"
	PurposeReturnPath    = "return_path"
	PurposeReturnPathSPF = "return_path_spf"
	PurposeSPF           = "spf"
	PurposeDMARC         = "dmarc"
)

/*
What a check found for a record. StatusInfo marks guidance a check never
blocks on.
*/
const (
	StatusVerified    = "verified"
	StatusMismatch    = "mismatch"
	StatusMissing     = "missing"
	StatusPropagating = "propagating"
	StatusError       = "error"
	StatusInfo        = "info"
)

/*
Record types the records use.
*/
const (
	TypeTXT   = "TXT"
	TypeCNAME = "CNAME"
	TypeMX    = "MX"
)

/*
Fixed parts of the records.
*/
const (
	ownershipLabel = "_dimmi-verification"
	ownershipValue = "dimmi-verification="
	dmarcLabel     = "_dmarc"
	dmarcValue     = "v=DMARC1; p=none;"
	sesSPFInclude  = "include:amazonses.com"
	mailFromPref   = 10
)

/*
Record is one DNS record of a domain: what it is for, where it goes and
what it should hold. Host is the fully qualified name; Required says
verification waits on it. Status and Observed hold what the last check
found. Selector names the provider's hint on a guidance record.
*/
type Record struct {
	Purpose  string `json:"purpose"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Value    string `json:"value"`
	Priority int    `json:"priority,omitempty"`
	Required bool   `json:"required"`
	Status   string `json:"status"`
	Observed string `json:"observed,omitempty"`
	Selector string `json:"selector,omitempty"`
}

/*
Spec says what a domain's records are built from. Tokens and Zone are the
three SES signing tokens and the zone they point into. Selector and
PublicKey are a relay's signing key, the key in base64. MailFrom is the
fully qualified return path name and Region the SES region it points at.
SPFInclude is the mechanism a relay needs in the root SPF record, when
the operator named one.
*/
type Spec struct {
	Domain         string
	OwnershipToken string
	Driver         string
	Tokens         []string
	Zone           string
	Selector       string
	PublicKey      string
	MailFrom       string
	Region         string
	SPFInclude     string
}

/*
Expected returns the records the spec's domain needs.
*/
func Expected(s Spec) []Record {
	records := []Record{{
		Purpose: PurposeOwnership, Type: TypeTXT, Host: ownershipLabel + "." + s.Domain,
		Value: ownershipValue + s.OwnershipToken, Required: true, Status: StatusMissing,
	}}

	switch s.Driver {
	case DriverSES:
		records = append(records, sesRecords(s)...)
	case DriverSMTP:
		records = append(records, relayRecords(s)...)
	default:
		records = append(records, manualRecords(s)...)
	}

	return append(records, Record{
		Purpose: PurposeDMARC, Type: TypeTXT, Host: dmarcLabel + "." + s.Domain, Value: dmarcValue,
		Status: StatusMissing,
	})
}

/*
sesRecords are the three signing aliases of Easy DKIM and the custom
return path.
*/
func sesRecords(s Spec) []Record {
	var records []Record

	for _, token := range s.Tokens {
		records = append(records, Record{
			Purpose: PurposeDKIM, Type: TypeCNAME, Host: token + "._domainkey." + s.Domain,
			Value: token + "." + s.Zone, Required: true, Status: StatusMissing,
		})
	}

	if s.MailFrom == "" {
		return records
	}

	return append(records,
		Record{
			Purpose: PurposeReturnPath, Type: TypeMX, Host: s.MailFrom, Priority: mailFromPref,
			Value: "feedback-smtp." + s.Region + ".amazonses.com", Status: StatusMissing,
		},
		Record{
			Purpose: PurposeReturnPathSPF, Type: TypeTXT, Host: s.MailFrom,
			Value: "v=spf1 " + sesSPFInclude + " ~all", Status: StatusMissing,
		},
	)
}

/*
relayRecords are the key we sign with, and the relay's SPF include when
one is known.
*/
func relayRecords(s Spec) []Record {
	records := []Record{{
		Purpose: PurposeDKIM, Type: TypeTXT, Host: s.Selector + "._domainkey." + s.Domain,
		Value: "v=DKIM1; k=rsa; p=" + s.PublicKey, Required: true, Status: StatusMissing,
	}}

	if s.SPFInclude == "" {
		return records
	}

	return append(records, Record{
		Purpose: PurposeSPF, Type: TypeTXT, Host: s.Domain, Value: "v=spf1 " + s.SPFInclude + " ~all",
		Status: StatusMissing,
	})
}

/*
manualHint is what we can say about a provider the operator set up by
hand: its usual signing selector, return path name and SPF include.
*/
type manualHint struct {
	selector   string
	returnPath string
	include    string
}

/*
manualHintFor returns the hint of the driver.
*/
func manualHintFor(driver string) manualHint {
	switch driver {
	case DriverPostmark:
		return manualHint{selector: "<yyyymmddhhmmss>pm", returnPath: "pm-bounces", include: "include:spf.mtasv.net"}
	case DriverResend:
		return manualHint{selector: "resend", returnPath: "send", include: "include:amazonses.com"}
	case DriverMailgun:
		return manualHint{selector: "k1", returnPath: "email", include: "include:mailgun.org"}
	case DriverSendGrid:
		return manualHint{selector: "s1", returnPath: "em<id>", include: "include:sendgrid.net"}
	}

	return manualHint{}
}

/*
manualRecords are guidance only: the values live in the provider's
dashboard, so a check reports what it sees and never blocks on it.
*/
func manualRecords(s Spec) []Record {
	hint := manualHintFor(s.Driver)
	if hint.selector == "" {
		return nil
	}

	return []Record{
		{
			Purpose: PurposeDKIM, Type: TypeTXT, Host: hint.selector + "._domainkey." + s.Domain, Status: StatusInfo,
			Selector: hint.selector,
		},
		{Purpose: PurposeReturnPath, Type: TypeCNAME, Host: hint.returnPath + "." + s.Domain, Status: StatusInfo},
		{
			Purpose: PurposeSPF, Type: TypeTXT, Host: s.Domain, Value: "v=spf1 " + hint.include + " ~all",
			Status: StatusInfo,
		},
	}
}

/*
MergeSPF returns the one SPF record the domain should have: the existing
one with the mechanism added before its closing all, or a new one. An
existing record that already has the mechanism is returned unchanged.
*/
func MergeSPF(existing, mechanism string) string {
	existing = strings.TrimSpace(existing)
	if existing == "" {
		return "v=spf1 " + mechanism + " ~all"
	}

	fields := strings.Fields(existing)
	for _, field := range fields {
		if strings.EqualFold(field, mechanism) {
			return existing
		}
	}

	for i, field := range fields {
		if strings.EqualFold(strings.TrimLeft(field, "+-~?"), "all") {
			merged := append(append(append([]string{}, fields[:i]...), mechanism), fields[i:]...)

			return strings.Join(merged, " ")
		}
	}

	return existing + " " + mechanism
}

/*
TXTChunks splits a TXT value into the strings of at most 255 bytes one
DNS record holds.
*/
func TXTChunks(value string) []string {
	const limit = 255

	var chunks []string

	for len(value) > limit {
		chunks, value = append(chunks, value[:limit]), value[limit:]
	}

	return append(chunks, value)
}

/*
BIND returns the records as a zone file snippet, with guidance records
that have no value left out. Long TXT values are split into quoted
strings, as a zone file expects.
*/
func BIND(records []Record) string {
	var b strings.Builder

	for _, r := range records {
		if r.Value == "" {
			continue
		}

		fmt.Fprintf(&b, "%s. 3600 IN %s %s\n", r.Host, r.Type, bindValue(r))
	}

	return b.String()
}

/*
bindValue returns the record's value as a zone file writes it.
*/
func bindValue(r Record) string {
	switch r.Type {
	case TypeMX:
		return fmt.Sprintf("%d %s.", r.Priority, r.Value)
	case TypeCNAME:
		return r.Value + "."
	}

	var quoted []string
	for _, chunk := range TXTChunks(r.Value) {
		quoted = append(quoted, `"`+chunk+`"`)
	}

	return strings.Join(quoted, " ")
}
