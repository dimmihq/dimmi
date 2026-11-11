import type { EnvironmentConfig } from "../../cdk/lib/config/environment.ts";

/** Record types this script ever writes. */
export type RecordType = "A" | "CNAME" | "MX" | "TXT";

/**
 * One record as this script sees it. `name` is the full host name, which
 * reads the same in a plan, a lookup and an error; Vercel's relative names
 * are only used at its API.
 */
export type DnsRecord = {
  type: RecordType;
  name: string;
  value: string;
  priority?: number;
  ttl: number;
};

/** What a deployed environment tells the script, read from its stack outputs. */
export type StackFacts = {
  /** The load balancer's public addresses. */
  addresses: string[];
  /** Every mail record, as the mail stack computed it. */
  mail: { type: "MX" | "CNAME" | "TXT"; name: string; value: string; priority?: number }[];
};

/** Low enough that a fix spreads in minutes, as Vercel's own default is. */
const TTL = 300;

/**
 * Every record the environment needs. The host gets one A record per load
 * balancer address: A records are allowed at a zone apex, where production's
 * host sits and a CNAME is not, and the addresses never change while the
 * load balancer lives.
 */
export function desiredRecords(config: EnvironmentConfig, facts: StackFacts): DnsRecord[] {
  if (facts.addresses.length === 0) {
    throw new Error("the environment lists no load balancer addresses");
  }

  const host = facts.addresses.map((value): DnsRecord => ({ type: "A", name: config.host, value, ttl: TTL }));
  const mail = facts.mail.map((record): DnsRecord => ({ ...record, name: trimDot(record.name), ttl: TTL }));

  for (const record of [...host, ...mail]) {
    if (record.name !== config.dnsZone && !record.name.endsWith(`.${config.dnsZone}`)) {
      throw new Error(`${record.name} is outside the zone ${config.dnsZone}`);
    }
  }

  return [...host, ...mail];
}

/** The same record, whatever case or trailing dot each side wrote it with. */
export function sameRecord(a: Omit<DnsRecord, "ttl">, b: Omit<DnsRecord, "ttl">): boolean {
  return a.type === b.type && sameName(a.name, b.name) && normalValue(a) === normalValue(b);
}

/** Host names compare without case and without a trailing dot. */
export function sameName(a: string, b: string): boolean {
  return trimDot(a).toLowerCase() === trimDot(b).toLowerCase();
}

/** A record's value as it compares: host names lowercased and undotted, text as written. */
export function normalValue(record: Pick<DnsRecord, "type" | "value">): string {
  return record.type === "TXT" ? record.value : trimDot(record.value).toLowerCase();
}

/** One line describing a record, as plans and reports print it. */
export function describe(record: Omit<DnsRecord, "ttl">): string {
  const priority = record.priority === undefined ? "" : `${record.priority} `;

  return `${record.type.padEnd(5)} ${record.name} -> ${priority}${record.value}`;
}

function trimDot(value: string): string {
  return value.replace(/\.$/, "");
}
