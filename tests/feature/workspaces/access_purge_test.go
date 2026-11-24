package workspaces_test

import (
	"testing"
	"time"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/tests"
)

func TestPurgeExpiredAccessRecords(t *testing.T) {
	t.Parallel()

	t.Run("deletes invitations, tokens and idempotency keys that ended and keeps the rest", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws := tc.Workspace.Workspace.ID
		_, member := join(t, tc, enums.RoleMember)
		now := tc.Clock.Now()
		long, short, ahead := now.Add(-31*24*time.Hour), now.Add(-24*time.Hour), now.Add(24*time.Hour)

		exec(t, tc, `INSERT INTO invitations (workspace_id, email, role, token_hash, status, expires_at, last_sent_at,
			revoked_at) VALUES
			($1, 'expired@example.com', 'member', '\x01', 'pending', $2, $2, NULL),
			($1, 'revoked@example.com', 'member', '\x02', 'revoked', $4, $2, $2),
			($1, 'lapsed@example.com', 'member', '\x03', 'pending', $3, $3, NULL),
			($1, 'waiting@example.com', 'member', '\x04', 'pending', $4, $3, NULL),
			($1, 'joined@example.com', 'member', '\x05', 'accepted', $2, $2, NULL)`, ws, long, short, ahead)
		exec(t, tc, `INSERT INTO personal_access_tokens
			(id, owner_id, tenant_id, name, hint, token_hash, created_at, expires_at, revoked_at) VALUES
			(uuidv7(), $1, $2, 'revoked', 'x', '\x01', $3, NULL, $3),
			(uuidv7(), $1, $2, 'expired', 'x', '\x02', $3, $3, NULL),
			(uuidv7(), $1, $2, 'just revoked', 'x', '\x03', $3, NULL, $4),
			(uuidv7(), $1, $2, 'live', 'x', '\x04', $3, $5, NULL),
			(uuidv7(), $1, $2, 'for ever', 'x', '\x05', $3, NULL, NULL)`, member.ID, ws.String(), long, short, ahead)
		exec(t, tc, `INSERT INTO idempotency_keys (tenant_id, scope, key, fingerprint, created_at, expires_at) VALUES
			($1, 'POST /x', 'old', 'f', $2, $3), ($1, 'POST /x', 'new', 'f', $3, $4)`, ws.String(), long, short, ahead)

		push(tc, jobs.PurgeExpiredAccessRecordsArgs{})

		tc.Table("invitations").AssertCount(3, testkit.Row{"workspace_id": ws})
		tc.Table("invitations").AssertCount(0, testkit.Row{"email": "expired@example.com"}).
			AssertCount(0, testkit.Row{"email": "revoked@example.com"}).
			AssertCount(1, testkit.Row{"email": "lapsed@example.com"}).
			AssertCount(1, testkit.Row{"email": "waiting@example.com"}).
			AssertCount(1, testkit.Row{"email": "joined@example.com"})
		tc.Table("personal_access_tokens").AssertCount(0, testkit.Row{"name": "revoked"}).
			AssertCount(0, testkit.Row{"name": "expired"}).
			AssertCount(1, testkit.Row{"name": "just revoked"}).
			AssertCount(1, testkit.Row{"name": "live"}).
			AssertCount(1, testkit.Row{"name": "for ever"})
		tc.Table("idempotency_keys").AssertCount(0, testkit.Row{"key": "old"}).AssertCount(1, testkit.Row{"key": "new"})
	})
}
