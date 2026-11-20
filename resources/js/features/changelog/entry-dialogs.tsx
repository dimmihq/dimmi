/*
 * The changelog editor's dialogs. Authors are either everyone who edits
 * the entry, kept up as they do, or up to five teammates picked by hand.
 * Publishing goes out now or at a time picked ahead, and can email the
 * people subscribed, once: an entry already mailed never mails again.
 * Settings hold the entry's address, when it says it shipped, and
 * deleting it.
 */
import { type CalendarDate, getLocalTimeZone, parseDate, Time, today } from "@internationalized/date";
import { RiCalendarScheduleLine, RiGroupLine, RiSearchLine, RiSendPlaneLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { DatePicker } from "@/components/base/date-picker/date-picker";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { TimeField } from "@/components/base/time-field/time-field";
import { untilOf } from "@/features/inbox/snooze/until";
import { avatarColor } from "@/features/shell/avatar-color";
import { type ChangelogEntry, type PublishOptions, subscribersQuery } from "@/lib/api/changelog";
import { membersQuery } from "@/lib/api/workspace";

import type { Draft } from "./entry-format";

const tallModal = "h-[min(640px,calc(100dvh-3rem))]";

const everyone = "editors";
const chosenOnes = "picked";

/*
 * mostAuthors is how many teammates an entry can name.
 */
const mostAuthors = 5;

/*
 * AuthorsDialog picks who an entry names as its authors: everyone who
 * edits it, or up to five teammates found by name and ticked, shown in
 * the order they were picked.
 */
export function AuthorsDialog({
  workspace,
  chosen,
  onSave,
  onClose,
}: {
  workspace: string;
  chosen: string[];
  onSave: (authors: string[]) => Promise<unknown>;
  onClose: () => void;
}) {
  const { data: members = [] } = useQuery({ ...membersQuery(workspace, {}), enabled: workspace !== "" });
  const [mode, setMode] = useState(chosen.length > 0 ? chosenOnes : everyone);
  const [picked, setPicked] = useState<string[]>(chosen);
  const [query, setQuery] = useState("");
  const [busy, setBusy] = useState(false);
  const active = members.filter((m) => m.status === "active");
  const needle = query.trim().toLowerCase();
  const shown = active.filter((m) => `${m.user.name} ${m.user.email}`.toLowerCase().includes(needle));
  const full = picked.length >= mostAuthors;
  const toggle = (id: string) =>
    setPicked(picked.includes(id) ? picked.filter((p) => p !== id) : full ? picked : [...picked, id]);

  return (
    <Modal size="md" className={tallModal} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title="Authors" description="Shown under the title on the changelog." />
      <ModalBody className="flex min-h-0 flex-col gap-4 pb-2">
        <SegmentedControl
          aria-label="Who is named"
          className="self-start"
          selectedKeys={new Set([mode])}
          onSelectionChange={(keys) => {
            const [key] = [...keys];
            if (key === everyone || key === chosenOnes) {
              setMode(key);
            }
          }}
        >
          <SegmentedControlItem id={everyone}>Everyone who edits</SegmentedControlItem>
          <SegmentedControlItem id={chosenOnes}>Picked teammates</SegmentedControlItem>
        </SegmentedControl>
        {mode === everyone ? (
          <div className="flex flex-1 flex-col items-center justify-center gap-2 rounded-2xl bg-background-secondary-default px-6 py-10 text-center">
            <RiGroupLine aria-hidden className="size-6 text-foreground-icon-tertiary" />
            <p className="text-body-medium text-text-primary">Everyone who edits it is named</p>
            <p className="max-w-xs text-body-2-regular text-text-secondary">
              Each teammate who saves the update is added, in the order they first saved.
            </p>
          </div>
        ) : (
          <>
            <Input
              aria-label="Search teammates"
              placeholder="Search teammates"
              leadingIcon={RiSearchLine}
              autoFocus
              value={query}
              onChange={setQuery}
            />
            <div className="flex min-h-0 flex-1 flex-col overflow-y-auto rounded-2xl bg-background-secondary-default p-1">
              {shown.map((m) => {
                const on = picked.includes(m.id);
                const place = picked.indexOf(m.id) + 1;

                return (
                  <button
                    key={m.id}
                    type="button"
                    aria-pressed={on}
                    disabled={!on && full}
                    onClick={() => toggle(m.id)}
                    className="flex min-h-11 w-full cursor-pointer items-center gap-3 rounded-xl px-2.5 text-start outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    <Avatar
                      size="xs"
                      src={m.user.avatar_url ?? undefined}
                      color={avatarColor(m.user.name)}
                      initials={m.user.name.charAt(0)}
                    />
                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate text-body-2-medium text-text-primary">{m.user.name}</span>
                      <span className="truncate text-caption-1-regular text-text-tertiary">{m.user.email}</span>
                    </span>
                    {on ? (
                      <span className="grid size-5 shrink-0 place-items-center rounded-full bg-button-primary text-caption-2-medium text-text-white tabular-nums">
                        {place}
                      </span>
                    ) : null}
                  </button>
                );
              })}
              {shown.length === 0 ? (
                <p className="px-2.5 py-3 text-body-2-regular text-text-tertiary">No teammate matches.</p>
              ) : null}
            </div>
            <p className="text-caption-1-regular text-text-tertiary">
              {picked.length} of {mostAuthors} picked. The number is the order they show in.
            </p>
          </>
        )}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button
          loading={busy}
          disabled={mode === chosenOnes && picked.length === 0}
          onClick={() => {
            setBusy(true);
            onSave(mode === chosenOnes ? picked : [])
              .then(onClose)
              .finally(() => setBusy(false));
          }}
        >
          Save authors
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * NotifyChoice offers to email the subscribers, or says they already
 * were.
 */
function NotifyChoice({
  mailed,
  subscribers,
  notify,
  onChange,
}: {
  mailed: boolean;
  subscribers: number;
  notify: boolean;
  onChange: (notify: boolean) => void;
}) {
  if (mailed) {
    return (
      <p className="text-body-2-regular text-text-tertiary">Subscribers were already emailed about this update.</p>
    );
  }
  const people = subscribers === 1 ? "subscriber" : "subscribers";

  return (
    <Checkbox isSelected={notify} onChange={onChange} isDisabled={subscribers === 0}>
      {subscribers === 0 ? "Email subscribers (nobody has subscribed yet)" : `Email ${subscribers} ${people}`}
    </Checkbox>
  );
}

/*
 * PublishDialog publishes now or schedules, and offers to email the
 * subscribers while the entry has never been mailed.
 */
export function PublishDialog({
  workspace,
  entry,
  onPublish,
  onClose,
}: {
  workspace: string;
  entry: ChangelogEntry;
  onPublish: (options: PublishOptions) => Promise<unknown>;
  onClose: () => void;
}) {
  const { data: active } = useQuery(subscribersQuery(workspace, entry.help_center_id, { status: "active", page: 1 }));
  const subscribers = active?.total ?? 0;
  const mailed = entry.notified_at !== null;
  const [when, setWhen] = useState(entry.status === "scheduled" ? "later" : "now");
  const [day, setDay] = useState<CalendarDate | null>(today(getLocalTimeZone()).add({ days: 1 }));
  const [time, setTime] = useState<Time | null>(new Time(9, 0));
  const [chosen, setNotify] = useState<boolean | null>(null);
  const notify = chosen ?? (!mailed && subscribers > 0);
  const [busy, setBusy] = useState(false);
  const at =
    day && time ? untilOf(day, `${String(time.hour).padStart(2, "0")}:${String(time.minute).padStart(2, "0")}`) : null;
  const options: PublishOptions = when === "later" && at ? { notify, scheduled_for: at } : { notify };

  return (
    <Modal size="md" className={tallModal} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title="Publish update" description="It appears on the changelog and in its feed." />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <RadioGroup aria-label="When" value={when} onChange={setWhen}>
          <RadioCard value="now" icon={RiSendPlaneLine} title="Publish now" description="Readers see it right away." />
          <RadioCard
            value="later"
            icon={RiCalendarScheduleLine}
            title="Schedule"
            description="It publishes itself at the time you pick."
          />
        </RadioGroup>
        {when === "later" ? (
          <div className="flex flex-col gap-1.5">
            <Label>Publish on</Label>
            <div className="flex gap-2">
              <DatePicker aria-label="Publish date" value={day} onChange={setDay} />
              <TimeField aria-label="Publish time" size="small" value={time} onChange={setTime} className="w-32" />
            </div>
          </div>
        ) : null}
        <NotifyChoice mailed={mailed} subscribers={subscribers} notify={notify} onChange={setNotify} />
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button
          loading={busy}
          disabled={when === "later" && !at}
          onClick={() => {
            setBusy(true);
            onPublish(options)
              .then(onClose)
              .finally(() => setBusy(false));
          }}
        >
          {when === "later" ? "Schedule" : "Publish"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/*
 * EntrySettings holds the entry's address, its shipped date once
 * published, and deleting it.
 */
export function EntrySettings({
  entry,
  draft,
  onChange,
  onDate,
  onDelete,
  onClose,
}: {
  entry: ChangelogEntry;
  draft: Draft;
  onChange: (change: Partial<Draft>) => void;
  onDate: (day: string) => Promise<unknown>;
  onDelete: () => Promise<unknown>;
  onClose: () => void;
}) {
  const [deleting, setDeleting] = useState(false);
  const shipped = entry.published_at ? parseDate(entry.published_at.slice(0, 10)) : null;

  return (
    <Modal size="md" className={tallModal} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title="Update settings" description="The address is saved with the update." />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <Input
          label="Address"
          hint="The last part of the update's link."
          value={draft.slug}
          onChange={(slug) => onChange({ slug })}
          maxLength={120}
          inputDir="ltr"
        />
        {entry.status === "published" ? (
          <div className="flex flex-col gap-1.5">
            <Label>Shipped on</Label>
            <DatePicker
              aria-label="Shipped on"
              value={shipped}
              isDisabled={entry.has_unpublished_changes}
              onChange={(day) => (day ? void onDate(`${day.toString()}T12:00:00Z`) : undefined)}
            />
            <p className="text-body-2-regular text-text-tertiary">
              {entry.has_unpublished_changes
                ? "Publish your changes first, then change the date."
                : "Backdate an update that went out earlier."}
            </p>
          </div>
        ) : null}
        <div className="mt-2 border-t border-separator-border pt-4">
          <Button variant="danger-secondary" onClick={() => setDeleting(true)}>
            Delete update
          </Button>
        </div>
      </ModalBody>
      <ModalFooter>
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
      <AlertDialog
        isOpen={deleting}
        onOpenChange={(open) => (open ? undefined : setDeleting(false))}
        title="Delete this update?"
        description="It leaves the changelog and its feed right away and cannot be brought back."
        confirmLabel="Delete update"
        onConfirm={onDelete}
      />
    </Modal>
  );
}
