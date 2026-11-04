import type { DnsRecord } from "./records.ts";
import { normalValue } from "./records.ts";

/** The lookups `verify` makes, as `node:dns/promises` offers them. */
export type Resolver = {
  resolve4(name: string): Promise<string[]>;
  resolveCname(name: string): Promise<string[]>;
  resolveMx(name: string): Promise<{ exchange: string; priority: number }[]>;
  resolveTxt(name: string): Promise<string[][]>;
};

export type Check = { record: DnsRecord; ok: boolean; message: string };

/**
 * Looks every record up in public DNS and says, in plain words, whether the
 * world sees it yet. A record that is right at Vercel but not yet visible is
 * usually waiting on caches, up to its TTL.
 */
export async function verify(records: DnsRecord[], resolver: Resolver): Promise<Check[]> {
  return Promise.all(records.map((record) => check(record, resolver)));
}

async function check(record: DnsRecord, resolver: Resolver): Promise<Check> {
  const wanted = normalValue(record);
  let seen: string[];

  try {
    seen = await lookup(record, resolver);
  } catch (error) {
    const code = (error as { code?: string }).code ?? String(error);
    return { record, ok: false, message: `${record.name} has no ${record.type} record yet (${code})` };
  }

  if (seen.includes(wanted)) {
    return { record, ok: true, message: `${record.name} ${record.type} points where it should` };
  }

  return {
    record,
    ok: false,
    message: `${record.name} ${record.type} shows ${seen.join(", ") || "nothing"}, expected ${record.value}`,
  };
}

async function lookup(record: DnsRecord, resolver: Resolver): Promise<string[]> {
  const host = (value: string) => normalValue({ type: "A", value });

  switch (record.type) {
    case "A":
      return resolver.resolve4(record.name);
    case "CNAME":
      return (await resolver.resolveCname(record.name)).map(host);
    case "MX":
      return (await resolver.resolveMx(record.name)).map((mx) => host(mx.exchange));
    case "TXT":
      return (await resolver.resolveTxt(record.name)).map((chunks) => chunks.join(""));
  }
}

/** The report `verify` prints, ending with a line a person can act on. */
export function formatChecks(checks: Check[]): string {
  const failed = checks.filter((item) => !item.ok).length;
  const lines = checks.map((item) => `${item.ok ? "ok     " : "MISSING"} ${item.message}`);
  const verdict =
    failed === 0
      ? "All records are visible. Certificates and SES verification can proceed."
      : `${failed} of ${checks.length} records are not visible yet. Wait for the TTL (5 minutes) and run verify again; if one stays wrong, run plan.`;

  return [...lines, verdict].join("\n");
}
