/*
|--------------------------------------------------------------------------
| Record Audit Log
|--------------------------------------------------------------------------
|
| Every change to a workspace someone may later have to answer for is
| written to its audit log inside the transaction that made it, so an
| entry exists exactly when the change does and a failed write undoes
| the change. The entry names who acted, through which credential, from
| which address, labeled as they were at that moment, so it still reads
| right after the member or the token is gone. Values under keys that
| look secret never reach the log.
|
| This is the one writer of the audit log: listeners and actions of
| every feature record through it.
|
*/

package audit

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The audit events of workspaces and their members, of the ways in, such
as access tokens and invitations, of teams and of custom roles.
*/
const (
	EventWorkspaceCreated     = "workspace.created"
	EventWorkspaceUpdated     = "workspace.updated"
	EventAccessPolicyChanged  = "workspace.access_policy_changed"
	EventMemberJoined         = "member.joined"
	EventMemberRoleChanged    = "member.role_changed"
	EventMemberDeactivated    = "member.deactivated"
	EventMemberReactivated    = "member.reactivated"
	EventMemberAccessChanged  = "member.access_changed"
	EventMemberAvailability   = "member.availability_changed"
	EventMemberLeft           = "member.left"
	EventOwnershipTransferred = "ownership.transferred"
	EventWorkspaceDeleted     = "workspace.deleted"
	EventWorkspaceRestored    = "workspace.restored"
	EventTokenCreated         = "token.created"
	EventTokenRevoked         = "token.revoked"
	EventInvitationCreated    = "invitation.created"
	EventInvitationAccepted   = "invitation.accepted"
	EventInvitationRevoked    = "invitation.revoked"
	EventTeamCreated          = "team.created"
	EventTeamUpdated          = "team.updated"
	EventTeamDeleted          = "team.deleted"
	EventTeamMemberAdded      = "team.member_added"
	EventTeamMemberRemoved    = "team.member_removed"
	EventRoleCreated          = "role.created"
	EventRoleUpdated          = "role.updated"
	EventRoleDeleted          = "role.deleted"
	EventMemberCustomRole     = "member.custom_role_changed"

	EventWidgetSettingsUpdated = "widget.settings_updated"
	EventWidgetDeleted         = "widget.deleted"
	EventTwoFactorResetAsked   = "member.two_factor_reset_requested"
	EventTwoFactorResetDone    = "member.two_factor_reset_completed"
	EventSignInLockCleared     = "member.sign_in_lock_cleared"

	EventRedactionRuleCreated     = "redaction_rule.created"
	EventRedactionRuleUpdated     = "redaction_rule.updated"
	EventRedactionRuleDeleted     = "redaction_rule.deleted"
	EventRedactionSettingsUpdated = "redaction_settings.updated"

	EventContactErasureRequested = "contact_erasure.requested"
	EventContactErasureCanceled  = "contact_erasure.canceled"
	EventContactErasureHeld      = "contact_erasure.held"
	EventContactErasureCompleted = "contact_erasure.completed"
)

/*
EventSSOConnection and EventOrganizationDomain prefix the audit events of
a company's connections and domains, followed by what happened to them,
such as sso_connection.enforced or organization_domain.verified.
*/
const (
	EventSSOConnection      = "sso_connection."
	EventOrganizationDomain = "organization_domain."
)

/*
The kinds of actor and of the channel they acted through. A member acts
through their browser session, their own app or an access token they
minted; the system acts on its own, or for an operator at the command
line.
*/
const (
	ActorMember      = "member"
	ActorSystem      = "system"
	ViaSession       = "session"
	ViaApp           = "app"
	ViaPersonalToken = "personal_token"
	ViaCLI           = "cli"
)

/*
systemLabel is how an entry names the system as its actor.
*/
const systemLabel = "System"

/*
Subject is what an entry is about: its kind, such as member or
workspace, its id, and the name it had at the time.
*/
type Subject struct {
	Type  string
	ID    ids.UUID
	Label string
}

/*
Entry is one change to record. ActorID is the member who made it, or
ids.Nil() when the system did. Changes describes what changed, such as
{"role": {"from": "member", "to": "admin"}}, and holds only values that
encode as JSON.

Work recorded after the fact, away from the request that caused it,
says what the request would have: Client is where it came from, At
when it happened, SystemLabel how the system is named as its actor
and Via the channel the system acted through, such as cli. Left empty,
they come from the request in the context and the clock.
*/
type Entry struct {
	WorkspaceID ids.UUID
	Event       string
	ActorID     ids.UUID
	Subject     Subject
	Changes     map[string]any
	Client      *appauth.Client
	At          time.Time
	SystemLabel string
	Via         string
}

/*
RecordAuditLog writes entries to workspaces' audit logs. The credential
and the client come from the request in the context, when there is one;
work done outside a request records no channel and no address.

Errors: a wrapped database error, which fails the change it records.
*/
type RecordAuditLog struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
actor is who an entry names as having acted, and through what.
*/
type actor struct {
	kind    string
	id      *ids.UUID
	viaKind *string
	viaID   *ids.UUID
	label   string
}

/*
Handle writes the entry, inside the transaction in the context when
there is one, and returns it as stored. It runs scoped to the entry's
workspace, so it also works for callers that hold no transaction.
*/
func (a *RecordAuditLog) Handle(ctx context.Context, e Entry) (models.AuditLog, error) {
	var entry models.AuditLog

	scoped := tenancy.With(ctx, e.WorkspaceID.String())

	err := a.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		var err error
		entry, err = a.write(ctx, models.New(tx), e)

		return err
	})

	return entry, err
}

/*
write stores the entry with the queries of the workspace's transaction.
*/
func (a *RecordAuditLog) write(ctx context.Context, q *models.Queries, e Entry) (models.AuditLog, error) {
	who, err := a.actor(ctx, q, e)
	if err != nil {
		return models.AuditLog{}, err
	}

	changes, err := json.Marshal(Redact(e.Changes))
	if err != nil {
		return models.AuditLog{}, fmt.Errorf("encode audit changes: %w", err)
	}

	params := models.CreateAuditLogParams{
		WorkspaceID: e.WorkspaceID, Event: e.Event, ActorType: who.kind, ActorID: who.id, ViaKind: who.viaKind,
		ViaID: who.viaID, ActorLabel: who.label, Changes: changes, Now: e.At,
	}
	if e.At.IsZero() {
		params.Now = a.Clock.Now()
	}

	withSubject(&params, e.Subject)
	withClient(ctx, &params, e.Client)

	entry, err := q.CreateAuditLog(ctx, params)
	if err != nil {
		return entry, fmt.Errorf("record audit event %s: %w", e.Event, err)
	}

	return entry, nil
}

/*
actor works out who the entry names: the system, or the member with
their account as it is now and, for a token, the token's name.
*/
func (a *RecordAuditLog) actor(ctx context.Context, q *models.Queries, e Entry) (actor, error) {
	who := actor{kind: ActorSystem, label: cmp.Or(e.SystemLabel, systemLabel)}
	principal, signedIn := appauth.CurrentPrincipal(ctx)

	if signedIn {
		who.viaKind, who.viaID = via(principal)
	}

	if e.Via != "" {
		who.viaKind = &e.Via
	}

	if e.ActorID == ids.Nil() {
		return who, nil
	}

	member, err := q.FindAuditActor(ctx, models.FindAuditActorParams{WorkspaceID: e.WorkspaceID, ID: e.ActorID})
	if err != nil {
		return who, fmt.Errorf("find audit actor: %w", err)
	}

	who.kind, who.id = ActorMember, &member.ID
	who.label = fmt.Sprintf("%s <%s>", member.Name, member.Email)

	if !signedIn || principal.Kind != appauth.KindToken {
		return who, nil
	}

	token, err := q.FindAuditToken(ctx, principal.CredentialID)
	if errors.Is(err, database.ErrNotFound) {
		return who, nil
	}

	if err != nil {
		return who, fmt.Errorf("find audit token: %w", err)
	}

	who.label = fmt.Sprintf("%s via Token %s (%s)", member.Name, token.Name, token.Hint)

	return who, nil
}

/*
via returns the channel the caller acts through and the credential's
id.
*/
func via(p *appauth.Principal) (*string, *ids.UUID) {
	kinds := map[string]string{
		appauth.KindSession: ViaSession, appauth.KindApp: ViaApp, appauth.KindToken: ViaPersonalToken,
	}

	kind, known := kinds[p.Kind]
	if !known {
		return nil, nil
	}

	id := p.CredentialID

	return &kind, &id
}

/*
withSubject fills in what the entry is about, when it names anything.
*/
func withSubject(p *models.CreateAuditLogParams, s Subject) {
	if s.Type == "" {
		return
	}

	p.SubjectType, p.SubjectLabel = &s.Type, &s.Label
	if s.ID != ids.Nil() {
		p.SubjectID = &s.ID
	}
}

/*
withClient fills in the address, browser and request id of the request
in the context, when there is one, the client given taking its place.
*/
func withClient(ctx context.Context, p *models.CreateAuditLogParams, given *appauth.Client) {
	if id := appauth.CurrentRequestID(ctx); id != "" {
		p.RequestID = &id
	}

	client, ok := appauth.CurrentClient(ctx)
	if given != nil {
		client, ok = *given, true
	}

	if !ok {
		return
	}

	p.Ip = appauth.Address(client.IP)

	if agent := client.TruncatedUserAgent(); agent != "" {
		p.UserAgent = &agent
	}
}
