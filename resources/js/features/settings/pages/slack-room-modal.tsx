/*
 * One Slack channel's settings: who counts as the customer there, when
 * a message becomes a conversation, how the agent answers and how long
 * it waits, where new conversations are posted for the team, when an
 * automatic reply goes out, and joining matching channels on its own.
 * A team-only channel never opens conversations, so its trigger and
 * agent settings step aside. Saving sends only what changed.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { NumberField } from "@/components/base/number-field/number-field";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { toast } from "@/components/base/toast/toast";
import { type SlackRoom, slackRoomsQuery, updateSlackRoom } from "@/lib/api/integrations";

import {
  agentModes,
  delayWhens,
  draftOf,
  type RoomDraft,
  roomChange,
  roomModes,
  roomTriggers,
  sensitivities,
} from "../integrations";

/*
 * OptionSelect picks one of the options, with the picked one explained
 * under it.
 */
function OptionSelect<T extends string>({
  label,
  options,
  value,
  disabled,
  onChange,
}: {
  label: string;
  options: { id: T; label: string; detail: string }[];
  value: T;
  disabled: boolean;
  onChange: (value: T) => void;
}) {
  const picked = options.find((option) => option.id === value);

  return (
    <div className="flex flex-col gap-1">
      <Select
        label={label}
        isDisabled={disabled}
        selectedKey={value}
        onSelectionChange={(next) => next !== null && onChange(String(next) as T)}
      >
        {options.map((option) => (
          <SelectItem key={option.id} id={option.id} textValue={option.label}>
            {option.label}
          </SelectItem>
        ))}
      </Select>
      {picked ? <p className="text-body-2-regular text-text-tertiary">{picked.detail}</p> : null}
    </div>
  );
}

/*
 * Toggle is a setting turned on or off, with what it does beside it.
 */
function Toggle({
  label,
  detail,
  on,
  disabled,
  onChange,
}: {
  label: string;
  detail: string;
  on: boolean;
  disabled: boolean;
  onChange: (on: boolean) => void;
}) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="flex min-w-0 flex-col">
        <span className="text-body-medium text-text-primary">{label}</span>
        <span className="text-body-2-regular text-text-tertiary">{detail}</span>
      </div>
      <Switch aria-label={label} size="sm" isSelected={on} isDisabled={disabled} onChange={onChange} />
    </div>
  );
}

/*
 * nowhere is the choice of no triage channel.
 */
const nowhere = "none";

/*
 * TriageSelect picks the team channel new conversations are posted to,
 * from the connection's other channels the app is still in.
 */
function TriageSelect({
  room,
  rooms,
  value,
  disabled,
  onChange,
}: {
  room: SlackRoom;
  rooms: SlackRoom[];
  value: string | null;
  disabled: boolean;
  onChange: (value: string | null) => void;
}) {
  const choices = rooms.filter((other) => other.id !== room.id && other.status !== "removed");

  return (
    <div className="flex flex-col gap-1">
      <Select
        label="Post new conversations to"
        isDisabled={disabled}
        selectedKey={value ?? nowhere}
        onSelectionChange={(next) => next !== null && onChange(next === nowhere ? null : String(next))}
      >
        <SelectItem id={nowhere} textValue="Nowhere">
          Nowhere
        </SelectItem>
        {choices.map((other) => (
          <SelectItem key={other.id} id={other.external_id} textValue={other.name}>
            {other.name}
          </SelectItem>
        ))}
      </Select>
      <p className="text-body-2-regular text-text-tertiary">
        A team channel where each new conversation is posted once, so someone can take it.
      </p>
    </div>
  );
}

/*
 * AgentFields are how the agent answers in the channel and how long it
 * waits for a teammate first.
 */
function AgentFields({
  draft,
  disabled,
  set,
}: {
  draft: RoomDraft;
  disabled: boolean;
  set: (change: Partial<RoomDraft>) => void;
}) {
  return (
    <>
      <OptionSelect
        label="The agent"
        options={agentModes}
        value={draft.agent_mode}
        disabled={disabled}
        onChange={(agent_mode) => set({ agent_mode })}
      />
      <NumberField
        label="Wait before the agent answers (seconds)"
        hint="0 answers at once. A teammate who replies first stops it. Up to 900."
        minValue={0}
        maxValue={900}
        isDisabled={disabled}
        value={draft.agent_delay_seconds}
        onChange={(seconds) => set({ agent_delay_seconds: Number.isNaN(seconds) ? 0 : seconds })}
      />
      {draft.agent_delay_seconds > 0 ? (
        <OptionSelect
          label="Wait"
          options={delayWhens}
          value={draft.agent_delay_when}
          disabled={disabled}
          onChange={(agent_delay_when) => set({ agent_delay_when })}
        />
      ) : null}
    </>
  );
}

/*
 * ReplyFields are the automatic reply sent when nobody answers in time.
 */
function ReplyFields({
  draft,
  disabled,
  set,
}: {
  draft: RoomDraft;
  disabled: boolean;
  set: (change: Partial<RoomDraft>) => void;
}) {
  const on = draft.auto_reply_after_seconds !== null;

  return (
    <>
      <Toggle
        label="Automatic reply"
        detail="Tell the customer the team has seen their message when nobody answers in time."
        on={on}
        disabled={disabled}
        onChange={(next) => set({ auto_reply_after_seconds: next ? 300 : null })}
      />
      {on ? (
        <NumberField
          label="Send it after (seconds)"
          hint="Up to 900. A reply from anyone first cancels it."
          minValue={0}
          maxValue={900}
          isDisabled={disabled}
          value={draft.auto_reply_after_seconds ?? 300}
          onChange={(seconds) => set({ auto_reply_after_seconds: Number.isNaN(seconds) ? 300 : seconds })}
        />
      ) : null}
    </>
  );
}

/*
 * CustomerFields are the settings of a channel that opens
 * conversations.
 */
function CustomerFields({
  room,
  rooms,
  draft,
  disabled,
  set,
}: {
  room: SlackRoom;
  rooms: SlackRoom[];
  draft: RoomDraft;
  disabled: boolean;
  set: (change: Partial<RoomDraft>) => void;
}) {
  return (
    <>
      <OptionSelect
        label="Open a conversation for"
        options={roomTriggers}
        value={draft.trigger}
        disabled={disabled}
        onChange={(trigger) => set({ trigger })}
      />
      {draft.trigger === "ai" ? (
        <OptionSelect
          label="Count as a request"
          options={sensitivities}
          value={draft.request_sensitivity}
          disabled={disabled}
          onChange={(request_sensitivity) => set({ request_sensitivity })}
        />
      ) : null}
      <AgentFields draft={draft} disabled={disabled} set={set} />
      <TriageSelect
        room={room}
        rooms={rooms}
        value={draft.triage_channel_id}
        disabled={disabled}
        onChange={(triage_channel_id) => set({ triage_channel_id })}
      />
      <ReplyFields draft={draft} disabled={disabled} set={set} />
    </>
  );
}

export function RoomSettingsModal({
  workspace,
  room,
  rooms,
  canManage,
  onClose,
}: {
  workspace: string;
  room: SlackRoom;
  rooms: SlackRoom[];
  canManage: boolean;
  onClose: () => void;
}) {
  const formId = useId();
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<RoomDraft>(() => draftOf(room));
  const set = (change: Partial<RoomDraft>) => setDraft((current) => ({ ...current, ...change }));
  const change = roomChange(room, draft);
  const disabled = !canManage;

  const save = useMutation({
    mutationFn: () => updateSlackRoom(workspace, room, change ?? {}),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: slackRoomsQuery(workspace, room.channel_id).queryKey });
      toast.success(`${room.name} saved.`);
      onClose();
    },
  });

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={room.name}
        description={
          room.status === "removed"
            ? "The app is no longer in this channel. Invite @dimmi to the channel again; these settings come back with it."
            : "How dimmi watches this Slack channel."
        }
      />
      <ModalBody className="pb-4">
        <form
          id={formId}
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            if (change) {
              save.mutate();
            }
          }}
        >
          <OptionSelect
            label="Who is the customer"
            options={roomModes}
            value={draft.mode}
            disabled={disabled}
            onChange={(mode) => set({ mode })}
          />
          {draft.mode === "internal" ? null : (
            <CustomerFields room={room} rooms={rooms} draft={draft} disabled={disabled} set={set} />
          )}
          <Toggle
            label="Join matching channels"
            detail="Join new public channels named like this one by itself, with these settings."
            on={draft.auto_join}
            disabled={disabled}
            onChange={(auto_join) => set({ auto_join })}
          />
          {save.error ? <p className="text-body-2-regular text-text-error-primary">{save.error.message}</p> : null}
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          {canManage ? "Cancel" : "Close"}
        </Button>
        {canManage ? (
          <Button type="submit" form={formId} loading={save.isPending} disabled={change === null}>
            Save
          </Button>
        ) : null}
      </ModalFooter>
    </Modal>
  );
}
