/*
 * The workspace's audit log, on the server: who changed what, through
 * which channel, and when, newest first. Owners and admins narrow it to
 * one event or a group of them, one member and a time range, step
 * through it a page at a time, open an entry in full, and download what
 * they see as a CSV file; everyone else is told the log is not theirs
 * to read.
 */
import { getLocalTimeZone, today } from "@internationalized/date";
import { RiArrowLeftLine, RiArrowRightLine, RiDownloadLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { DateRangePicker, type DateRangeValue } from "@/components/base/date-picker/date-range-picker";
import { Select, SelectItem, SelectSection } from "@/components/base/select/select";
import { Table, TableBody, TableCell, TableColumn, TableHeader, TableRow } from "@/components/base/table/table";
import { type AuditEntry, type AuditFilters, auditLogsQuery, exportAuditLog } from "@/lib/api/audit";
import { ApiError } from "@/lib/api/client";
import { membersQuery } from "@/lib/api/workspace";

import { AuditEntryDetail } from "../audit-entry-detail";
import { oldestDay, presetRanges, rangeFilters } from "../audit-range";
import { Empty, SettingsPage } from "../layout";
import { actorDetail, actorName, auditEventGroups, auditEventLabel, formatTime, groupFilter } from "../log-labels";
import { useAccount } from "../use-account";

const everything = "all";
const groupKey = "group:";

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

/*
 * EventFilter picks one event, or every event of a group, from the
 * events grouped by kind.
 */
function EventFilter({
  value,
  onChange,
}: {
  value: string | undefined;
  onChange: (event: string | undefined) => void;
}) {
  const groups = auditEventGroups.filter((group) => Object.keys(group.events).length > 0);
  const chosenGroup = groups.find((group) => groupFilter(group) === value);
  const selected = chosenGroup ? groupKey + chosenGroup.id : (value ?? everything);
  const choose = (key: string) => {
    const group = groups.find((g) => groupKey + g.id === key);
    onChange(key === everything ? undefined : group ? groupFilter(group) : key);
  };

  return (
    <Select
      aria-label="Event"
      size="sm"
      selectedKey={selected}
      onSelectionChange={(key) => choose(String(key))}
      popoverClassName="w-72"
    >
      <SelectItem id={everything} textValue="All events">
        All events
      </SelectItem>
      {groups.map((group) => (
        <SelectSection key={group.id} title={group.name}>
          <SelectItem id={groupKey + group.id} textValue={`Any ${group.name.toLowerCase()} event`}>
            {`Any ${group.name.toLowerCase()} event`}
          </SelectItem>
          {Object.entries(group.events).map(([id, name]) => (
            <SelectItem key={id} id={id} textValue={name}>
              {name}
            </SelectItem>
          ))}
        </SelectSection>
      ))}
    </Select>
  );
}

function Entries({ entries, zone, onOpen }: { entries: AuditEntry[]; zone?: string; onOpen: (e: AuditEntry) => void }) {
  return (
    <div className="overflow-hidden rounded-2xl bg-background-secondary-default">
      <Table
        aria-label="Audit log"
        size="sm"
        className="[&_th]:border-t-0"
        onRowAction={(key) => {
          const entry = entries.find((e) => e.id === key);
          if (entry) {
            onOpen(entry);
          }
        }}
      >
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
            <TableRow id={entry.id} className="cursor-pointer">
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

/*
 * save hands the browser a downloaded file under its own name.
 */
function save(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
}

export function AuditLogSettings() {
  const me = useAccount();
  const workspace = me.workspace.id;
  const zone = me.displayZone ?? getLocalTimeZone();
  const [filters, setFilters] = useState<AuditFilters>({});
  const [range, setRange] = useState<{ value: DateRangeValue | null; preset: string | null }>({
    value: null,
    preset: null,
  });
  const [cursors, setCursors] = useState<string[]>([]);
  const [open, setOpen] = useState<AuditEntry | null>(null);
  const cursor = cursors.at(-1) ?? null;
  const { data: members = [] } = useQuery(membersQuery(workspace));
  const { data, error } = useQuery(auditLogsQuery(workspace, filters, cursor));
  const download = useMutation({
    mutationFn: () => exportAuditLog(workspace, filters),
    onSuccess: (file) => save(file.blob, file.filename),
  });
  const filter = (changes: AuditFilters) => {
    setFilters({ ...filters, ...changes });
    setCursors([]);
  };
  const pickRange = (value: DateRangeValue | null, preset: string | null) => {
    setRange({ value, preset });
    filter({ from: undefined, to: undefined, ...rangeFilters(value, preset, zone, new Date()) });
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
          <EventFilter value={filters.event} onChange={(event) => filter({ event })} />
          <Filter
            label="Actor"
            every="Everyone"
            options={members.map((m) => ({ id: m.id, name: m.user.name }))}
            value={filters.actor}
            onChange={(actor) => filter({ actor })}
          />
          <DateRangePicker
            aria-label="Time range"
            placeholder="Any time"
            className="py-1.5"
            value={range.value}
            preset={range.preset}
            presets={presetRanges(zone)}
            minValue={oldestDay(me.auditRetentionDays, zone)}
            maxValue={today(zone)}
            onChange={pickRange}
          />
          {range.value ? (
            <Button variant="secondary" size="small" onClick={() => pickRange(null, null)}>
              Clear dates
            </Button>
          ) : null}
          <Button
            variant="secondary"
            size="small"
            leadingIcon={RiDownloadLine}
            className="ms-auto"
            disabled={download.isPending}
            onClick={() => download.mutate()}
          >
            Export CSV
          </Button>
        </div>
        {error ? <p className="px-3 text-body-2-regular text-text-error-primary">{error.message}</p> : null}
        {download.error ? (
          <p role="alert" className="px-3 text-body-2-regular text-text-error-primary">
            {download.error.message}
          </p>
        ) : null}
        <Entries zone={me.displayZone} entries={data?.items ?? []} onOpen={setOpen} />
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
      <AuditEntryDetail entry={open} zone={me.displayZone} onClose={() => setOpen(null)} />
    </SettingsPage>
  );
}
