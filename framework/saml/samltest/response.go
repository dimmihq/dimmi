/*
|--------------------------------------------------------------------------
| Test Answers
|--------------------------------------------------------------------------
|
| Builds the answer an identity provider posts back, signed, then broken
| on request. Each knob maps to one way an answer goes wrong, so a test
| names the attack it checks.
|
*/

package samltest

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/beevik/etree"
)

/*
Response describes an answer. Destination, Audience and Recipient default
to the ACS and entity id the test passes; Issuer to the identity
provider. IssuedAt is when it was made and Validity how long it lasts,
five minutes when zero. SignResponse signs the envelope, SignAssertion
the assertion; with neither it goes unsigned. The tamper fields break it
after signing; NoAudience leaves the audience out and
ConfirmationInResponseTo makes the subject confirmation name another
request than the response does.
*/
type Response struct {
	ACSURL       string
	Audience     string
	Destination  string
	Issuer       string
	InResponseTo string
	NameID       string
	NameIDFormat string
	Attributes   map[string][]string
	IssuedAt     time.Time
	Validity     time.Duration
	Status       string

	SignResponse  bool
	SignAssertion bool

	ExtraUnsignedAssertion   bool
	WrapSignature            bool
	NameIDComment            int
	NoAudience               bool
	ConfirmationInResponseTo string
}

/*
Encode builds the answer and returns it base64 encoded, as posted.
*/
func (i *IDP) Encode(r Response) (string, error) {
	doc, err := i.build(r)
	if err != nil {
		return "", err
	}

	out, err := doc.WriteToBytes()
	if err != nil {
		return "", fmt.Errorf("write test answer: %w", err)
	}

	return base64.StdEncoding.EncodeToString(out), nil
}

/*
build returns the answer as a document, signed and tampered as asked.
*/
func (i *IDP) build(r Response) (*etree.Document, error) {
	r = defaults(r, i)

	doc := etree.NewDocument()
	response := envelope(r)
	doc.SetRoot(response)

	assertion := i.assertion(r)
	if r.SignAssertion {
		signed, err := i.signer().SignEnveloped(assertion)
		if err != nil {
			return nil, fmt.Errorf("sign test assertion: %w", err)
		}

		assertion = signed
	}

	response.AddChild(assertion)

	if r.SignResponse {
		signed, err := i.signer().SignEnveloped(response)
		if err != nil {
			return nil, fmt.Errorf("sign test answer: %w", err)
		}

		doc.SetRoot(signed)
		response = signed
	}

	tamper(r, i, response)

	return doc, nil
}

/*
defaults fills in what the test left out.
*/
func defaults(r Response, i *IDP) Response {
	if r.Destination == "" {
		r.Destination = r.ACSURL
	}

	if r.Issuer == "" {
		r.Issuer = i.EntityID
	}

	if r.Validity == 0 {
		r.Validity = 5 * time.Minute
	}

	if r.Status == "" {
		r.Status = statusOK
	}

	if r.NameIDFormat == "" {
		r.NameIDFormat = EmailFormat
	}

	return r
}

/*
envelope returns the response element without its assertion.
*/
func envelope(r Response) *etree.Element {
	response := etree.NewElement("samlp:Response")
	response.CreateAttr("xmlns:samlp", protocolNS)
	response.CreateAttr("xmlns:saml", assertionNS)
	response.CreateAttr("ID", newID())
	response.CreateAttr("Version", "2.0")
	response.CreateAttr("IssueInstant", r.IssuedAt.UTC().Format(timeLayout))
	response.CreateAttr("Destination", r.Destination)

	if r.InResponseTo != "" {
		response.CreateAttr("InResponseTo", r.InResponseTo)
	}

	response.CreateElement("saml:Issuer").SetText(r.Issuer)
	response.CreateElement("samlp:Status").CreateElement("samlp:StatusCode").CreateAttr("Value", r.Status)

	return response
}

/*
assertion returns the assertion element about the person.
*/
func (i *IDP) assertion(r Response) *etree.Element {
	start, end := r.IssuedAt.UTC(), r.IssuedAt.Add(r.Validity).UTC()

	a := etree.NewElement("saml:Assertion")
	a.CreateAttr("xmlns:saml", assertionNS)
	a.CreateAttr("ID", newID())
	a.CreateAttr("Version", "2.0")
	a.CreateAttr("IssueInstant", start.Format(timeLayout))
	a.CreateElement("saml:Issuer").SetText(r.Issuer)

	subject(a, r, end)

	conditions := a.CreateElement("saml:Conditions")
	conditions.CreateAttr("NotBefore", start.Format(timeLayout))
	conditions.CreateAttr("NotOnOrAfter", end.Format(timeLayout))
	if !r.NoAudience {
		conditions.CreateElement("saml:AudienceRestriction").CreateElement("saml:Audience").SetText(r.Audience)
	}

	authn := a.CreateElement("saml:AuthnStatement")
	authn.CreateAttr("AuthnInstant", start.Format(timeLayout))

	if len(r.Attributes) > 0 {
		statement := a.CreateElement("saml:AttributeStatement")
		for key, values := range r.Attributes {
			attribute := statement.CreateElement("saml:Attribute")
			attribute.CreateAttr("Name", key)

			for _, value := range values {
				attribute.CreateElement("saml:AttributeValue").SetText(value)
			}
		}
	}

	return a
}

/*
subject writes who the assertion is about and how they confirm it.
*/
func subject(a *etree.Element, r Response, end time.Time) {
	subject := a.CreateElement("saml:Subject")
	name := subject.CreateElement("saml:NameID")
	name.CreateAttr("Format", r.NameIDFormat)
	nameID(name, r)

	confirmation := subject.CreateElement("saml:SubjectConfirmation")
	confirmation.CreateAttr("Method", subjectMethod)
	data := confirmation.CreateElement("saml:SubjectConfirmationData")
	data.CreateAttr("NotOnOrAfter", end.Format(timeLayout))
	data.CreateAttr("Recipient", r.ACSURL)

	if inResponseTo := cmpOr(r.ConfirmationInResponseTo, r.InResponseTo); inResponseTo != "" {
		data.CreateAttr("InResponseTo", inResponseTo)
	}
}

/*
nameID writes the name, split by a comment when the test asks.
*/
func nameID(el *etree.Element, r Response) {
	if r.NameIDComment <= 0 || r.NameIDComment >= len(r.NameID) {
		el.SetText(r.NameID)

		return
	}

	el.SetText(r.NameID[:r.NameIDComment])
	el.CreateComment("comment")
	el.CreateText(r.NameID[r.NameIDComment:])
}

/*
tamper breaks a signed answer the ways the test asked for.
*/
func tamper(r Response, i *IDP, response *etree.Element) {
	if r.ExtraUnsignedAssertion {
		extra := r
		extra.NameID = "intruder@attacker.example"
		response.AddChild(i.assertion(extra))
	}

	if r.WrapSignature {
		signed := response.FindElement("./Assertion")
		if signed == nil {
			return
		}

		evil := signed.Copy()
		evil.FindElement("./Subject/NameID").SetText("intruder@attacker.example")

		if signature := evil.FindElement("./Signature"); signature != nil {
			evil.RemoveChild(signature)
		}

		response.RemoveChild(signed)
		extensions := response.CreateElement("samlp:Extensions")
		extensions.AddChild(signed)
		response.AddChild(evil)
	}
}

/*
cmpOr returns the first value that is not empty.
*/
func cmpOr(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}
