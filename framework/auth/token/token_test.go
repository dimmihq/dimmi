package token_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/auth/token"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/testkit"
)

const prefix = "app_test_"

/*
openDB returns a small pool on a fresh database migrated with the
tokens table.
*/
func openDB(t *testing.T) *database.DB {
	t.Helper()

	url := testkit.NewDatabase(t).URL
	_, err := migrate.Runner{URL: url, Framework: token.Migrations()}.Up(t.Context())
	require.NoError(t, err)

	db, err := database.Open(t.Context(), database.Config{URL: url, MaxConns: 2, PoolMode: database.PoolModeSession})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return db
}

/*
newTokens returns tokens on the database with a frozen clock.
*/
func newTokens(t *testing.T, db *database.DB) (*token.Tokens, *token.PostgresStore, *clock.Fake) {
	t.Helper()

	clk := clock.NewFake(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC))
	store := token.NewPostgresStore(db, "")

	tokens, err := token.New(store, clk, token.Options{Prefix: prefix})
	require.NoError(t, err)

	return tokens, store, clk
}

func create(t *testing.T, tokens *token.Tokens, tok token.Token) (token.Token, string) {
	t.Helper()

	created, plain, err := tokens.Create(t.Context(), tok)
	require.NoError(t, err)

	return created, plain
}

func TestMigrations(t *testing.T) {
	t.Parallel()

	t.Run("pass the migration lint", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, migrate.Lint(token.Migrations()))
	})
}

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("refuses a prefix people could not recognize", func(t *testing.T) {
		t.Parallel()

		for _, p := range []string{"", "app-live-", "app live"} {
			_, err := token.New(nil, clock.System(), token.Options{Prefix: p})

			require.ErrorIs(t, err, token.ErrInvalidPrefix, p)
		}
	})
}

func TestCreate(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("issues a prefixed token and stores only its digest", func(t *testing.T) {
		t.Parallel()
		tokens, store, clk := newTokens(t, db)
		owner := ids.New()

		created, plain := create(t, tokens, token.Token{OwnerID: owner, Name: "CLI", TenantID: "tenant-1"})

		assert.True(t, strings.HasPrefix(plain, prefix))
		assert.Len(t, plain, len(prefix)+38)
		assert.Equal(t, plain[:len(prefix)+4], created.Hint)
		assert.Equal(t, clk.Now(), created.CreatedAt)
		stored, err := store.FindByHash(t.Context(), hashing.SHA256([]byte(plain)))
		require.NoError(t, err)
		assert.Equal(t, created.ID, stored.ID)
		assert.Equal(t, "tenant-1", stored.TenantID)
		assert.Equal(t, []string{}, stored.Abilities)
		assert.True(t, stored.ExpiresAt.IsZero())
	})

	t.Run("never issues the same token twice", func(t *testing.T) {
		t.Parallel()
		tokens, _, _ := newTokens(t, db)

		_, first := create(t, tokens, token.Token{OwnerID: ids.New(), Name: "one"})
		_, second := create(t, tokens, token.Token{OwnerID: ids.New(), Name: "two"})

		assert.NotEqual(t, first, second)
	})
}

func TestFind(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("returns the live token", func(t *testing.T) {
		t.Parallel()
		tokens, _, clk := newTokens(t, db)
		expires := clk.Now().Add(time.Hour)
		created, plain := create(t, tokens, token.Token{
			OwnerID: ids.New(), Name: "CI", Abilities: []string{"records:read"}, ExpiresAt: expires,
		})

		found, err := tokens.Find(t.Context(), plain)

		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		assert.Equal(t, []string{"records:read"}, found.Abilities)
		assert.Equal(t, expires, found.ExpiresAt.UTC())
	})

	t.Run("turns away malformed tokens without asking the database", func(t *testing.T) {
		t.Parallel()
		closed := openDB(t)
		tokens, _, _ := newTokens(t, closed)
		_, plain := create(t, tokens, token.Token{OwnerID: ids.New(), Name: "CI"})
		closed.Close()

		typo := plain[:len(plain)-1] + string(plain[len(plain)-1]^1)
		malformed := []string{typo, "other_" + plain[len(prefix):], plain + "x", prefix + strings.Repeat("-", 38)}
		for _, bad := range malformed {
			_, err := tokens.Find(t.Context(), bad)

			require.ErrorIs(t, err, auth.ErrInvalidToken, bad)
		}
	})

	t.Run("does not know tokens issued elsewhere", func(t *testing.T) {
		t.Parallel()
		elsewhere, _, _ := newTokens(t, openDB(t))
		_, plain := create(t, elsewhere, token.Token{OwnerID: ids.New(), Name: "CI"})
		tokens, _, _ := newTokens(t, db)

		_, err := tokens.Find(t.Context(), plain)

		require.ErrorIs(t, err, auth.ErrInvalidToken)
	})

	t.Run("refuses an expired token", func(t *testing.T) {
		t.Parallel()
		tokens, _, clk := newTokens(t, db)
		_, plain := create(t, tokens, token.Token{OwnerID: ids.New(), Name: "CI", ExpiresAt: clk.Now().Add(time.Hour)})

		clk.Travel(time.Hour)
		_, err := tokens.Find(t.Context(), plain)

		require.ErrorIs(t, err, token.ErrTokenExpired)
	})

	t.Run("refuses a revoked token", func(t *testing.T) {
		t.Parallel()
		tokens, _, _ := newTokens(t, db)
		created, plain := create(t, tokens, token.Token{OwnerID: ids.New(), Name: "CI"})

		require.NoError(t, tokens.Revoke(t.Context(), created.ID))
		_, err := tokens.Find(t.Context(), plain)

		require.ErrorIs(t, err, token.ErrTokenRevoked)
		require.ErrorIs(t, tokens.Revoke(t.Context(), created.ID), token.ErrNotFound)
	})
}

func TestUsage(t *testing.T) {
	t.Parallel()

	db := openDB(t)

	t.Run("is written at most once per interval", func(t *testing.T) {
		t.Parallel()
		tokens, _, clk := newTokens(t, db)
		owner := ids.New()
		_, plain := create(t, tokens, token.Token{OwnerID: owner, Name: "CI"})
		firstUse := clk.Now()

		use(t, tokens, plain, "192.0.2.10")
		clk.Travel(30 * time.Second)
		use(t, tokens, plain, "192.0.2.20")
		assert.Equal(t, usage{firstUse, "192.0.2.10"}, lastUse(t, tokens, owner))

		clk.Travel(30 * time.Second)
		use(t, tokens, plain, "192.0.2.30")
		assert.Equal(t, usage{clk.Now(), "192.0.2.30"}, lastUse(t, tokens, owner))
	})

	t.Run("lists the owner's tokens newest first, revoked ones too", func(t *testing.T) {
		t.Parallel()
		tokens, _, clk := newTokens(t, db)
		owner := ids.New()
		older, _ := create(t, tokens, token.Token{OwnerID: owner, Name: "old"})
		clk.Travel(time.Second)
		newer, _ := create(t, tokens, token.Token{OwnerID: owner, Name: "new"})
		create(t, tokens, token.Token{OwnerID: ids.New(), Name: "someone else's"})
		require.NoError(t, tokens.Revoke(t.Context(), older.ID))

		listed, err := tokens.ForOwner(t.Context(), owner)

		require.NoError(t, err)
		require.Len(t, listed, 2)
		assert.Equal(t, newer.ID, listed[0].ID)
		assert.Equal(t, clk.Now(), listed[1].RevokedAt.UTC())
	})
}

func use(t *testing.T, tokens *token.Tokens, plain, ip string) {
	t.Helper()

	found, err := tokens.Find(t.Context(), plain)
	require.NoError(t, err)
	require.NoError(t, tokens.Use(t.Context(), found, ip))
}

/*
usage is when and from where a token was last used.
*/
type usage struct {
	at time.Time
	ip string
}

/*
lastUse returns the recorded last use of the owner's only token.
*/
func lastUse(t *testing.T, tokens *token.Tokens, owner ids.UUID) usage {
	t.Helper()

	listed, err := tokens.ForOwner(t.Context(), owner)
	require.NoError(t, err)
	require.Len(t, listed, 1)

	return usage{at: listed[0].LastUsedAt.UTC(), ip: listed[0].LastUsedIP}
}

func TestResolver(t *testing.T) {
	t.Parallel()

	db := openDB(t)
	tokens, _, clk := newTokens(t, db)
	h := guardedRoutes(t, tokens)
	owner := ids.New()
	_, reader := create(t, tokens, token.Token{
		OwnerID: owner, TenantID: "tenant-1", Name: "reader", Abilities: []string{"records:read"},
		ExpiresAt: clk.Now().Add(time.Hour),
	})

	t.Run("authenticates the owner with the token's tenant", func(t *testing.T) {
		t.Parallel()
		rec := call(t, h, http.MethodGet, reader)

		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"id":"`+owner.String()+`","guard":"token","tenant":"tenant-1"}`, rec.Body.String())
	})

	t.Run("holds the token to its abilities", func(t *testing.T) {
		t.Parallel()
		rec := call(t, h, http.MethodPost, reader)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Equal(t, "missing_ability", code(t, rec))
	})

	t.Run("leaves tokens of another kind to other resolvers", func(t *testing.T) {
		t.Parallel()
		rec := call(t, h, http.MethodGet, "other_"+reader[len(prefix):])

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "invalid_token", code(t, rec))
	})

	t.Run("records the address the token was used from", func(t *testing.T) {
		t.Parallel()
		call(t, h, http.MethodGet, reader)

		listed, err := tokens.ForOwner(t.Context(), owner)
		require.NoError(t, err)
		assert.Equal(t, "192.0.2.1", listed[0].LastUsedIP)
	})
}

func TestStoreFailures(t *testing.T) {
	t.Parallel()

	db := openDB(t)
	tokens, store, clk := newTokens(t, db)
	created, plain := create(t, tokens, token.Token{OwnerID: ids.New(), Name: "CI"})
	h := guardedRoutes(t, tokens)
	db.Close()

	t.Run("answer 500 rather than a verdict", func(t *testing.T) {
		t.Parallel()
		rec := call(t, h, http.MethodGet, plain)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("surface every store error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		_, _, err := tokens.Create(ctx, token.Token{OwnerID: ids.New(), Name: "CI"})
		require.Error(t, err)
		_, err = tokens.Find(ctx, plain)
		require.Error(t, err)
		require.Error(t, tokens.Use(ctx, created, "192.0.2.1"))
		require.Error(t, tokens.Revoke(ctx, created.ID))
		_, err = tokens.ForOwner(ctx, created.OwnerID)
		require.Error(t, err)
		require.Error(t, store.Touch(ctx, created.ID, clk.Now(), ""))
	})

	t.Run("fail the request when usage cannot be written", func(t *testing.T) {
		t.Parallel()
		live := openDB(t)
		liveTokens, _, _ := newTokens(t, live)
		_, livePlain := create(t, liveTokens, token.Token{OwnerID: ids.New(), Name: "CI"})
		_, err := live.Exec(t.Context(), "REVOKE UPDATE ON personal_access_tokens FROM CURRENT_USER")
		require.NoError(t, err)

		rec := call(t, guardedRoutes(t, liveTokens), http.MethodGet, livePlain)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

/*
guardedRoutes returns a router with a read and a write route behind the
bearer guard that resolves the tokens.
*/
func guardedRoutes(t *testing.T, tokens *token.Tokens) http.Handler {
	t.Helper()

	guards := auth.NewGuards(http.Errors{})
	guards.Register("token", auth.BearerGuard(tokens.Resolver("token")))

	kernel := http.NewKernel()
	kernel.Alias("auth", guards.Middleware)

	r := routing.New(routing.Options{Middleware: kernel})
	r.Handle(http.MethodGet, "/records", "records.index", whoami()).Middleware("auth").Ability("records:read")
	r.Handle(http.MethodPost, "/records", "records.store", whoami()).Middleware("auth").Ability("records:write")

	h, err := r.Handler()
	require.NoError(t, err)

	return h
}

func whoami() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, _ := auth.From(r.Context())
		http.WriteJSON(w, http.StatusOK, map[string]string{
			"id": p.ID().String(), "guard": p.Guard(), "tenant": p.Tenant(),
		})
	})
}

func call(t *testing.T, h http.Handler, method, plain string) *httptest.ResponseRecorder {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), method, "/records", strings.NewReader(""))
	r.Header.Set("Authorization", "Bearer "+plain)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	return rec
}

func code(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	var out struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	return out.Code
}
