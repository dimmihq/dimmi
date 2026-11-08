/*
 * The workspace's office hours: when the team is around, as workflows
 * read it to run only in or out of hours, wait until the team is back
 * and count only open time. Each weekday is open from one time to
 * another, or closed; holidays close a whole day. Hours are kept in
 * their own time zone, so daylight saving never moves them. The page
 * edits the default schedule, which every workflow uses unless it names
 * another; changes apply to waits that start after saving.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Action, Danger, Field, Toggle } from "@/features/settings/controls";
import { usePermission } from "@/features/settings/inboxes";
import type { Holiday, Schedule, Weekday } from "@/lib/api/schedules";
import { schedulesQuery, updateSchedule } from "@/lib/api/schedules";
import { Item, Row, RowsSkeleton, Section, SettingsPage } from "../layout";
import { sameWeek, statusLine, withDay } from "../office-hours";
import { ZonePicker } from "../preferences/zone-picker";
import { useAccount } from "../use-account";

const days: [Weekday, string][] = [
  ["mon", "Monday"],
  ["tue", "Tuesday"],
  ["wed", "Wednesday"],
  ["thu", "Thursday"],
  ["fri", "Friday"],
  ["sat", "Saturday"],
  ["sun", "Sunday"],
];

function DayRow({
  label,
  range,
  readOnly,
  onChange,
}: {
  label: string;
  range: [string, string] | undefined;
  readOnly: boolean;
  onChange: (range: [string, string] | null) => void;
}) {
  const [start, end] = range ?? ["09:00", "17:00"];

  return (
    <Row label={label} description={range ? undefined : "Closed"}>
      {range ? (
        <span className="flex items-center gap-2">
          <Field
            label={`${label} opens`}
            value={start}
            width="w-20"
            disabled={readOnly}
            onChange={(v) => onChange([v, end])}
          />
          <span className="text-body-2-regular text-text-tertiary">to</span>
          <Field
            label={`${label} closes`}
            value={end}
            width="w-20"
            disabled={readOnly}
            onChange={(v) => onChange([start, v])}
          />
        </span>
      ) : null}
      <Toggle
        label={`Open on ${label}`}
        on={range !== undefined}
        disabled={readOnly}
        onChange={(on) => onChange(on ? [start, end] : null)}
      />
    </Row>
  );
}

function Holidays({
  holidays,
  readOnly,
  onChange,
}: {
  holidays: Holiday[];
  readOnly: boolean;
  onChange: (holidays: Holiday[]) => void;
}) {
  const [date, setDate] = useState("");
  const [name, setName] = useState("");

  return (
    <Section title="Holidays" description="Closed the whole day, every workflow included.">
      {holidays.map((h) => (
        <Item key={h.date} mark={null} title={h.name || "Holiday"} detail={h.date}>
          {readOnly ? null : (
            <Danger onPress={() => onChange(holidays.filter((other) => other.date !== h.date))}>Remove</Danger>
          )}
        </Item>
      ))}
      {readOnly ? null : (
        <Row label="Add a holiday">
          <span className="flex items-center gap-2">
            <Field label="Holiday date" placeholder="2026-12-25" value={date} width="w-28" onChange={setDate} />
            <Field label="Holiday name" placeholder="Christmas" value={name} width="w-32" onChange={setName} />
            <Action
              onPress={() => {
                if (date.trim()) {
                  onChange([...holidays, { date: date.trim(), name: name.trim() || undefined }]);
                }
              }}
            >
              Add
            </Action>
          </span>
        </Row>
      )}
    </Section>
  );
}

function Editor({ schedule, workspace }: { schedule: Schedule; workspace: string }) {
  const queryClient = useQueryClient();
  const readOnly = !usePermission(workspace, "schedules:manage");
  const [weekly, setWeekly] = useState(schedule.weekly);
  const [holidays, setHolidays] = useState(schedule.holidays);
  const save = useMutation({
    mutationFn: (changes: Parameters<typeof updateSchedule>[2]) => updateSchedule(workspace, schedule.id, changes),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: schedulesQuery(workspace).queryKey }),
  });
  const zones = ["UTC", ...Intl.supportedValuesOf("timeZone")].map((id) => ({ id, label: id }));
  const changed = !sameWeek(weekly, schedule.weekly);

  return (
    <>
      <Section title="Right now">
        <Row label={statusLine(schedule)} description={`Hours are kept in ${schedule.timezone}.`}>
          {readOnly ? null : (
            <ZonePicker
              label="Office hours time zone"
              choices={zones}
              selected={schedule.timezone}
              onPick={(timezone) => save.mutate({ timezone })}
            />
          )}
        </Row>
      </Section>
      <Section
        title="Weekly hours"
        description="Workflows can run only in or out of these hours, wait for them, and count only them."
        action={
          readOnly ? undefined : (
            <Button size="small" loading={save.isPending} disabled={!changed} onClick={() => save.mutate({ weekly })}>
              Save hours
            </Button>
          )
        }
      >
        {days.map(([day, label]) => (
          <DayRow
            key={day}
            label={label}
            range={weekly[day]?.[0]}
            readOnly={readOnly}
            onChange={(range) => setWeekly((current) => withDay(current, day, range))}
          />
        ))}
        {save.error ? (
          <p className="px-3 py-2 text-body-2-regular text-text-error-primary">{save.error.message}</p>
        ) : null}
      </Section>
      <Holidays
        holidays={holidays}
        readOnly={readOnly}
        onChange={(next) => {
          setHolidays(next);
          save.mutate({ holidays: next });
        }}
      />
    </>
  );
}

export function OfficeHoursSettings() {
  const workspace = useAccount().workspace.id;
  const { data } = useQuery(schedulesQuery(workspace));
  const schedule = data?.find((s) => s.is_default);

  return (
    <SettingsPage title="Office hours" description="When your team is around to reply.">
      {schedule ? (
        <Editor key={`${schedule.id}-${schedule.timezone}`} schedule={schedule} workspace={workspace} />
      ) : (
        <Section title="Weekly hours">
          <RowsSkeleton rows={7} />
        </Section>
      )}
    </SettingsPage>
  );
}
