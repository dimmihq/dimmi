package database_test

import (
	"context"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/database/migrations"
	"github.com/dimmihq/dimmi/tests"
)

/*
open connects to the database at the URL for the test.
*/
func open(t *testing.T, databaseURL string) *database.DB {
	t.Helper()

	db, err := database.Open(t.Context(), database.Config{
		URL: databaseURL, MaxConns: 2, PoolMode: database.PoolModeSession, TenantSetting: "app.tenant_id",
	})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return db
}

/*
exec runs a statement and returns its error.
*/
func exec(ctx context.Context, q database.Querier, sql string, args ...any) error {
	_, err := q.Exec(ctx, sql, args...)

	return err
}

/*
checkValues returns the values a strict CHECK constraint on the column
allows, read from the catalog.
*/
func checkValues(t *testing.T, db *database.DB, table, column string) []string {
	t.Helper()

	var definition string
	err := db.QueryRow(t.Context(), `
		SELECT pg_get_constraintdef(c.oid) FROM pg_constraint c
		JOIN pg_class r ON r.oid = c.conrelid
		WHERE r.relname = $1 AND c.contype = 'c' AND pg_get_constraintdef(c.oid) LIKE '%(' || $2 || ' = ANY%'`,
		table, column).Scan(&definition)
	require.NoError(t, err, "check on %s.%s", table, column)

	var values []string
	for _, match := range regexp.MustCompile(`'([a-z_]+)'::text`).FindAllStringSubmatch(definition, -1) {
		values = append(values, match[1])
	}

	return values
}

/*
asStrings returns the enum values as plain strings.
*/
func asStrings[E ~string](values []E) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, string(v))
	}

	return out
}

func TestMigrations(t *testing.T) {
	t.Parallel()

	t.Run("only expand the schema, in ways a live database can take", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, migrate.Lint(migrations.Files()))
	})

	t.Run("run on an empty database, roll back, and run again", func(t *testing.T) {
		t.Parallel()
		empty := testkit.NewDatabase(t)
		runner := tests.Migrator(empty.URL)
		files, err := fs.Glob(migrations.Files(), "*.sql")
		require.NoError(t, err)

		_, err = runner.Up(t.Context())
		require.NoError(t, err)

		for range files {
			_, err := runner.Rollback(t.Context())
			require.NoError(t, err)
		}

		_, err = runner.Up(t.Context())
		require.NoError(t, err)
	})
}

func TestSchema(t *testing.T) {
	t.Parallel()

	db := open(t, tests.MigratedDatabase(t).URL)

	t.Run("allows exactly the values of each strict enum", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, asStrings(enums.Roles()), checkValues(t, db, "workspace_members", "role"))
		assert.Equal(t, asStrings(enums.MemberStatuses()), checkValues(t, db, "workspace_members", "status"))
		assert.Equal(t, asStrings(enums.Availabilities()),
			checkValues(t, db, "workspace_members", "availability"))
		assert.Equal(t, asStrings(enums.InvitationStatuses()), checkValues(t, db, "invitations", "status"))
		assert.Equal(t, asStrings(enums.AuthFactorStatuses()), checkValues(t, db, "user_auth_factors", "status"))
		assert.Equal(t, asStrings(enums.OneTimePurposes()), checkValues(t, db, "one_time_tokens", "purpose"))
		assert.Equal(t, []string{"admin", "member"}, checkValues(t, db, "invitations", "role"))
		assert.Equal(t, asStrings(enums.ContactKinds()), checkValues(t, db, "contacts", "kind"))
		assert.Equal(t, asStrings(enums.ConversationStatuses()), checkValues(t, db, "conversations", "status"))
		assert.Equal(t, asStrings(enums.MessageTypes()), checkValues(t, db, "messages", "type"))
		assert.Equal(t, asStrings(enums.AuthorTypes()), checkValues(t, db, "messages", "author_type"))
		assert.Equal(t, asStrings(enums.ParticipantRoles()),
			checkValues(t, db, "conversation_participants", "role"))
		assert.Equal(t, asStrings(enums.DeliveryStatuses()), checkValues(t, db, "messages", "delivery_status"))
		assert.Equal(t, asStrings(enums.InboundEmailStatuses()), checkValues(t, db, "inbound_emails", "status"))
		assert.Equal(t, asStrings(enums.EmailClassifications()),
			checkValues(t, db, "inbound_emails", "classification"))
		assert.Equal(t, asStrings(enums.OutboundEmailKinds()), checkValues(t, db, "outbound_emails", "kind"))
		assert.Equal(t, asStrings(enums.OutboundEmailStatuses()), checkValues(t, db, "outbound_emails", "status"))
		assert.Equal(t, asStrings(enums.EmailSuppressionReasons()),
			checkValues(t, db, "email_suppressions", "reason"))
		assert.Equal(t, asStrings(enums.EmailSuppressionScopes()), checkValues(t, db, "email_suppressions", "scope"))
		assert.Equal(t, asStrings(enums.ReplyToModes()), checkValues(t, db, "email_addresses", "reply_to_mode"))
		assert.Equal(t, asStrings(enums.FromNameStyles()), checkValues(t, db, "email_addresses", "from_name_style"))
	})

	t.Run("knows every enum value it stores", func(t *testing.T) {
		t.Parallel()

		for _, valid := range []bool{
			enums.RoleOwner.Valid(), enums.MemberStatusActive.Valid(), enums.AvailabilityAway.Valid(),
			enums.InvitationStatusExpired.Valid(), enums.AuthFactorStatusPending.Valid(),
			enums.OneTimePurposeConfirmCode.Valid(), enums.SignupModeInviteOnly.Valid(),
			enums.ContactKindLead.Valid(), enums.ConversationStatusSnoozed.Valid(), enums.MessageTypeNote.Valid(),
			enums.AuthorTypeIntegration.Valid(), enums.ParticipantRoleCC.Valid(),
		} {
			assert.True(t, valid)
		}

		for _, valid := range []bool{
			enums.Role("guest").Valid(), enums.MemberStatus("").Valid(), enums.Availability("busy").Valid(),
			enums.InvitationStatus("sent").Valid(), enums.AuthFactorStatus("lost").Valid(),
			enums.OneTimePurpose("login").Valid(), enums.SignupMode("closed").Valid(),
			enums.ContactKind("visitor").Valid(), enums.ConversationStatus("pending").Valid(),
			enums.MessageType("reply").Valid(), enums.AuthorType("bot").Valid(), enums.ParticipantRole("to").Valid(),
		} {
			assert.False(t, valid)
		}
	})

	t.Run("stores every registered value and refuses one of the wrong shape", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		providers := []string{"github", "google"}
		registered := slices.Concat(appauth.PrimaryMethods(providers).Values(), appauth.CreatedVia(providers).Values())

		for _, value := range registered {
			require.NoError(t, exec(ctx, db, "INSERT INTO users (name, email, created_via) VALUES ('a', $1, $2)",
				value+"."+testkit.UniqueName("")+"@example.com", value), value)
		}

		user := seedUser(t, db, "events")
		for _, event := range appauth.SecurityEvents().Values() {
			require.NoError(t, exec(ctx, db,
				"INSERT INTO security_events (user_id, type) VALUES ($1, $2)", user, event), event)
		}

		require.Error(t, exec(ctx, db,
			"INSERT INTO users (name, email, created_via) VALUES ('a', 'bad@example.com', 'Bad-Value')"))
		require.Error(t, exec(ctx, db, "INSERT INTO security_events (user_id, type) VALUES ($1, 'logout')", user))
	})

	t.Run("keeps one pending invitation per address and workspace", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		workspace := seedWorkspace(t, db)
		insert := "INSERT INTO invitations (workspace_id, email, role, token_hash, expires_at, last_sent_at, status) " +
			"VALUES ($1, 'Grace@example.com', 'member', $2, now() + interval '7 days', now(), $3)"

		require.NoError(t, exec(ctx, db, insert, workspace, []byte("t1"), "pending"))
		require.NoError(t, exec(ctx, db, insert, workspace, []byte("t2"), "revoked"))
		require.Error(t, exec(ctx, db, insert, workspace, []byte("t3"), "pending"))
		require.Error(t, exec(ctx, db, insert, workspace, []byte("t4"), "owner"))
	})

	t.Run("links a provider account to one user and a user to one account per provider", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		ada, grace := seedUser(t, db, "ada"), seedUser(t, db, "grace")
		insert := "INSERT INTO oauth_identities (user_id, provider, provider_user_id) VALUES ($1, 'github', $2)"

		require.NoError(t, exec(ctx, db, insert, ada, "1"))
		require.Error(t, exec(ctx, db, insert, grace, "1"))
		require.Error(t, exec(ctx, db, insert, ada, "2"))
	})

	t.Run("keeps one password reset per user", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		user := seedUser(t, db, "reset")
		insert := "INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, now())"

		require.NoError(t, exec(ctx, db, insert, user, []byte("r1")))
		require.Error(t, exec(ctx, db, insert, user, []byte("r2")))
	})

	t.Run("keeps one live set of recovery codes and one preferred factor per user", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		user := seedUser(t, db, "factors")
		insert := "INSERT INTO user_auth_factors (user_id, type, name, status, is_preferred) " +
			"VALUES ($1, $2, 'x', $3, $4)"

		require.NoError(t, exec(ctx, db, insert, user, "recovery_code", "active", false))
		require.NoError(t, exec(ctx, db, insert, user, "recovery_code", "pending", false))
		require.Error(t, exec(ctx, db, insert, user, "recovery_code", "active", false))
		require.NoError(t, exec(ctx, db, insert, user, "totp", "active", true))
		require.Error(t, exec(ctx, db, insert, user, "webauthn", "active", true))
		require.Error(t, exec(ctx, db, insert, user, "Bad-Value", "active", false))
	})

	t.Run("never lets the application edit the audit log", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		workspace := seedWorkspace(t, db)
		require.NoError(t, exec(ctx, db, "INSERT INTO audit_logs (workspace_id, event, actor_type, actor_label) "+
			"VALUES ($1, 'x.y', 'system', 'System')", workspace))

		err := exec(ctx, db, "UPDATE audit_logs SET event = 'changed.event'")

		require.ErrorContains(t, err, "permission denied")
	})
}

func TestRowLevelSecurity(t *testing.T) {
	t.Parallel()

	t.Run("hides tenant rows from the application role but lets the lookups through", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		migrated := tests.MigratedDatabase(t)
		owner := open(t, migrated.URL)
		workspace := seedWorkspace(t, owner)
		user := seedUser(t, owner, "member")
		require.NoError(t, exec(ctx, owner,
			"INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')", workspace, user))
		app := open(t, appRole(t, migrated))

		var hidden, visible, listed int
		require.NoError(t, app.QueryRow(ctx, "SELECT count(*) FROM workspace_members").Scan(&hidden))
		err := app.Transaction(tenant(ctx, workspace), func(ctx context.Context, tx *database.Tx) error {
			return tx.QueryRow(ctx, "SELECT count(*) FROM workspace_members").Scan(&visible)
		})
		require.NoError(t, err)
		require.NoError(t, app.QueryRow(ctx, "SELECT count(*) FROM auth_user_memberships($1)", user).Scan(&listed))

		assert.Equal(t, 0, hidden)
		assert.Equal(t, 1, visible)
		assert.Equal(t, 1, listed)
	})

	t.Run("lets the application role end a user's tokens in every workspace, and nobody else's", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		migrated := tests.MigratedDatabase(t)
		owner := open(t, migrated.URL)
		user, other := seedUser(t, owner, "holder"), seedUser(t, owner, "bystander")
		for _, holder := range []string{user, user, other} {
			workspace := seedWorkspace(t, owner)
			var member string
			require.NoError(t, owner.QueryRow(ctx, "INSERT INTO workspace_members (workspace_id, user_id, role) "+
				"VALUES ($1, $2, 'member') RETURNING id::text", workspace, holder).Scan(&member))
			require.NoError(t, exec(ctx, owner, "INSERT INTO personal_access_tokens "+
				"(id, owner_id, tenant_id, name, hint, token_hash, created_at) "+
				"VALUES (uuidv7(), $1, $2, 'CI', 'dmi_live_x', uuid_send(uuidv7()), now())", member, workspace))
		}
		app := open(t, appRole(t, migrated))

		var revoked int64
		require.NoError(t, app.QueryRow(ctx, "SELECT auth_revoke_user_tokens($1, now())", user).Scan(&revoked))

		var live int
		require.NoError(t, owner.QueryRow(ctx,
			"SELECT count(*) FROM personal_access_tokens WHERE revoked_at IS NULL").Scan(&live))
		assert.Equal(t, int64(2), revoked)
		assert.Equal(t, 1, live)
	})
}

/*
appRole creates a role that is not the tables' owner, as the cloud's
application role is, and returns the database URL that connects as it.
*/
func appRole(t *testing.T, db testkit.Database) string {
	t.Helper()

	return db.NewRole(t, "SELECT ON ALL TABLES IN SCHEMA public").URL
}

/*
seedUser inserts a bare account and returns its id.
*/
func seedUser(t *testing.T, db *database.DB, name string) string {
	t.Helper()

	var id string
	err := db.QueryRow(t.Context(), "INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id::text",
		name, name+"."+strings.ToLower(testkit.UniqueName(""))+"@example.com").Scan(&id)
	require.NoError(t, err)

	return id
}

/*
seedWorkspace inserts a bare organization and workspace and returns the
workspace's id.
*/
func seedWorkspace(t *testing.T, db *database.DB) string {
	t.Helper()

	var id string
	err := db.QueryRow(t.Context(), `
		WITH org AS (INSERT INTO organizations (name) VALUES ('Acme') RETURNING id)
		INSERT INTO workspaces (organization_id, name, slug) SELECT id, 'Acme', $1 FROM org RETURNING id::text`,
		testkit.UniqueName("acme-")).Scan(&id)
	require.NoError(t, err)

	return id
}

/*
tenant returns the context scoped to the workspace.
*/
func tenant(ctx context.Context, workspace string) context.Context {
	return tenancy.With(ctx, workspace)
}
