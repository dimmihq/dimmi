/*
 * The channels that bring conversations into an inbox, each on a row of
 * its own: its messengers, its email addresses and its API channel, which
 * every inbox has. Someone who manages inboxes can add a messenger here
 * or send a channel's new conversations to another inbox; conversations
 * it already brought in stay where they are.
 */
import { RiChat3Line, RiMailLine, RiPlugLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { emailAddressesQuery, updateEmailAddress } from "@/lib/api/email";
import type { Inbox } from "@/lib/api/inboxes";
import { createWidget, moveWidget, widgetSettingsQuery } from "@/lib/api/widget-settings";

import { Glyph, Item, Section } from "../layout";
import { failureText } from "../teams";

function MovePick({
  label,
  inbox,
  inboxes,
  onMove,
}: {
  label: string;
  inbox: string;
  inboxes: Inbox[];
  onMove: (inbox: string) => void;
}) {
  if (inboxes.length < 2) {
    return null;
  }

  return (
    <Select
      aria-label={label}
      size="sm"
      selectedKey={inbox}
      onSelectionChange={(key) => key !== inbox && onMove(String(key))}
      popoverClassName="w-56"
    >
      {inboxes.map((i) => (
        <SelectItem key={i.id} id={i.id} textValue={i.name}>
          {i.name}
        </SelectItem>
      ))}
    </Select>
  );
}

export function AddMessenger({ workspace, inbox }: { workspace: string; inbox: Inbox }) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const create = useMutation({
    mutationFn: () => createWidget(workspace, name.trim(), inbox.id),
    onSuccess: async () => {
      setOpen(false);
      setName("");
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "widget-settings"] });
    },
  });

  return (
    <>
      <Button size="small" variant="secondary" onClick={() => setOpen(true)}>
        Add messenger
      </Button>
      <AlertDialog
        isOpen={open}
        onOpenChange={(next) => {
          setOpen(next);
          create.reset();
        }}
        title="Add a messenger"
        description={`Its conversations land in ${inbox.name}. Install it on a site or in an app with its own snippet.`}
        icon={false}
        variant="default"
        confirmLabel="Add messenger"
        pendingLabel="Adding…"
        isConfirmDisabled={name.trim() === ""}
        onConfirm={() => create.mutateAsync()}
      >
        <Input label="Name" placeholder="Mobile app" value={name} onChange={setName} autoFocus />
        {create.error ? (
          <p role="alert" className="text-body-2-regular text-text-error-primary">
            {failureText(create.error)}
          </p>
        ) : null}
      </AlertDialog>
    </>
  );
}

export function ChannelsSection({
  workspace,
  inbox,
  inboxes,
  canManage,
}: {
  workspace: string;
  inbox: Inbox;
  inboxes: Inbox[];
  canManage: boolean;
}) {
  const queryClient = useQueryClient();
  const { data: widgets = [], isPending: loadingWidgets } = useQuery(widgetSettingsQuery(workspace));
  const { data: addresses = [], isPending: loadingAddresses } = useQuery(emailAddressesQuery(workspace));
  const refresh = () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
  const widget = useMutation({
    mutationFn: ({ id, to }: { id: string; to: string }) => moveWidget(workspace, id, to),
    onSuccess: refresh,
  });
  const address = useMutation({
    mutationFn: ({ id, to }: { id: string; to: string }) => updateEmailAddress(workspace, id, { inbox_id: to }),
    onSuccess: refresh,
  });
  const here = (i: { inbox: { id: string } }) => i.inbox.id === inbox.id;
  const moving = canManage ? inboxes : [];

  return (
    <Section
      title="Channels"
      description="Where this inbox's conversations come from."
      action={canManage ? <AddMessenger workspace={workspace} inbox={inbox} /> : null}
      loading={loadingWidgets || loadingAddresses}
      rows={2}
    >
      {widgets.filter(here).map((w) => (
        <Item key={w.id} mark={<Glyph icon={RiChat3Line} />} title={w.name} detail={`Messenger · ${w.id}`}>
          <MovePick
            label={`Inbox of ${w.name}`}
            inbox={inbox.id}
            inboxes={moving}
            onMove={(to) => widget.mutate({ id: w.id, to })}
          />
        </Item>
      ))}
      {addresses.filter(here).map((a) => (
        <Item key={a.id} mark={<Glyph icon={RiMailLine} />} title={a.address} detail="Email">
          <MovePick
            label={`Inbox of ${a.address}`}
            inbox={inbox.id}
            inboxes={moving}
            onMove={(to) => address.mutate({ id: a.address, to })}
          />
        </Item>
      ))}
      <Item
        mark={<Glyph icon={RiPlugLine} />}
        title="API"
        detail="Conversations started in this inbox through the REST API."
      />
      {widget.error || address.error ? (
        <p role="alert" className="px-3 pb-3 text-body-2-regular text-text-error-primary">
          {failureText(widget.error ?? address.error)}
        </p>
      ) : null}
    </Section>
  );
}
