package contacts_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

func TestInboundSenderRace(t *testing.T) {
	t.Parallel()

	t.Run("messages from one new sender taken in at once make one contact", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		ws := d.TC.Workspace.Workspace.ID
		receiver := d.TC.App.Actions.ProcessInboundEmail.Receiver

		var wg sync.WaitGroup
		errs := make([]error, 4)
		for i := range errs {
			wg.Go(func() {
				_, errs[i] = receiver.ReceiveInboundMessage(tenancy.With(t.Context(), ws.String()),
					contracts.InboundMessage{
						ConnectionID: d.Inbox.API.ID, DedupeKey: fmt.Sprintf("race-%d", i),
						Sender: contracts.InboundSender{Name: "Ann", Identities: []contracts.IdentityClaim{
							{Type: enums.IdentityTypeEmail, Value: "ann@example.com"},
						}},
						Subject: "Burst", Body: fmt.Sprintf("part %d", i),
					})
			})
		}
		wg.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}
		var contacts int
		require.NoError(t, d.TC.Operator().QueryRow(t.Context(),
			"SELECT count(DISTINCT contact_id) FROM contact_identities WHERE workspace_id = $1 AND value = $2",
			ws, "ann@example.com").Scan(&contacts))
		assert.Equal(t, 1, contacts)
	})
}
