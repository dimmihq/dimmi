/*
 * How the audit log and the security log read to people: each event a
 * short phrase, each actor's channel the way they reached dimmi, and
 * times in the reader's own clock. An event the browser app does not
 * know yet shows its name as the server sent it.
 */
import type { AuditEntry } from "@/lib/api/audit";
import { i18n } from "@/lib/i18n";
import { formatDateTime } from "@/lib/i18n/format";

/*
 * AuditEventGroup is one kind of event in the audit log's filter, such
 * as sign-ins, with the events in it, named in the reader's language. A
 * group no feature writes to yet has no events and is left out of the
 * filter.
 */
export interface AuditEventGroup {
  id: string;
  name: string;
  events: Record<string, string>;
}

/*
 * groups lists the audit events the browser app names, by kind, in the
 * order the filter offers them.
 */
const groups: { id: string; events: string[] }[] = [
  {
    id: "sign_in",
    events: ["member.signed_in", "member.sign_in_failed", "member.sign_in_locked", "member.sessions_revoked"],
  },
  {
    id: "security",
    events: [
      "member.two_factor_enabled",
      "member.two_factor_disabled",
      "member.auth_factor_added",
      "member.auth_factor_removed",
      "member.recovery_code_used",
      "member.password_changed",
      "member.password_reset",
      "member.email_changed",
      "member.identity_linked",
      "member.identity_unlinked",
      "member.two_factor_reset_requested",
      "member.two_factor_reset_completed",
      "member.sign_in_lock_cleared",
      "workspace.access_policy_changed",
    ],
  },
  {
    id: "members",
    events: [
      "workspace.created",
      "workspace.updated",
      "workspace.deleted",
      "workspace.restored",
      "workspace.suspended",
      "workspace.unsuspended",
      "member.joined",
      "member.role_changed",
      "ownership.transferred",
      "member.deactivated",
      "member.reactivated",
      "member.left",
      "invitation.created",
      "invitation.accepted",
      "invitation.revoked",
      "team.created",
      "team.updated",
      "team.deleted",
      "team.member_added",
      "team.member_removed",
      "role.created",
      "role.updated",
      "role.deleted",
      "member.custom_role_changed",
    ],
  },
  { id: "tokens", events: ["token.created", "token.revoked"] },
  {
    id: "inbox",
    events: [
      "inbox.access_changed",
      "redaction_rule.created",
      "redaction_rule.updated",
      "redaction_rule.deleted",
      "redaction_settings.updated",
      "contact_erasure.requested",
      "contact_erasure.canceled",
      "contact_erasure.held",
      "contact_erasure.completed",
    ],
  },
  { id: "email", events: [] },
  { id: "export", events: ["audit_logs.exported", "members.exported"] },
];

/*
 * auditEvents are every audit event the browser app names.
 */
export const auditEvents = groups.flatMap((group) => group.events);

/*
 * auditEventGroups returns the groups with their names and event labels
 * in the reader's current language.
 */
export function auditEventGroups(): AuditEventGroup[] {
  return groups.map((group) => ({
    id: group.id,
    name: i18n.t(`settings:audit_groups.${group.id}`),
    events: Object.fromEntries(group.events.map((event) => [event, auditEventLabel(event)])),
  }));
}

/*
 * groupFilter is the event filter that keeps every event of the group.
 */
export function groupFilter(group: AuditEventGroup): string {
  return Object.keys(group.events).join(",");
}

/*
 * named returns the message under the key, or the fallback when the
 * key names no message.
 */
function named(key: string, fallback: string): string {
  return i18n.exists(key) ? i18n.t(key) : fallback;
}

export function auditEventLabel(event: string): string {
  return named(`settings:audit.${event}`, event);
}

export function securityEventLabel(type: string): string {
  return named(`settings:security.${type}`, type);
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
  const system = entry.actor.type === "system" ? i18n.t("settings:channels.automatic") : "";
  const channel = kind ? named(`settings:channels.${kind}`, kind) : system;

  return [channel, entry.ip].filter(Boolean).join(" · ");
}

/*
 * formatTime writes a moment as a date and a time in the reader's
 * language, in the time zone given, else the browser's.
 */
export function formatTime(iso: string, timeZone?: string): string {
  return formatDateTime(iso, { timeZone });
}
