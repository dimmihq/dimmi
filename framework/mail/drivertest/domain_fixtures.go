/*
|--------------------------------------------------------------------------
| Provider Domain Fixtures
|--------------------------------------------------------------------------
|
| Each provider's domain API, as its public reference shows it, reduced to
| the fields that matter. The contract runs the drivers against these, and
| application tests can stub the same answers, so both stay in step when
| a staging run corrects a provider's shape. Every fixture's domain is
| example.com.
|
*/

package drivertest

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/mail"
)

/*
DomainToken is the credential every fixture expects.
*/
const DomainToken = "domain-token"

func answer(body string) httpclient.Response {
	return httpclient.Response{Body: []byte(body)}
}

func bearer(req httpclienttest.SentRequest) bool {
	return req.Header.Get("Authorization") == "Bearer "+DomainToken
}

/*
PostmarkDomains is the fixture of Postmark's domains API. It reports a
missing domain with 422 and its own error code, as that API does.
*/
func PostmarkDomains() DomainAPI {
	const (
		base    = "https://api.postmarkapp.com/domains"
		dkimKey = "k=rsa; p=MIGfMA0GCSqGSIb3"
		host    = "20261104pm._domainkey.example.com"
		bounce  = "pm-bounces.example.com"
	)

	domain := func(pending, dkimOK, bounceOK bool) string {
		active, next := host, ""
		if pending {
			active, next = "", host
		}

		return fmt.Sprintf(`{"ID":36736,"Name":"example.com","DKIMVerified":%t,"DKIMHost":%q,"DKIMTextValue":%q,`+
			`"DKIMPendingHost":%q,"DKIMPendingTextValue":%[3]q,"ReturnPathDomain":%[5]q,`+
			`"ReturnPathDomainVerified":%[6]t,"ReturnPathDomainCNAMEValue":"pm.mtasv.net"}`,
			dkimOK, active, dkimKey, next, bounce, bounceOK)
	}
	unverified := answer(domain(true, false, false))

	return DomainAPI{
		ID: "36736",
		Records: []mail.DomainRecord{
			{Purpose: mail.PurposeDKIM, Type: "TXT", Host: host, Value: dkimKey},
			{Purpose: mail.PurposeReturnPath, Type: "CNAME", Host: bounce, Value: "pm.mtasv.net"},
		},
		Create: DomainRoute{"POST " + base, unverified},
		Read:   DomainRoute{"GET " + base + "/36736", unverified},
		Delete: DomainRoute{"DELETE " + base + "/36736", answer(`{"ErrorCode":0,"Message":"Domain removed."}`)},
		Verify: []DomainRoute{
			{"PUT " + base + "/36736/verifyDkim", answer(domain(false, true, false))},
			{"PUT " + base + "/36736/verifyReturnPath", answer(domain(false, true, true))},
		},
		NotFound: httpclient.Response{
			Status: http.StatusUnprocessableEntity,
			Body:   []byte(`{"ErrorCode":510,"Message":"This domain was not found."}`),
		},
		Credential: func(req httpclienttest.SentRequest) bool {
			return req.Header.Get("X-Postmark-Account-Token") == DomainToken
		},
	}
}

/*
ResendDomains is the fixture of Resend's domains API.
*/
func ResendDomains() DomainAPI {
	const (
		id      = "4dd369bc-aa82-4ff3-97de-514ae3000ee0"
		base    = "https://api.resend.com/domains"
		dkimKey = "p=MIGfMA0GCSqGSIb3"
		bounce  = "feedback-smtp.us-east-1.amazonses.com"
		spf     = "v=spf1 include:amazonses.com ~all"
	)

	domain := func(status string) string {
		return fmt.Sprintf(`{"id":%q,"name":"example.com","status":%q,"region":"us-east-1","records":[`+
			`{"record":"DKIM","name":"resend._domainkey","type":"TXT","ttl":"Auto","status":%[2]q,"value":%[3]q},`+
			`{"record":"SPF","name":"send","type":"MX","ttl":"Auto","status":%[2]q,"value":%[4]q,"priority":10},`+
			`{"record":"SPF","name":"send","type":"TXT","ttl":"Auto","status":%[2]q,"value":%[5]q}]}`,
			id, status, dkimKey, bounce, spf)
	}
	unverified := answer(domain("not_started"))

	return DomainAPI{
		ID: id,
		Records: []mail.DomainRecord{
			{Purpose: mail.PurposeDKIM, Type: "TXT", Host: "resend._domainkey.example.com", Value: dkimKey},
			{Purpose: mail.PurposeMX, Type: "MX", Host: "send.example.com", Value: bounce, Priority: 10},
			{Purpose: mail.PurposeSPF, Type: "TXT", Host: "send.example.com", Value: spf},
		},
		Create: DomainRoute{"POST " + base, unverified},
		Read:   DomainRoute{"GET " + base + "/" + id, unverified},
		Delete: DomainRoute{"DELETE " + base + "/" + id, answer(`{"object":"domain","deleted":true}`)},
		Verify: []DomainRoute{
			{"POST " + base + "/" + id + "/verify", answer(`{"object":"domain","id":"` + id + `"}`)},
			{"GET " + base + "/" + id, answer(domain("verified"))},
		},
		NotFound: httpclient.Response{
			Status: http.StatusNotFound,
			Body:   []byte(`{"statusCode":404,"name":"not_found","message":"Domain not found"}`),
		},
		Credential: bearer,
	}
}

/*
MailgunDomains is the fixture of Mailgun's domains API.
*/
func MailgunDomains() DomainAPI {
	const (
		name    = "example.com"
		v4      = "https://api.mailgun.net/v4/domains"
		v3      = "https://api.mailgun.net/v3/domains"
		spf     = "v=spf1 include:mailgun.org ~all"
		dkimKey = "k=rsa; p=MIGfMA0GCSqGSIb3"
	)

	domain := func(state, valid string) string {
		return fmt.Sprintf(`{"message":"ok","domain":{"id":"123","name":%q,"state":%q,"web_prefix":"email"},`+
			`"receiving_dns_records":[{"record_type":"MX","name":"example.com","value":"mxa.mailgun.org",`+
			`"priority":"10","valid":%[3]q}],"sending_dns_records":[`+
			`{"record_type":"TXT","name":"example.com","value":%[4]q,"valid":%[3]q},`+
			`{"record_type":"TXT","name":"k1._domainkey.example.com","value":%[5]q,"valid":%[3]q},`+
			`{"record_type":"CNAME","name":"email.example.com","value":"mailgun.org","valid":%[3]q}]}`,
			name, state, valid, spf, dkimKey)
	}
	unverified := answer(domain("unverified", "unknown"))
	updated := answer(`{"message":"Domain tracking settings have been updated"}`)

	return DomainAPI{
		ID: name,
		Records: []mail.DomainRecord{
			{Purpose: mail.PurposeSPF, Type: "TXT", Host: name, Value: spf},
			{Purpose: mail.PurposeDKIM, Type: "TXT", Host: "k1._domainkey.example.com", Value: dkimKey},
			{Purpose: mail.PurposeTracking, Type: "CNAME", Host: "email.example.com", Value: "mailgun.org"},
		},
		Create: DomainRoute{"POST " + v4, unverified},
		Read:   DomainRoute{"GET " + v4 + "/" + name, unverified},
		Delete: DomainRoute{"DELETE " + v3 + "/" + name, answer(`{"message":"Domain will be deleted"}`)},
		Verify: []DomainRoute{{"PUT " + v4 + "/" + name + "/verify", answer(domain("active", "valid"))}},
		Tracking: []DomainRoute{
			{"PUT " + v3 + "/" + name + "/tracking/click", updated},
			{"PUT " + v3 + "/" + name + "/tracking/open", updated},
		},
		NotFound: httpclient.Response{Status: http.StatusNotFound, Body: []byte(`{"message":"Domain not found"}`)},
		Credential: func(req httpclienttest.SentRequest) bool {
			credentials := base64.StdEncoding.EncodeToString([]byte("api:" + DomainToken))

			return req.Header.Get("Authorization") == "Basic "+credentials
		},
	}
}

/*
SendGridDomains is the fixture of SendGrid's domain authentication API.
*/
func SendGridDomains() DomainAPI {
	const (
		base  = "https://api.sendgrid.com/v3/whitelabel/domains"
		track = "https://api.sendgrid.com/v3/tracking_settings"
		mailC = "u7.wl.sendgrid.net"
		dkim1 = "s1._domainkey.u7.wl.sendgrid.net"
		dkim2 = "s2._domainkey.u7.wl.sendgrid.net"
	)

	domain := func(valid bool) string {
		return fmt.Sprintf(`{"id":45373692,"subdomain":"em123","domain":"example.com","valid":%[1]t,`+
			`"automatic_security":true,"dns":{`+
			`"mail_cname":{"valid":%[1]t,"type":"cname","host":"em123.example.com","data":%[2]q},`+
			`"dkim1":{"valid":%[1]t,"type":"cname","host":"s1._domainkey.example.com","data":%[3]q},`+
			`"dkim2":{"valid":%[1]t,"type":"cname","host":"s2._domainkey.example.com","data":%[4]q}}}`,
			valid, mailC, dkim1, dkim2)
	}
	unverified := answer(domain(false))
	validated := `{"id":45373692,"valid":true,"validation_results":{"mail_cname":{"valid":true,"reason":null},` +
		`"dkim1":{"valid":true,"reason":null},"dkim2":{"valid":true,"reason":null}}}`

	return DomainAPI{
		ID: "45373692",
		Records: []mail.DomainRecord{
			{Purpose: mail.PurposeReturnPath, Type: "CNAME", Host: "em123.example.com", Value: mailC},
			{Purpose: mail.PurposeDKIM, Type: "CNAME", Host: "s1._domainkey.example.com", Value: dkim1},
			{Purpose: mail.PurposeDKIM, Type: "CNAME", Host: "s2._domainkey.example.com", Value: dkim2},
		},
		Create: DomainRoute{"POST " + base, httpclient.Response{Status: http.StatusCreated, Body: unverified.Body}},
		Read:   DomainRoute{"GET " + base + "/45373692", unverified},
		Delete: DomainRoute{"DELETE " + base + "/45373692", httpclient.Response{Status: http.StatusNoContent}},
		Verify: []DomainRoute{
			{"POST " + base + "/45373692/validate", answer(validated)},
			{"GET " + base + "/45373692", answer(domain(true))},
		},
		Tracking: []DomainRoute{
			{"PATCH " + track + "/click", answer(`{"enabled":false}`)},
			{"PATCH " + track + "/open", answer(`{"enabled":false}`)},
		},
		NotFound: httpclient.Response{
			Status: http.StatusNotFound,
			Body:   []byte(`{"errors":[{"message":"authenticated domain not found"}]}`),
		},
		Credential: bearer,
	}
}
