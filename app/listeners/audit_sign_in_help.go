/*
|--------------------------------------------------------------------------
| Audit Log: Sign-In Help
|--------------------------------------------------------------------------
|
| Helping a teammate back in weakens their account for a while, so the
| workspace keeps a record of each step: the admin who mailed a reset
| link, the teammate who used it, and the admin who lifted a sign-in
| lock. The entry is about the teammate either way.
|
*/

package listeners

import (
	"context"

	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/actions/audit"
	appevents "github.com/dimmihq/dimmi/app/events"
)

/*
listenSignInHelp registers the entries of an admin helping a teammate
sign in.
*/
func (l *WriteAuditLog) listenSignInHelp(d *events.Dispatcher) {
	events.Listen(d, events.InTx, l.twoFactorResetRequested)
	events.Listen(d, events.InTx, l.twoFactorResetCompleted)
	events.Listen(d, events.InTx, l.signInLockCleared)
}

/*
twoFactorResetRequested records an admin mailing a teammate the link
that turns their second factor off.
*/
func (l *WriteAuditLog) twoFactorResetRequested(ctx context.Context, e appevents.MemberTwoFactorResetRequested) error {
	return l.member(ctx, e.WorkspaceID, e.MemberID, audit.Entry{
		Event: audit.EventTwoFactorResetAsked, ActorID: e.RequestedBy, Changes: map[string]any{},
	})
}

/*
twoFactorResetCompleted records the teammate turning their second
factor off with the link, as their own act.
*/
func (l *WriteAuditLog) twoFactorResetCompleted(ctx context.Context, e appevents.MemberTwoFactorResetCompleted) error {
	return l.member(ctx, e.WorkspaceID, e.MemberID, audit.Entry{
		Event: audit.EventTwoFactorResetDone, ActorID: e.MemberID,
		Changes: map[string]any{"two_factor": map[string]any{"from": true, "to": false}},
	})
}

/*
signInLockCleared records an admin lifting a teammate's sign-in lock.
*/
func (l *WriteAuditLog) signInLockCleared(ctx context.Context, e appevents.MemberSignInLockCleared) error {
	return l.member(ctx, e.WorkspaceID, e.MemberID, audit.Entry{
		Event: audit.EventSignInLockCleared, ActorID: e.ClearedBy, Changes: map[string]any{},
	})
}
