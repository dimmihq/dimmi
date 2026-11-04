/*
|--------------------------------------------------------------------------
| Mirror Security Event To Audit
|--------------------------------------------------------------------------
|
| A person's sign-ins and changes to how they sign in live in their own
| security log, which no workspace can read. The admins of every
| workspace they work in still need to see them: a run of failed
| sign-ins or a removed second factor puts the workspace at risk too.
|
| Each such entry is copied, once it is saved, into the audit log of
| each workspace the person is an active member of, at most fifty. The
| copy runs as a queued job, so signing in never waits for it. It holds
| where the attempt came from, the device and the method, never a code
| or a password. A failed attempt at an address without an account has
| no security log entry, so it reaches no audit log.
|
*/

package listeners

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
mirrorMembershipCap is the most workspaces one entry is copied to.
*/
const mirrorMembershipCap = 50

/*
unknownActor is how a failed sign-in names its actor: whoever typed the
password is not known to be the member.
*/
const unknownActor = "Unknown"

/*
cliAgent is the browser a change made at the command line records.
*/
const cliAgent = "cli"

/*
MirrorSecurityEventToAuditArgs is the security log entry to copy, as
SecurityEventRecorded carried it.
*/
type MirrorSecurityEventToAuditArgs struct {
	UserID      ids.UUID       `json:"user_id"`
	Type        string         `json:"type"`
	IP          string         `json:"ip"`
	UserAgent   string         `json:"user_agent"`
	DeviceLabel string         `json:"device_label"`
	Metadata    map[string]any `json:"metadata"`
	At          time.Time      `json:"at"`
}

/*
Kind names the job.
*/
func (MirrorSecurityEventToAuditArgs) Kind() string {
	return "mirror_security_event_to_audit"
}

/*
mirroring is how one kind of security log entry reads in the audit
log: its event name, whether the system rather than the member is its
actor, and the reason an attempt failed.
*/
type mirroring struct {
	event  string
	system bool
	reason string
}

/*
mirrorings lists the security log entries workspace admins see, by
type. Other entries, such as signing out, stay in the person's own log.
*/
func mirrorings() map[string]mirroring {
	return map[string]mirroring{
		appauth.EventLoginFailed:                {audit.EventMemberSignInFailed, true, "invalid_credentials"},
		appauth.EventLoginLocked:                {audit.EventMemberSignInLocked, true, "too_many_attempts"},
		appauth.EventLoginSucceeded:             {event: audit.EventMemberSignedIn},
		appauth.EventTwoFactorEnabled:           {event: audit.EventMemberTwoFactorEnabled},
		appauth.EventTwoFactorDisabled:          {event: audit.EventMemberTwoFactorDisabled},
		appauth.EventAuthFactorAdded:            {event: audit.EventMemberAuthFactorAdded},
		appauth.EventAuthFactorRemoved:          {event: audit.EventMemberAuthFactorRemoved},
		appauth.EventAuthFactorRecoveryCodeUsed: {event: audit.EventMemberRecoveryCodeUsed},
		appauth.EventPasswordChanged:            {event: audit.EventMemberPasswordChanged},
		appauth.EventPasswordReset:              {event: audit.EventMemberPasswordReset},
		appauth.EventEmailChanged:               {event: audit.EventMemberEmailChanged},
		appauth.EventIdentityLinked:             {event: audit.EventMemberIdentityLinked},
		appauth.EventIdentityUnlinked:           {event: audit.EventMemberIdentityUnlinked},
		appauth.EventSessionsRevokedOthers:      {event: audit.EventMemberSessionsRevoked},
	}
}

/*
mirroredDetails maps the details of a security log entry an admin may
see to the name they carry in the audit log. Every other detail, such
as a session or device id, stays behind.
*/
func mirroredDetails() map[string]string {
	return map[string]string{
		"method": "method", "auth_methods": "methods", "provider": "provider", "type": "factor_type",
		"count": "sessions_ended",
	}
}

/*
MirrorSecurityEventToAudit copies security log entries into the audit
logs of the person's workspaces.
*/
type MirrorSecurityEventToAudit struct {
	DB     *database.DB
	Record *audit.RecordAuditLog
}

/*
Listen queues a copy of every entry worth mirroring, in the transaction
that wrote it, so the copy exists only if the entry does.
*/
func (l *MirrorSecurityEventToAudit) Listen(d *events.Dispatcher) {
	events.ListenQueued(d, func(e appevents.SecurityEventRecorded) queue.Args {
		if _, ok := mirrorings()[e.Type]; !ok {
			return nil
		}

		return MirrorSecurityEventToAuditArgs(e)
	})
}

/*
Handle writes one audit entry in each workspace the person is an
active member of, all in one transaction, so a retry never writes one
twice.
*/
func (l *MirrorSecurityEventToAudit) Handle(ctx context.Context, job *queue.Job[MirrorSecurityEventToAuditArgs]) error {
	e := job.Args

	memberships, err := models.New(l.DB).ListUserActiveMemberships(ctx, e.UserID)
	if err != nil {
		return fmt.Errorf("list memberships to mirror security event: %w", err)
	}

	memberships = memberships[:min(len(memberships), mirrorMembershipCap)]

	return l.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		for _, m := range memberships {
			if err := l.mirror(ctx, tx, e, m); err != nil {
				return err
			}
		}

		return nil
	})
}

/*
mirror writes the entry into one workspace's audit log, about the
member and, unless the system acted, by them. A membership that began
after the entry was written did not exist when it happened, so it gets
no copy.
*/
func (l *MirrorSecurityEventToAudit) mirror(
	ctx context.Context, tx *database.Tx, e MirrorSecurityEventToAuditArgs, m models.ListUserActiveMembershipsRow,
) error {
	if err := tx.SetTenant(ctx, m.WorkspaceID.String()); err != nil {
		return err
	}

	found, err := models.New(tx).FindWorkspaceMemberWithUser(ctx, models.FindWorkspaceMemberWithUserParams{
		WorkspaceID: m.WorkspaceID, ID: m.MemberID,
	})
	if err != nil {
		return fmt.Errorf("find mirrored member: %w", err)
	}

	if found.WorkspaceMember.CreatedAt.After(e.At) {
		return nil
	}

	entry := mirroredEntry(e)
	entry.WorkspaceID = m.WorkspaceID
	entry.Subject = audit.Subject{Type: subjectMember, ID: m.MemberID, Label: found.User.Name}

	if entry.SystemLabel == "" && entry.Via == "" {
		entry.ActorID = m.MemberID
	}

	return l.record(ctx, entry)
}

/*
record writes the entry.
*/
func (l *MirrorSecurityEventToAudit) record(ctx context.Context, entry audit.Entry) error {
	_, err := l.Record.Handle(ctx, entry)

	return err
}

/*
mirroredEntry is the audit entry a security log entry becomes, before
it is placed in a workspace. A failed attempt names an unknown actor,
a block the system, and a change made at the command line the system
acting there.
*/
func mirroredEntry(e MirrorSecurityEventToAuditArgs) audit.Entry {
	how := mirrorings()[e.Type]
	entry := audit.Entry{
		Event: how.event, Changes: mirroredChanges(e, how), At: e.At,
		Client: &appauth.Client{IP: e.IP, UserAgent: e.UserAgent},
	}

	switch {
	case e.Type == appauth.EventLoginFailed:
		entry.SystemLabel = unknownActor
	case e.UserAgent == cliAgent && e.IP == "":
		entry.Via = audit.ViaCLI
	case how.system:
		entry.SystemLabel = "System"
	}

	return entry
}

/*
mirroredChanges holds where the entry came from, the device, why an
attempt failed and the details an admin may see.
*/
func mirroredChanges(e MirrorSecurityEventToAuditArgs, how mirroring) map[string]any {
	changes := map[string]any{}

	for key, value := range map[string]string{"ip": e.IP, "device_label": e.DeviceLabel, "reason": how.reason} {
		if value != "" {
			changes[key] = value
		}
	}

	for from, to := range mirroredDetails() {
		if value, ok := e.Metadata[from]; ok {
			changes[to] = value
		}
	}

	return changes
}
