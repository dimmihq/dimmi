import { environments } from "../../cdk/lib/config/environments.ts";
import type { OutputsFile } from "../src/outputs.ts";
import type { LiveRecord } from "../src/vercel.ts";

/** Staging behind two documentation addresses, so plans show one A record each. */
export const STAGING = { ...environments.staging, ingressAddresses: ["192.0.2.10", "192.0.2.11"] };

/** Outputs as `cdk deploy --outputs-file` writes them for staging, with documentation addresses. */
export const STAGING_OUTPUTS: OutputsFile = {
  "dimmi-staging-mail": {
    DnsRecords: JSON.stringify([
      { type: "MX", name: "inbound.staging.trydimmi.com", value: "inbound-smtp.us-east-2.amazonaws.com", priority: 10 },
      { type: "TXT", name: "inbound.staging.trydimmi.com", value: "v=spf1 include:amazonses.com -all" },
      { type: "CNAME", name: "abc._domainkey.inbound.staging.trydimmi.com", value: "abc.dkim.amazonses.com" },
    ]),
  },
};

/** A record at Vercel, with the defaults a listing gives. */
export function live(overrides: Partial<LiveRecord> & Pick<LiveRecord, "type" | "name" | "value">): LiveRecord {
  return { id: `rec_${overrides.name}_${overrides.value}`, ttl: 300, comment: "", ...overrides };
}
