/*
 * The workspace's audit log, on the server: who changed what, through
 * which channel, and when, newest first. Owners and admins narrow it to
 * one kind of event or one member and step through it a page at a time;
 * everyone else is told the log is not theirs to read.
 */
import { RiArrowLeftLine, RiArrowRightLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Select, SelectItem } from "@/components/base/select/select";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { type AuditEntry, type AuditFilters, auditLogsQuery } from "@/lib/api/audit";
import { ApiError } from "@/lib/api/client";
import { membersQuery } from "@/lib/api/workspace";

import { Empty, SettingsPage } from "../layout";
import { actorDetail, actorName, auditEventLabel, auditEvents, formatTime } from "../log-labels";
import { useAccount } from "../use-account";

const everything = "all";

/*
 * Filter picks one value of a filter, or everything.
 */
function Filter({
  label,
  every,
  options,
  value,
  onChange,
}: {
  label: string;
  every: string;
  options: { id: string; name: string }[];
  value: string | undefined;
  onChange: (value: string | undefined) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      selectedKey={value ?? everything}
      onSelectionChange={(key) => onChange(key === everything ? undefined : String(key))}
      popoverClassName="w-64"
    >
      {[{ id: everything, name: every }, ...options].map((option) => (
        <SelectItem key={option.id} id={option.id} textValue={option.name}>
          {option.name}
        </SelectItem>
      ))}
    </Select>
  );
}

function Entries({ entries, zone }: { entries: AuditEntry[]; zone?: string }) {
  return (
    <div className="overflow-hidden rounded-2xl bg-background-secondary-default">
      <Table aria-label="Audit log" size="sm" className="[&_th]:border-t-0">
        <TableHeader>
          <TableColumn isRowHeader>Event</TableColumn>
          <TableColumn>Actor</TableColumn>
          <TableColumn>Target</TableColumn>
          <TableColumn>Time</TableColumn>
        </TableHeader>
        <TableBody
          items={entries}
          renderEmptyState={() => <p className="py-2 text-body-2-regular text-text-tertiary">No events match.</p>}
        >
          {(entry) => (
            <TableRow id={entry.id}>
              <TableCell>
                <span className="flex flex-col">
                  <span className="text-body-2-medium text-text-primary">{auditEventLabel(entry.event)}</span>
                  <span className="font-mono text-caption-1-regular text-text-tertiary">{entry.event}</span>
                </span>
              </TableCell>
              <TableCell>
                <span className="flex flex-col">
                  <span title={entry.actor.label} className="text-body-2-regular text-text-primary">
                    {actorName(entry.actor.label)}
                  </span>
                  <span className="text-caption-1-regular text-text-tertiary">{actorDetail(entry)}</span>
                </span>
              </TableCell>
              <TableCell>
                <span className="text-body-2-regular text-text-secondary">{entry.subject?.label ?? "—"}</span>
              </TableCell>
              <TableCell>
                <span className="whitespace-nowrap text-body-2-regular text-text-secondary">
                  {formatTime(entry.created_at, undefined, zone)}
                </span>
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  );
}

export function AuditLogSettings() {
  const me = useAccount();
  const workspace = me.workspace.id;
  const [filters, setFilters] = useState<AuditFilters>({});
  const [cursors, setCursors] = useState<string[]>([]);
  const cursor = cursors.at(-1) ?? null;
  const { data: members = [] } = useQuery(membersQuery(workspace));
  const { data, error } = useQuery(auditLogsQuery(workspace, filters, cursor));
  const filter = (changes: AuditFilters) => {
    setFilters({ ...filters, ...changes });
    setCursors([]);
  };

  if (error instanceof ApiError && error.status === 403) {
    return (
      <SettingsPage title="Audit log" description="Who changed what in the workspace, and when.">
        <div className="flex flex-col rounded-2xl bg-background-secondary-default ps-3">
          <Empty>Only owners and admins can read the audit log.</Empty>
        </div>
      </SettingsPage>
    );
  }

  return (
    <SettingsPage title="Audit log" description="Kept for a year on cloud, for ever on self-host.">
      <section aria-label="Audit log" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <Filter
            label="Event"
            every="All events"
            options={Object.entries(auditEvents).map(([id, name]) => ({ id, name }))}
            value={filters.event}
            onChange={(event) => filter({ event })}
          />
          <Filter
            label="Actor"
            every="Everyone"
            options={members.map((m) => ({ id: m.id, name: m.user.name }))}
            value={filters.actor}
            onChange={(actor) => filter({ actor })}
          />
        </div>
        {error ? <p className="px-3 text-body-2-regular text-text-error-primary">{error.message}</p> : null}
        <Entries zone={me.displayZone} entries={data?.items ?? []} />
        <div className="flex justify-end gap-2">
          <Button
            variant="secondary"
            size="small"
            leadingIcon={RiArrowLeftLine}
            disabled={cursors.length === 0}
            onClick={() => setCursors(cursors.slice(0, -1))}
          >
            Newer
          </Button>
          <Button
            variant="secondary"
            size="small"
            trailingIcon={RiArrowRightLine}
            disabled={!data?.next}
            onClick={() => data?.next && setCursors([...cursors, data.next])}
          >
            Older
          </Button>
        </div>
      </section>
    </SettingsPage>
  );
}
