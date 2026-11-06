/*
|--------------------------------------------------------------------------
| Forwarding Test Resource
|--------------------------------------------------------------------------
|
| A forwarding test as the settings page shows it: where its mail went,
| how far it got, how long the forward took, and what it learned, each
| finding with a plain message saying what to do. The token never leaves
| the server, so a test mail cannot be made from the API's answers.
|
*/

package resources

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"

	"github.com/dimmihq/dimmi/app/models"
)

/*
EmailAddressTest is a forwarding test of an address. SentTo is the
public address the mail went to, MessageID the Message-ID of that mail,
empty before it left. LatencyMS is how long the mail took to come back.
*/
type EmailAddressTest struct {
	ID           string        `json:"id"`
	Object       string        `json:"object"`
	EmailAddress string        `json:"email_address"`
	Status       string        `json:"status"`
	SentTo       string        `json:"sent_to"`
	MessageID    string        `json:"message_id"`
	Findings     []TestFinding `json:"findings"`
	LatencyMS    *int32        `json:"latency_ms"`
	ExpiresAt    time.Time     `json:"expires_at"`
	CompletedAt  *time.Time    `json:"completed_at"`
	CreatedAt    time.Time     `json:"created_at"`
}

/*
TestFinding is one thing a forwarding test learned, with its severity
(error, warn or info) and what it means in plain words.
*/
type TestFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

/*
NewEmailAddressTest returns the resource of a test whose mail went to
the public address with the Message-ID.
*/
func NewEmailAddressTest(ctx context.Context, test models.EmailAddressTest, sentTo, messageID string) EmailAddressTest {
	return EmailAddressTest{
		ID: models.PublicID(models.PrefixEmailAddressTest, test.ID), Object: "email_address_test",
		EmailAddress: models.PublicID(models.PrefixEmailAddress, test.EmailAddressID), Status: test.Status,
		SentTo: sentTo, MessageID: messageID, Findings: testFindings(ctx, test.Findings),
		LatencyMS: test.LatencyMs, ExpiresAt: test.ExpiresAt, CompletedAt: test.CompletedAt, CreatedAt: test.CreatedAt,
	}
}

/*
testFindings reads the stored findings and words each one.
*/
func testFindings(ctx context.Context, raw json.RawMessage) []TestFinding {
	findings := []TestFinding{}
	if err := json.Unmarshal(raw, &findings); err != nil {
		return []TestFinding{}
	}

	for i, f := range findings {
		findings[i].Message = i18n.T(ctx, "email_setup.findings."+f.Code, nil)
	}

	return findings
}
