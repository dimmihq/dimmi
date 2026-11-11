package privacy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/erasures"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
traces gives ann a rated article with a note, changelog subscriptions
under her contact and under her bare address, and a broadcast delivered
to her, beside bob's subscription, which must survive her erasure.
*/
func traces(t *testing.T, tc *tests.TestCase, contact ids.UUID) {
	t.Helper()
	ctx, db, ws := t.Context(), tc.Operator(), tc.Workspace.Workspace.ID

	var center, article, broadcast ids.UUID
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO help_centers (workspace_id, name, subdomain)
		VALUES ($1, 'Help', $2) RETURNING id`, ws, testkit.UniqueName("erase-")).Scan(&center))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO articles
		(workspace_id, help_center_id, title, slug, content_hash, position)
		VALUES ($1, $2, 'Refunds', 'refunds', 'h', 1) RETURNING id`, ws, center).Scan(&article))
	require.NoError(t, db.QueryRow(ctx, `INSERT INTO broadcasts (workspace_id, name, channel)
		VALUES ($1, 'Launch', 'email') RETURNING id`, ws).Scan(&broadcast))

	seeds := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO article_ratings (workspace_id, article_id, rater_hash, contact_id, helpful, comment)
			VALUES ($1, $2, 'r1', $3, false, 'Ann here, my refund never came')`, []any{ws, article, contact}},
		{`INSERT INTO changelog_subscribers (workspace_id, help_center_id, email, contact_id, nonce)
			VALUES ($1, $2, 'ann@work.example', $3, 'n1'), ($1, $2, 'ann@example.com', NULL, 'n2'),
			($1, $2, 'bob@example.com', NULL, 'n3')`, []any{ws, center, contact}},
		{
			`INSERT INTO broadcast_recipients (workspace_id, broadcast_id, contact_id, address, reason, reply_token)
			VALUES ($1, $2, $3, 'ann@example.com', 'bounced for ann@example.com', 't1')`,
			[]any{ws, broadcast, contact},
		},
	}
	for _, seed := range seeds {
		_, err := db.Exec(ctx, seed.query, seed.args...)
		require.NoError(t, err)
	}
}

func TestErasureTraces(t *testing.T) {
	t.Parallel()

	t.Run("forgets the customer's article notes, changelog subscriptions and broadcast address", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)
		_, contact := customer(t, d)
		contactID, err := d.TC.App.IDs.Decode(string(models.PrefixContact), contact)
		require.NoError(t, err)
		traces(t, d.TC, contactID)

		erasure := d.Owner.Delete("/api/v1/contacts/" + contact + "?erase=true").AssertStatus(202).String("data.id")
		d.TC.Clock.Travel(8 * 24 * time.Hour)
		assert.Equal(t, erasures.Done, erase(t, d.TC, erasure))

		d.TC.Table("article_ratings").AssertCount(1, testkit.Row{"comment": nil, "helpful": false})
		d.TC.Table("changelog_subscribers").AssertCount(1)
		d.TC.Table("changelog_subscribers").AssertCount(1, testkit.Row{"email": "bob@example.com"})
		d.TC.Table("broadcast_recipients").AssertCount(1, testkit.Row{"address": "", "reason": nil})

		assert.Equal(t, erasures.Done, erase(t, d.TC, erasure))
	})
}
