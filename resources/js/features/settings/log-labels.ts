/*
 * How the audit log and the security log read to people: each event a
 * short phrase, each actor's channel the way they reached dimmi, and
 * times in the reader's own clock. An event the browser app does not
 * know yet shows its name as the server sent it.
 */
import type { AuditEntry } from "@/lib/api/audit";

export const auditEvents: Record<string, string> = {
  "workspace.created": "Created the workspace",
  "workspace.updated": "Changed workspace settings",
  "member.joined": "Joined the workspace",
  "member.role_changed": "Changed a role",
  "ownership.transferred": "Handed over ownership",
  "member.deactivated": "Deactivated a member",
  "member.reactivated": "Reactivated a member",
  "member.left": "Left the workspace",
  "workspace.deleted": "Deleted the workspace",
  "workspace.restored": "Restored the workspace",
  "token.created": "Created an API token",
  "token.revoked": "Revoked an API token",
  "invitation.created": "Invited someone",
  "invitation.accepted": "Accepted an invitation",
  "invitation.revoked": "Withdrew an invitation",
  "team.created": "Created a team",
  "team.updated": "Renamed a team",
  "team.deleted": "Deleted a team",
  "team.member_added": "Added someone to a team",
  "team.member_removed": "Removed someone from a team",
  "redaction_rule.created": "Added a redaction rule",
  "redaction_rule.updated": "Changed a redaction rule",
  "redaction_rule.deleted": "Deleted a redaction rule",
  "redaction_settings.updated": "Changed the built-in redaction rules",
};

const securityEvents: Record<string, string> = {
  "login.succeeded": "Signed in",
  "login.failed": "Failed sign-in",
  "login.locked": "Sign-in blocked after failed attempts",
  "logout.succeeded": "Signed out",
  "password.changed": "Changed the password",
  "password.reset": "Reset the password",
  "two_factor.enabled": "Turned on two-factor authentication",
  "two_factor.disabled": "Turned off two-factor authentication",
  "auth_factor.added": "Added an authenticator",
  "auth_factor.removed": "Removed an authenticator",
  "auth_factor.renamed": "Renamed an authenticator",
  "auth_factor.recovery_code_used": "Used a recovery code",
  "auth_factor.recovery_codes_regenerated": "Made new recovery codes",
  "email.change_requested": "Asked to change the email",
  "email.changed": "Changed the email",
  "identity.linked": "Linked a sign-in provider",
  "identity.unlinked": "Unlinked a sign-in provider",
  "session.revoked": "Signed out a session",
  "sessions.revoked_others": "Signed out every other session",
  "magic_link.used": "Signed in with an email link",
  "token.created": "Created an API token",
};

const channels: Record<string, string> = {
  session: "Browser",
  app: "App",
  personal_token: "API token",
  oauth_client: "Connected app",
  cli: "Command line",
};

export function auditEventLabel(event: string): string {
  return auditEvents[event] ?? event;
}

export function securityEventLabel(type: string): string {
  return securityEvents[type] ?? type;
}

/*
 * actorName is who an entry names, without the email address the
 * server keeps beside a member's name; a token's name stays.
 */
export function actorName(label: string): string {
  return label.replace(/\s<[^>]*>$/, "");
}

/*
 * actorDetail says how the actor reached dimmi and from which address.
 * The system acts on its own; a member signing up has no channel yet.
 */
export function actorDetail(entry: Pick<AuditEntry, "actor" | "ip">): string {
  const kind = entry.actor.via?.kind;
  const channel = kind ? (channels[kind] ?? kind) : entry.actor.type === "system" ? "Automatic" : "";

  return [channel, entry.ip].filter(Boolean).join(" · ");
}

/*
 * formatTime writes a moment as a date and a time in the reader's
 * locale and time zone.
 */
export function formatTime(iso: string, locale?: string): string {
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(iso));
}
