import type { DnsRecord } from "./records.ts";
import { describe, sameName, sameRecord } from "./records.ts";
import type { LiveRecord } from "./vercel.ts";

/**
 * The comment that marks a record as written by this script for one
 * environment. The script changes and deletes only records carrying its own
 * environment's marker; everything else in the zone is someone else's.
 */
export function ownerComment(environmentName: string): string {
  return `managed by dimmi deploy/dns for ${environmentName}; edits are overwritten`;
}

export type Plan = {
  create: DnsRecord[];
  update: { id: string; record: DnsRecord }[];
  remove: LiveRecord[];
  /** Desired records that already exist by hand. Left alone, never duplicated. */
  unmanaged: LiveRecord[];
  /** Desired records that would clash with a record this script does not own. */
  conflicts: { record: DnsRecord; existing: LiveRecord }[];
  unchanged: number;
};

/**
 * Compares what the environment needs with what the zone holds. Records are
 * matched by type, name and value, so a name with several values (two A
 * records, several TXT) is handled one value at a time.
 */
export function plan(desired: DnsRecord[], live: LiveRecord[], environmentName: string): Plan {
  const marker = ownerComment(environmentName);
  const owned = live.filter((record) => record.comment === marker);
  const result: Plan = { create: [], update: [], remove: [], unmanaged: [], conflicts: [], unchanged: 0 };

  for (const record of desired) {
    const existing = live.find((candidate) => sameRecord(candidate as DnsRecord, record));

    if (existing && existing.comment !== marker) {
      result.unmanaged.push(existing);
      continue;
    }
    if (existing) {
      const drifted = existing.ttl !== record.ttl || (record.type === "MX" && existing.priority !== record.priority);
      if (drifted) {
        result.update.push({ id: existing.id, record });
      } else {
        result.unchanged += 1;
      }
      continue;
    }

    const clash = live.find((candidate) => candidate.comment !== marker && clashes(candidate, record));
    if (clash) {
      result.conflicts.push({ record, existing: clash });
      continue;
    }

    result.create.push(record);
  }

  result.remove = owned.filter((record) => !desired.some((wanted) => sameRecord(record as DnsRecord, wanted)));

  return result;
}

/**
 * Whether a record someone else owns stops this one from being added.
 * Nothing may sit beside a CNAME; a stray A or MX at the same name would
 * send some traffic elsewhere; two SPF records make SPF fail.
 */
function clashes(existing: LiveRecord, record: DnsRecord): boolean {
  if (!sameName(existing.name, record.name)) {
    return false;
  }
  if (existing.type === "CNAME" || record.type === "CNAME") {
    return true;
  }
  if (existing.type === record.type && (record.type === "A" || record.type === "MX")) {
    return true;
  }

  return record.type === "TXT" && existing.type === "TXT" && isSpf(existing.value) && isSpf(record.value);
}

function isSpf(value: string): boolean {
  return value.toLowerCase().startsWith("v=spf1");
}

/** The plan in plain words, one line per change. */
export function formatPlan(result: Plan): string {
  const lines = [
    ...result.create.map((record) => `+ create  ${describe(record)}`),
    ...result.update.map(({ record }) => `~ update  ${describe(record)} (ttl or priority)`),
    ...result.remove.map((record) => `- delete  ${describe(record as DnsRecord)}`),
    ...result.unmanaged.map((record) => `= exists  ${describe(record as DnsRecord)} (added by hand, left alone)`),
    ...result.conflicts.map(
      ({ record, existing }) =>
        `! blocked ${describe(record)}: clashes with ${describe(existing as DnsRecord)}, which this script does not own`,
    ),
  ];
  const summary =
    `${result.create.length} to create, ${result.update.length} to update, ${result.remove.length} to delete, ` +
    `${result.unchanged} unchanged, ${result.conflicts.length} blocked`;

  return [...lines, summary].join("\n");
}
