/*
 * The changelog editor's dialogs. Authors are either everyone who edits
 * the entry, kept up as they do, or up to five teammates picked by hand.
 * Publishing goes out now or at a time picked ahead, and can email the
 * people subscribed, once: an entry already mailed never mails again.
 * Settings hold the entry's address, when it says it shipped, and
 * deleting it.
 */
import { type CalendarDate, getLocalTimeZone, parseDate, Time, today } from "@internationalized/date";
import { RiCalendarScheduleLine, RiGroupLine, RiSendPlaneLine, RiUserFollowLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { DatePicker } from "@/components/base/date-picker/date-picker";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { TimeField } from "@/components/base/time-field/time-field";
import { untilOf } from "@/features/inbox/snooze/until";
import { type ChangelogEntry, type PublishOptions, subscribersQuery } from "@/lib/api/changelog";
import { membersQuery } from "@/lib/api/workspace";

import type { Draft } from "./entry-format";

const tallModal = "h-[min(640px,calc(100dvh-3rem))]";

/*
 * AuthorsDialog picks who an entry names as its authors.
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
  const [mode, setMode] = useState(chosen.length > 0 ? "picked" : "editors");
  const [picked, setPicked] = useState<string[]>(chosen);
  const [busy, setBusy] = useState(false);
  const active = members.filter((m) => m.status === "active");
  const toggle = (id: string, on: boolean) =>
    setPicked(on ? [...picked.filter((p) => p !== id), id].slice(-5) : picked.filter((p) => p !== id));

  return (
    <Modal size="md" className={tallModal} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader title="Authors" description="Shown under the title on the changelog." />
      <ModalBody className="flex flex-col gap-4 pb-4">
        <RadioGroup aria-label="Who is named" value={mode} onChange={setMode}>
          <RadioCard
            value="editors"
            icon={RiGroupLine}
            title="Everyone who edits it"
            description="Each teammate who saves the update is added, in the order they joined in."
          />
          <RadioCard
            value="picked"
            icon={RiUserFollowLine}
            title="Pick authors"
            description="Name up to five teammates, whoever edits it."
          />
        </RadioGroup>
        {mode === "picked" ? (
          <fieldset className="flex flex-col gap-2">
            <legend className="sr-only">Teammates</legend>
            {active.map((m) => (
              <Checkbox key={m.id} isSelected={picked.includes(m.id)} onChange={(on) => toggle(m.id, on)}>
                {m.user.name}
              </Checkbox>
            ))}
          </fieldset>
        ) : null}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button
          loading={busy}
          disabled={mode === "picked" && picked.length === 0}
          onClick={() => {
            setBusy(true);
            onSave(mode === "picked" ? picked : [])
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
