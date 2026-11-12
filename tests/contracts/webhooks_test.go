package contracts_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/webhooks"
	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
webhookContract is what a webhook consumer depends on: the envelope every
request carries and each event with the kind of object in its data. The
objects themselves are the API's resources, kept by the REST contract.
*/
type webhookContract struct {
	Envelope map[string]any               `json:"envelope"`
	Events   []appevents.WebhookEventType `json:"events"`
}

/*
TestWebhookContract keeps the webhook events and envelope in a golden
file that scripts/api-compat.ts compares with the last release, so an
event is never renamed or dropped and the envelope never loses a field.
*/
func TestWebhookContract(t *testing.T) {
	t.Parallel()

	t.Run("webhook events and their envelope stay as consumers were built against", func(t *testing.T) {
		t.Parallel()

		envelope, err := routing.Schemas[webhooks.Envelope]()
		require.NoError(t, err)

		contract := webhookContract{Envelope: envelope, Events: appevents.WebhookCatalog()}

		out, err := json.MarshalIndent(contract, "", "  ")
		require.NoError(t, err)

		testkit.Golden(t, "webhooks_v1.json", append(out, '\n'))
		hintOnFailure(t)
	})
}

/*
hintOnFailure tells the author what a contract change asks of them.
*/
func hintOnFailure(t *testing.T) {
	t.Helper()

	if t.Failed() {
		t.Log("The public contract changed. Run make api-snapshot, then make api-compat, " +
			"make sdk-generate and make sdk-check, and add a changeset (pnpm changeset). See AGENTS.md.")
	}
}
