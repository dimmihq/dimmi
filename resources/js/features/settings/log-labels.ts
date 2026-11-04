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
 * auditEvents are the audit events the browser app names, in the order
 * the audit log's filter offers them.
 */
export const auditEvents = [
  "workspace.created",
  "workspace.updated",
  "member.joined",
  "member.role_changed",
  "ownership.transferred",
  "member.deactivated",
  "member.reactivated",
  "member.left",
  "workspace.deleted",
  "workspace.restored",
  "token.created",
  "token.revoked",
  "invitation.created",
  "invitation.accepted",
  "invitation.revoked",
  "team.created",
  "team.updated",
  "team.deleted",
  "team.member_added",
  "team.member_removed",
];

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
