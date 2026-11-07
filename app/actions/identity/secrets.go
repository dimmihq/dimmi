/*
|--------------------------------------------------------------------------
| Identity Secrets
|--------------------------------------------------------------------------
|
| The keys a workspace's host app signs identity tokens with. Every member
| may see which exist, never their material; only those who manage
| identity create, rotate, retire or revoke one. A new shared secret is
| shown in the answer that made it and never again. Rotating makes a new
| secret and lets the old one keep working for a grace period, so a deploy
| can switch keys without a single failed sign-in. Revoking is final and,
| unless asked otherwise, signs out every visitor the secret verified.
|
*/

package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Ended is a visitor session a revoked secret signed out.
*/
type Ended struct {
	SessionID ids.UUID
	ContactID ids.UUID
}

/*
SessionCloser disconnects the streams of sessions that ended.
*/
type SessionCloser interface {
	/*
		CloseSessions tells each session's streams it ended and closes
		them.
	*/
	CloseSessions(ctx context.Context, ended []Ended) error
}

/*
Secrets manages a workspace's identity secrets. MaxActive caps the
secrets not yet revoked, RetireDefault is how long a rotated or retired
secret keeps working when no date is given and RetireMax the latest a
retirement may be set.
*/
type Secrets struct {
	DB            *database.DB
	Clock         clock.Clock
	Keys          *encryption.Keys
	Streams       SessionCloser
	MaxActive     int64
	RetireDefault time.Duration
	RetireMax     time.Duration
}

/*
NewSecret is a secret to create: its name and algorithm, with the PEM
public key for RS256 and EdDSA.
*/
type NewSecret struct {
	Membership authz.Membership
	Name       string
	Algorithm  string
	PublicKey  string
}

/*
Created is a new secret with its shared secret, empty for public keys,
shown only now.
*/
type Created struct {
	Secret models.IdentitySecret
	Value  string
}

/*
Change is what to do with an existing secret: rename it, retire it at
RetiresAt (or the default grace), or revoke it for Reason, signing its
visitors out unless KeepSessions.
*/
type Change struct {
	Membership   authz.Membership
	ID           ids.UUID
	Name         *string
	Status       *string
	RetiresAt    *time.Time
	Reason       string
	KeepSessions bool
}

/*
Failures of managing identity secrets.
*/
var (
	ErrSecretNotFound = fault.New(fault.NotFound, "identity_secret_not_found",
		"No identity secret of this workspace has this id.")
	ErrTooManySecrets = fault.New(fault.Invalid, "identity_secret_limit",
		"A workspace keeps at most 5 identity secrets; revoke one first.")
	ErrInvalidTransition = fault.New(fault.Invalid, "invalid_transition",
		"A revoked secret stays revoked, and a retiring one goes back to active only by rotating.")
	ErrSecretNotActive = fault.New(fault.Conflict, "secret_not_active",
		"Only an active secret can be rotated.")
)

/*
RequireRead checks the caller may see the workspace's secrets.
*/
func RequireRead(m authz.Membership) error {
	if !m.Can("identity:read") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
RequireManage checks the caller may change the workspace's secrets.
*/
func RequireManage(m authz.Membership) error {
	if !m.Can("identity:manage") {
		return exceptions.ErrForbidden
	}
	return nil
}

/*
List returns the workspace's secrets, revoked ones included.
*/
func (s *Secrets) List(ctx context.Context, m authz.Membership) ([]models.IdentitySecret, error) {
	if err := RequireRead(m); err != nil {
		return nil, err
	}
	var out []models.IdentitySecret
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ListIdentitySecrets(ctx, m.Workspace.ID)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("list identity secrets: %w", err)
	}
	return out, nil
}

/*
Show returns one secret.
*/
func (s *Secrets) Show(ctx context.Context, m authz.Membership, id ids.UUID) (models.IdentitySecret, error) {
	if err := RequireRead(m); err != nil {
		return models.IdentitySecret{}, err
	}
	var out models.IdentitySecret
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = find(ctx, models.New(tx), m.Workspace.ID, id)
		return err
	})
	return out, err
}

/*
Create makes a new active secret.
*/
func (s *Secrets) Create(ctx context.Context, in NewSecret) (Created, error) {
	if err := RequireManage(in.Membership); err != nil {
		return Created{}, err
	}
	var out Created
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = s.insert(ctx, tx, in)
		return err
	})
	return out, err
}

/*
Rotate makes a new secret with the same algorithm and name and retires
the old one after the default grace period. A public key secret needs
the new public key.
*/
func (s *Secrets) Rotate(ctx context.Context, m authz.Membership, id ids.UUID, publicKey string) (Created, error) {
	if err := RequireManage(m); err != nil {
		return Created{}, err
	}
	var out Created
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		old, err := lock(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}
		if old.Status != "active" {
			return ErrSecretNotActive
		}
		now := s.Clock.Now()
		if _, err := q.RetireIdentitySecret(ctx, models.RetireIdentitySecretParams{
			RetiresAt: new(now.Add(s.RetireDefault)), Now: now, WorkspaceID: old.WorkspaceID, ID: old.ID,
		}); err != nil {
			return fmt.Errorf("retire identity secret: %w", err)
		}
		out, err = s.insert(ctx, tx, NewSecret{
			Membership: m, Name: old.Name, Algorithm: old.Algorithm, PublicKey: publicKey,
		})
		return err
	})
	return out, err
}

/*
Update renames, retires or revokes a secret.
*/
func (s *Secrets) Update(ctx context.Context, in Change) (models.IdentitySecret, error) {
	if err := RequireManage(in.Membership); err != nil {
		return models.IdentitySecret{}, err
	}
	var out models.IdentitySecret
	var ended []Ended
	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		row, err := lock(ctx, q, in.Membership.Workspace.ID, in.ID)
		if err != nil {
			return err
		}
		if row, err = s.rename(ctx, q, row, in.Name); err != nil {
			return err
		}
		out, ended, err = s.transition(ctx, q, row, in)
		return err
	})
	if err != nil || len(ended) == 0 || s.Streams == nil {
		return out, err
	}
	return out, s.Streams.CloseSessions(ctx, ended)
}

/*
insert checks the limit and stores a new secret, sealing a shared one.
*/
func (s *Secrets) insert(ctx context.Context, tx *database.Tx, in NewSecret) (Created, error) {
	q, ws, now := models.New(tx), in.Membership.Workspace.ID, s.Clock.Now()
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "Default"
	}
	if len([]rune(name)) > 60 {
		return Created{}, fault.New(fault.Invalid, "name_too_long", "Names are at most 60 characters.").
			OnField("name")
	}
	live, err := q.CountLiveIdentitySecrets(ctx, ws)
	if err != nil {
		return Created{}, fmt.Errorf("count identity secrets: %w", err)
	}
	if live >= s.MaxActive {
		return Created{}, ErrTooManySecrets
	}
	params := models.CreateIdentitySecretParams{
		ID: ids.NewV7At(now), WorkspaceID: ws, Name: name, Algorithm: in.Algorithm,
		CreatedByMemberID: memberID(in.Membership), Now: now,
	}
	out := Created{}
	switch in.Algorithm {
	case AlgorithmHS256:
		out.Value = newSharedSecret()
		if params.SecretEncrypted, err = sealSecret(ctx, s.Keys, secretSlot(ws, params.ID), out.Value); err != nil {
			return Created{}, err
		}
		params.Fingerprint = out.Value[len(out.Value)-4:]
	case AlgorithmRS256, AlgorithmEdDSA:
		key, err := parsePublicKey(in.Algorithm, in.PublicKey)
		if err != nil {
			return Created{}, err
		}
		params.PublicKeyJwk, params.Fingerprint = key.stored, key.fingerprint
	default:
		return Created{}, fault.New(fault.Invalid, "algorithm_invalid", "The algorithm is HS256, RS256 or EdDSA.").
			OnField("algorithm")
	}
	if out.Secret, err = q.CreateIdentitySecret(ctx, params); err != nil {
		return Created{}, fmt.Errorf("create identity secret: %w", err)
	}
	return out, nil
}

/*
rename changes the name when one was sent.
*/
func (s *Secrets) rename(ctx context.Context, q *models.Queries, row models.IdentitySecret, name *string,
) (models.IdentitySecret, error) {
	if name == nil {
		return row, nil
	}
	clean := strings.TrimSpace(*name)
	if clean == "" || len([]rune(clean)) > 60 {
		return row, fault.New(fault.Invalid, "name_invalid", "Names are 1 to 60 characters.").OnField("name")
	}
	out, err := q.RenameIdentitySecret(ctx, models.RenameIdentitySecretParams{
		Name: clean, Now: s.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
	if err != nil {
		return row, fmt.Errorf("rename identity secret: %w", err)
	}
	return out, nil
}

/*
transition moves the secret to the status asked for.
*/
func (s *Secrets) transition(ctx context.Context, q *models.Queries, row models.IdentitySecret, in Change,
) (models.IdentitySecret, []Ended, error) {
	if in.Status == nil {
		if in.RetiresAt != nil {
			return row, nil, fault.New(fault.Invalid, "retires_at_without_status",
				"Send status retiring with retires_at.").OnField("retires_at")
		}
		return row, nil, nil
	}
	if row.Status == "revoked" {
		return row, nil, ErrInvalidTransition
	}
	switch *in.Status {
	case "retiring":
		out, err := s.retire(ctx, q, row, in.RetiresAt)
		return out, nil, err
	case "revoked":
		return s.revoke(ctx, q, row, in)
	}
	return row, nil, ErrInvalidTransition
}

/*
retire keeps the secret working until the date, which must lie within
the allowed window.
*/
func (s *Secrets) retire(ctx context.Context, q *models.Queries, row models.IdentitySecret, at *time.Time,
) (models.IdentitySecret, error) {
	now := s.Clock.Now()
	when := now.Add(s.RetireDefault)
	if at != nil {
		when = *at
	}
	if !when.After(now) || when.After(now.Add(s.RetireMax)) {
		return row, fault.New(fault.Invalid, "retires_at_invalid",
			"Pick a retirement date in the next 30 days.").OnField("retires_at")
	}
	out, err := q.RetireIdentitySecret(ctx, models.RetireIdentitySecretParams{
		RetiresAt: &when, Now: now, WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
	if err != nil {
		return row, fmt.Errorf("retire identity secret: %w", err)
	}
	return out, nil
}

/*
revoke stops the secret for good and, unless asked to keep them, ends
every session it verified.
*/
func (s *Secrets) revoke(ctx context.Context, q *models.Queries, row models.IdentitySecret, in Change,
) (models.IdentitySecret, []Ended, error) {
	reason := in.Reason
	if reason == "" {
		reason = "manual"
	}
	if reason != "manual" && reason != "compromised" {
		return row, nil, fault.New(fault.Invalid, "reason_invalid", "The reason is manual or compromised.").
			OnField("reason")
	}
	now := s.Clock.Now()
	out, err := q.RevokeIdentitySecret(ctx, models.RevokeIdentitySecretParams{
		Now: now, Reason: reason, WorkspaceID: row.WorkspaceID, ID: row.ID,
	})
	if err != nil {
		return row, nil, fmt.Errorf("revoke identity secret: %w", err)
	}
	if in.KeepSessions {
		return out, nil, nil
	}
	rows, err := q.RevokeWidgetSessionsForSecret(ctx, models.RevokeWidgetSessionsForSecretParams{
		Now: now, WorkspaceID: row.WorkspaceID, IdentitySecretID: &row.ID,
	})
	if err != nil {
		return out, nil, fmt.Errorf("end sessions of identity secret: %w", err)
	}
	ended := make([]Ended, len(rows))
	for i, r := range rows {
		ended[i] = Ended{SessionID: r.ID, ContactID: r.ContactID}
	}
	return out, ended, nil
}

/*
find loads a secret; an unknown one is not found.
*/
func find(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.IdentitySecret, error) {
	row, err := q.FindIdentitySecret(ctx, models.FindIdentitySecretParams{WorkspaceID: workspaceID, ID: id})
	return row, notFound(err)
}

/*
lock loads a secret and holds it until the transaction ends.
*/
func lock(ctx context.Context, q *models.Queries, workspaceID, id ids.UUID) (models.IdentitySecret, error) {
	row, err := q.LockIdentitySecret(ctx, models.LockIdentitySecretParams{WorkspaceID: workspaceID, ID: id})
	return row, notFound(err)
}

/*
notFound turns a missing row into the secret's not found failure.
*/
func notFound(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return ErrSecretNotFound
	}
	if err != nil {
		return fmt.Errorf("find identity secret: %w", err)
	}
	return nil
}

/*
memberID is the member acting, nil for a caller without one.
*/
func memberID(m authz.Membership) *ids.UUID {
	if m.Member.ID == (ids.UUID{}) {
		return nil
	}
	return &m.Member.ID
}
