/*
 * Adding a Slack channel the team hears in, or changing one: the
 * channel, picked from those the app can post in and fixed once added,
 * what it hears about, its digests and when they arrive, which inboxes
 * count, and whether customers' words show in the post.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useId, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Label } from "@/components/base/input/label";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { inboxesQuery } from "@/lib/api/inboxes";
import {
  createChannelDestination,
  type DestinationHearing,
  destinationsQuery,
  type NotificationDestination,
  slackPostablesQuery,
  updateDestination,
} from "@/lib/api/notification-destinations";

import { defaultHearing, destinationDigests, destinationTriggers, hearingOf, toggled } from "../destinations";

const halfHours = Array.from(
  { length: 48 },
  (_, i) => `${String(Math.floor(i / 2)).padStart(2, "0")}:${i % 2 ? "30" : "00"}`,
);

/*
 * ChannelPicker picks the Slack channel a new destination posts in.
 */
function ChannelPicker({
  workspace,
  integration,
  value,
  onChange,
}: {
  workspace: string;
  integration: string;
  value: string | null;
  onChange: (channel: string) => void;
}) {
  const { data: channels = [], isPending, error } = useQuery(slackPostablesQuery(workspace, integration));

  return (
    <div className="flex flex-col gap-1">
      <Select
        label="Channel"
        placeholder={isPending ? "Loading channels" : "Pick a channel"}
        selectedKey={value}
        onSelectionChange={(next) => next !== null && onChange(String(next))}
      >
        {channels.map((channel) => (
          <SelectItem key={channel.id} id={channel.id} textValue={`#${channel.name}`}>
            {`#${channel.name}${channel.is_private ? " (private)" : ""}`}
          </SelectItem>
        ))}
      </Select>
      <p className="text-body-2-regular text-text-tertiary">
        {error ? error.message : "Missing a private channel? Invite @dimmi to it in Slack first."}
      </p>
    </div>
  );
}

/*
 * InboxPicker narrows the destination to some inboxes, or every one.
 */
function InboxPicker({
  workspace,
  value,
  onChange,
}: {
  workspace: string;
  value: string[] | null;
  onChange: (inboxes: string[] | null) => void;
}) {
  const { data: inboxes = [] } = useQuery(inboxesQuery(workspace));

  return (
    <fieldset className="flex flex-col gap-2">
      <Label>Inboxes</Label>
      <Checkbox size="sm" isSelected={value === null} onChange={(all) => onChange(all ? null : [])}>
        Every inbox
      </Checkbox>
      {value === null
        ? null
        : inboxes.map((inbox) => (
            <Checkbox
              key={inbox.id}
              size="sm"
              isSelected={value.includes(inbox.id)}
              onChange={(on) => onChange(toggled(value, inbox.id, on))}
            >
              {inbox.name}
            </Checkbox>
          ))}
    </fieldset>
  );
}

/*
 * HearingFields are what the channel hears about and how.
 */
function HearingFields({
  workspace,
  hearing,
  set,
}: {
  workspace: string;
  hearing: DestinationHearing;
  set: (change: Partial<DestinationHearing>) => void;
}) {
  return (
    <>
      <fieldset className="flex flex-col gap-2">
        <Label>Post when</Label>
        {destinationTriggers.map((trigger) => (
          <Checkbox
            key={trigger.id}
            size="sm"
            isSelected={hearing.triggers.includes(trigger.id)}
            onChange={(on) => set({ triggers: toggled(hearing.triggers, trigger.id, on) })}
          >
            {trigger.label}
          </Checkbox>
        ))}
        <Checkbox
          size="sm"
          isSelected={hearing.include_ai_handled}
          onChange={(include_ai_handled) => set({ include_ai_handled })}
        >
          Include conversations the agent is answering
        </Checkbox>
      </fieldset>
      <fieldset className="flex flex-col gap-2">
        <Label>Digests</Label>
        {destinationDigests.map((digest) => (
          <Checkbox
            key={digest.id}
            size="sm"
            isSelected={hearing.digests.includes(digest.id)}
            onChange={(on) => set({ digests: toggled(hearing.digests, digest.id, on) })}
          >
            {`${digest.label}: ${digest.detail}`}
          </Checkbox>
        ))}
      </fieldset>
      {hearing.digests.length > 0 ? (
        <Select
          label="Send digests at"
          selectedKey={hearing.digest_time}
          onSelectionChange={(next) => next !== null && set({ digest_time: String(next) })}
        >
          {halfHours.map((time) => (
            <SelectItem key={time} id={time} textValue={time}>
              {time}
            </SelectItem>
          ))}
        </Select>
      ) : null}
      <InboxPicker workspace={workspace} value={hearing.inbox_ids} onChange={(inbox_ids) => set({ inbox_ids })} />
      <Checkbox size="sm" isSelected={hearing.show_previews} onChange={(show_previews) => set({ show_previews })}>
        Show the customer's message in the post
      </Checkbox>
    </>
  );
}

export function SlackDestinationModal({
  workspace,
  integration,
  destination,
  onClose,
}: {
  workspace: string;
  integration: string;
  destination: NotificationDestination | null;
  onClose: () => void;
}) {
  const formId = useId();
  const queryClient = useQueryClient();
  const [channel, setChannel] = useState<string | null>(null);
  const [hearing, setHearing] = useState<DestinationHearing>(() =>
    destination ? hearingOf(destination) : defaultHearing,
  );
  const set = (change: Partial<DestinationHearing>) => setHearing((current) => ({ ...current, ...change }));

  const save = useMutation({
    mutationFn: () => {
      if (!destination) {
        return createChannelDestination(workspace, integration, channel ?? "", hearing);
      }
      const { inbox_ids, ...rest } = hearing;

      return updateDestination(
        workspace,
        destination.id,
        inbox_ids ? { ...rest, inbox_ids } : { ...rest, all_inboxes: true },
      );
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: destinationsQuery(workspace).queryKey });
      onClose();
    },
  });

  const ready = destination !== null || channel !== null;

  return (
    <Modal size="md" isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={destination ? `#${destination.slack_channel_name ?? destination.name}` : "Post to a Slack channel"}
        description="The team hears about conversations here, and gets the digests you pick."
      />
      <ModalBody className="pb-4">
        <form
          id={formId}
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            if (ready) {
              save.mutate();
            }
          }}
        >
          {destination ? null : (
            <ChannelPicker workspace={workspace} integration={integration} value={channel} onChange={setChannel} />
          )}
          <HearingFields workspace={workspace} hearing={hearing} set={set} />
          {save.error ? <p className="text-body-2-regular text-text-error-primary">{save.error.message}</p> : null}
        </form>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button type="submit" form={formId} loading={save.isPending} disabled={!ready}>
          {destination ? "Save" : "Add channel"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
