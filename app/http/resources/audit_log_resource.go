/*
|--------------------------------------------------------------------------
| Audit Log Resource
|--------------------------------------------------------------------------
|
| One entry of a workspace's audit log, as admins and their programs
| read it. Every id is the public id of what it names; the labels are
| the names things had when the entry was written.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
AuditLog is one entry of the audit log.
*/
type AuditLog struct {
	ID        string          `json:"id"`
	Object    string          `json:"object"`
	Event     string          `json:"event"`
	Actor     AuditActor      `json:"actor"`
	Subject   *AuditSubject   `json:"subject"`
	Changes   json.RawMessage `json:"changes"`
	IP        *string         `json:"ip"`
	UserAgent *string         `json:"user_agent"`
	RequestID *string         `json:"request_id"`
	CreatedAt time.Time       `json:"created_at"`
}

/*
AuditActor is who made the change and the credential they used, null
for work the system did on its own.
*/
type AuditActor struct {
	Type  string    `json:"type"`
	ID    *string   `json:"id"`
	Via   *AuditVia `json:"via"`
	Label string    `json:"label"`
}

/*
AuditVia is the kind of credential an actor used and its id.
*/
type AuditVia struct {
	Kind string  `json:"kind"`
	ID   *string `json:"id"`
}

/*
AuditSubject is what the change was about.
*/
type AuditSubject struct {
	Type  string  `json:"type"`
	ID    *string `json:"id"`
	Label *string `json:"label"`
}

/*
auditPrefixes maps the kinds of actor, credential and subject to the
prefix of their public ids.
*/
func auditPrefixes() map[string]models.Prefix {
	return map[string]models.Prefix{
		"member": models.PrefixMember, "workspace": models.PrefixWorkspace, "team": models.PrefixTeam,
		"invitation": models.PrefixInvitation, "token": models.PrefixPersonalToken,
		"session": models.PrefixSession, "app": models.PrefixOAuthGrant, "personal_token": models.PrefixPersonalToken,
		"oauth_client": models.PrefixOAuthClient,
	}
}

/*
NewAuditLog shapes one entry.
*/
func NewAuditLog(e models.AuditLog) AuditLog {
	out := AuditLog{
		ID:        models.PublicID(models.PrefixAuditLog, e.ID),
		Object:    "audit_log",
		Event:     e.Event,
		Actor:     AuditActor{Type: e.ActorType, ID: auditID(e.ActorType, e.ActorID), Label: e.ActorLabel},
		Changes:   e.Changes,
		UserAgent: e.UserAgent,
		RequestID: e.RequestID,
		CreatedAt: e.CreatedAt.UTC(),
	}

	if e.ViaKind != nil {
		out.Actor.Via = &AuditVia{Kind: *e.ViaKind, ID: auditID(*e.ViaKind, e.ViaID)}
	}

	if e.SubjectType != nil {
		out.Subject = &AuditSubject{
			Type: *e.SubjectType, ID: auditID(*e.SubjectType, e.SubjectID), Label: e.SubjectLabel,
		}
	}

	if e.Ip != nil {
		ip := e.Ip.String()
		out.IP = &ip
	}

	return out
}

/*
NewAuditLogs shapes a page of entries, in the order given.
*/
func NewAuditLogs(entries []models.AuditLog) []AuditLog {
	out := make([]AuditLog, 0, len(entries))
	for _, e := range entries {
		out = append(out, NewAuditLog(e))
	}

	return out
}

/*
auditID returns the public id of the thing of the kind, its bare id for
a kind without a prefix, or nil when there is none.
*/
func auditID(kind string, id *ids.UUID) *string {
	if id == nil {
		return nil
	}

	public := id.String()
	if prefix, ok := auditPrefixes()[kind]; ok {
		public = models.PublicID(prefix, *id)
	}

	return &public
}
