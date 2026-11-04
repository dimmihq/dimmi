package email_test

import (
	"context"
	"io/fs"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
	fixtures "github.com/dimmihq/dimmi/tests/fixtures/email"
)

/*
Settings every email test runs with: the inbound domain forwarding
addresses live on, and the secret of the test inbound source.
*/
const (
	inboundDomain = "inbound.test"
	testSecret    = "inbound-test-secret"
)

/*
emailCase boots the application receiving mail on the test inbound
domain through the test source, with DNS answered by a fake zone so no
lookup leaves the machine.
*/
func emailCase(t *testing.T, opts ...tests.Option) (*tests.TestCase, *dns.Fake) {
	t.Helper()

	env := tests.WithEnv(
		"DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN="+inboundDomain,
		"DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=test",
		"DIMMI_CHANNELS__EMAIL__INBOUND__TEST_SECRET="+testSecret,
	)

	tc := tests.New(t, append([]tests.Option{env}, opts...)...)

	return tc, tc.App.DNS.Fake()
}

/*
fixture reads a raw email from the fixtures, with its placeholders
filled in.
*/
func fixture(t *testing.T, name string, values map[string]string) []byte {
	t.Helper()

	raw, err := fs.ReadFile(fixtures.Files, name)
	require.NoError(t, err)

	pairs := make([]string, 0, len(values)*2)
	for key, value := range values {
		pairs = append(pairs, "{{"+key+"}}", value)
	}

	return []byte(strings.NewReplacer(pairs...).Replace(string(raw)))
}

/*
deliver hands a raw email to the test inbound source, the way a
receiving provider would, and runs the processing it queues. Query
holds the envelope recipients and verdicts.
*/
func deliver(tc *tests.TestCase, raw []byte, query string) *testkit.Response {
	tc.T.Helper()

	path := "/webhooks/test/inbound"
	if query != "" {
		path += "?" + query
	}

	res := tc.Client().WithHeader("Authorization", "Bearer "+testSecret).Send("POST", path, raw)
	tc.Drain()

	return res
}

/*
query runs the function with the workspace's queries, as the workspace
sees its rows.
*/
func query[T any](
	tc *tests.TestCase, workspaceID ids.UUID, fn func(ctx context.Context, q *models.Queries) (T, error),
) T {
	tc.T.Helper()

	var out T

	ctx := tenancy.With(tc.T.Context(), workspaceID.String())
	err := tc.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = fn(ctx, models.New(tx))

		return err
	})
	require.NoError(tc.T, err)

	return out
}

/*
defaultAddress returns the workspace's default email address.
*/
func defaultAddress(tc *tests.TestCase) models.EmailAddress {
	tc.T.Helper()

	ws := tc.Workspace.Workspace.ID

	return query(tc, ws, func(ctx context.Context, q *models.Queries) (models.EmailAddress, error) {
		return q.FindDefaultEmailAddress(ctx, ws)
	})
}

/*
conversations returns the workspace's conversations, newest first.
*/
func conversations(tc *tests.TestCase) []models.Conversation {
	tc.T.Helper()

	ws := tc.Workspace.Workspace.ID

	return query(tc, ws, func(ctx context.Context, q *models.Queries) ([]models.Conversation, error) {
		return q.ListConversations(ctx, models.ListConversationsParams{WorkspaceID: ws, RowLimit: 100})
	})
}

/*
messages returns a conversation's timeline, oldest first.
*/
func messages(tc *tests.TestCase, conversationID ids.UUID) []models.Message {
	tc.T.Helper()

	ws := tc.Workspace.Workspace.ID

	return query(tc, ws, func(ctx context.Context, q *models.Queries) ([]models.Message, error) {
		return q.ListMessages(ctx, models.ListMessagesParams{
			WorkspaceID: ws, ConversationID: conversationID, RowLimit: 100,
		})
	})
}

/*
inboundEmails returns what reached the address, newest first.
*/
func inboundEmails(tc *tests.TestCase, address models.EmailAddress) []models.InboundEmail {
	tc.T.Helper()

	return query(tc, address.WorkspaceID, func(ctx context.Context, q *models.Queries) ([]models.InboundEmail, error) {
		return q.ListAddressInboundEmails(ctx, models.ListAddressInboundEmailsParams{
			WorkspaceID: address.WorkspaceID, EmailAddressID: &address.ID, RowLimit: 100,
		})
	})
}

/*
lastInbound returns the newest email that reached the address.
*/
func lastInbound(tc *tests.TestCase, address models.EmailAddress) models.InboundEmail {
	tc.T.Helper()

	rows := inboundEmails(tc, address)
	require.NotEmpty(tc.T, rows, "no email reached %s", address.ForwardingAddress)

	return rows[0]
}

/*
to returns the query that delivers to the addresses, with the extra
parameters given as name and value pairs.
*/
func to(address string, extra ...string) string {
	values := url.Values{"to": {address}}
	for i := 0; i+1 < len(extra); i += 2 {
		values.Add(extra[i], extra[i+1])
	}

	return values.Encode()
}
