/*
 * The records a domain needs, one block each: what it is for, whether
 * the last check found it, its type, and its name and value to copy. Names
 * are shown the way the customer's DNS host wants them typed. Values are
 * DNS text, so they sit in a left-to-right region whatever the page's
 * direction, and a value found that differs is shown beside the one
 * expected.
 */
import { Row, Section } from "@/features/settings/layout";
import type { DomainRecord } from "@/lib/api/email-domains";

import { CopyIconButton } from "../setup/copy-button";
import { RecordStatusChip } from "./domain-status";

const purposeLabels: Record<string, string> = {
  ownership: "Ownership",
  dkim: "Signing key",
  dkim_next: "New signing key",
  return_path: "Return path",
  return_path_spf: "Return path SPF",
  spf: "SPF",
  dmarc: "DMARC",
};

/*
 * Value is one piece of a record, copyable unless it is short enough to
 * type, like the record's type.
 */
function Value({ label, value, copyable = true }: { label: string; value: string; copyable?: boolean }) {
  return (
    <div className="flex min-w-0 items-start gap-2">
      <span className="w-12 shrink-0 pt-1 text-caption-1-regular text-text-tertiary">{label}</span>
      <code dir="ltr" className="min-w-0 flex-1 break-all pt-0.5 font-mono text-body-2-regular text-text-primary">
        {value}
      </code>
      {copyable ? <CopyIconButton value={value} label={`Copy ${label.toLowerCase()}`} /> : null}
    </div>
  );
}

/*
 * RecordBlock is one record to publish.
 */
function RecordBlock({ record }: { record: DomainRecord }) {
  const label = purposeLabels[record.purpose] ?? record.purpose;
  const differs = record.status === "mismatch" && record.observed.length > 0;

  return (
    <div
      data-testid="domain-record"
      className="flex flex-col gap-2 border-b border-separator-border py-3 pe-3 last:border-b-0"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-body-regular text-text-primary">
          {label}
          {record.required ? null : <span className="text-text-tertiary"> · recommended</span>}
        </span>
        <RecordStatusChip status={record.status} />
      </div>
      <Value
        label="Type"
        value={record.priority ? `${record.type} ${record.priority}` : record.type}
        copyable={false}
      />
      <Value label="Name" value={record.host} />
      {record.value ? (
        <Value label="Value" value={record.value} />
      ) : (
        <p className="text-body-2-regular text-text-secondary">
          Copy this record from your email provider's dashboard
          {record.selector ? ` (its name usually starts with ${record.selector})` : ""}.
        </p>
      )}
      {differs ? (
        <p className="text-body-2-regular text-text-secondary">
          Found instead:{" "}
          <code dir="ltr" className="break-all font-mono text-text-primary">
            {record.observed.join(", ")}
          </code>
        </p>
      ) : null}
    </div>
  );
}

/*
 * DomainRecords lists the domain's records.
 */
export function DomainRecords({ records }: { records: DomainRecord[] }) {
  return (
    <Section title="DNS records" description="Add these at your DNS host. Required records prove the domain is yours.">
      {records.length === 0 ? <Row label="No records yet." /> : null}
      {records.map((record) => (
        <RecordBlock key={`${record.purpose}-${record.name}-${record.type}`} record={record} />
      ))}
    </Section>
  );
}
