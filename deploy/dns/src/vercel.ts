import type { DnsRecord, RecordType } from "./records.ts";

/** A record as it exists at Vercel, with its full name and its comment. */
export type LiveRecord = Omit<DnsRecord, "type"> & { id: string; type: string; comment: string };

/** Just enough of `fetch` to talk to the API, so tests can answer for it. */
export type Fetch = (
  url: string,
  init: { method: string; headers: Record<string, string>; body?: string },
) => Promise<{
  ok: boolean;
  status: number;
  json(): Promise<unknown>;
  text(): Promise<string>;
}>;

export type VercelOptions = {
  token: string;
  /** The zone, such as `trydimmi.com`. */
  domain: string;
  /** Set when the domain belongs to a team rather than a personal account. */
  teamId?: string;
  fetch: Fetch;
};

type ApiRecord = {
  id: string;
  name: string;
  type: string;
  value: string;
  mxPriority?: number;
  ttl?: number;
  comment?: string;
};

type ApiPage = { records: ApiRecord[]; pagination?: { next: number | null } };

const API = "https://api.vercel.com";
const PAGE_SIZE = 100;

/**
 * The four calls the script makes to Vercel's DNS API: list, create, update
 * and delete. Names cross the boundary here: Vercel wants them relative to
 * the zone, with an empty string for the apex.
 */
export class Vercel {
  private readonly options: VercelOptions;

  constructor(options: VercelOptions) {
    this.options = options;
  }

  /** Every record in the zone, following pagination to the end. */
  async list(): Promise<LiveRecord[]> {
    const records: LiveRecord[] = [];
    let until: number | null = null;

    do {
      const query: Record<string, string> = { limit: String(PAGE_SIZE) };
      if (until !== null) {
        query.until = String(until);
      }
      const page = (await this.call("GET", `/v5/domains/${this.options.domain}/records`, query)) as ApiPage;
      records.push(...page.records.map((record) => this.fromApi(record)));
      until = page.pagination?.next ?? null;
    } while (until !== null);

    return records;
  }

  async create(record: DnsRecord, comment: string): Promise<void> {
    await this.call("POST", `/v2/domains/${this.options.domain}/records`, {}, this.toApi(record, comment));
  }

  async update(id: string, record: DnsRecord, comment: string): Promise<void> {
    await this.call("PATCH", `/v1/domains/records/${id}`, {}, this.toApi(record, comment));
  }

  async remove(id: string): Promise<void> {
    await this.call("DELETE", `/v2/domains/${this.options.domain}/records/${id}`, {});
  }

  private toApi(record: DnsRecord, comment: string): Record<string, unknown> {
    return {
      type: record.type satisfies RecordType,
      name: this.relative(record.name),
      value: record.value,
      ttl: record.ttl,
      comment,
      ...(record.type === "MX" ? { mxPriority: record.priority } : {}),
    };
  }

  private fromApi(record: ApiRecord): LiveRecord {
    return {
      id: record.id,
      type: record.type,
      name: record.name === "" ? this.options.domain : `${record.name}.${this.options.domain}`,
      value: record.value,
      priority: record.mxPriority,
      ttl: record.ttl ?? 0,
      comment: record.comment ?? "",
    };
  }

  private relative(name: string): string {
    const { domain } = this.options;
    if (name === domain) {
      return "";
    }
    if (!name.endsWith(`.${domain}`)) {
      throw new Error(`${name} is outside the zone ${domain}`);
    }

    return name.slice(0, -(domain.length + 1));
  }

  private async call(method: string, path: string, query: Record<string, string>, body?: unknown): Promise<unknown> {
    const params = new URLSearchParams(query);
    if (this.options.teamId) {
      params.set("teamId", this.options.teamId);
    }
    const search = params.size > 0 ? `?${params}` : "";
    const response = await this.options.fetch(`${API}${path}${search}`, {
      method,
      headers: { Authorization: `Bearer ${this.options.token}`, "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
    });

    if (!response.ok) {
      throw new Error(`Vercel ${method} ${path} answered ${response.status}: ${await response.text()}`);
    }

    return method === "DELETE" ? undefined : response.json();
  }
}
