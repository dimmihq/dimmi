/*
 * The channels that bring conversations into an inbox, each on a row of
 * its own: its messengers, its email addresses and its API channel, which
 * every inbox has. Someone who manages inboxes can add a messenger here
 * at once, or pick another inbox for a channel's new conversations,
 * which moves it when the inbox page is saved; conversations it already
 * brought in stay where they are.
 */
import { RiChat3Line, RiMailLine, RiPlugLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { emailAddressesQuery } from "@/lib/api/email";
import type { Inbox } from "@/lib/api/inboxes";
import { createWidget, widgetSettingsQuery } from "@/lib/api/widget-settings";

import { Glyph, Item, Section } from "../layout";
import { failureText } from "../teams";
import type { ChannelsDraft } from "./channels-draft";

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

/*
 * ChannelsSection lists the inbox's channels. A channel picked to go to
 * another inbox stays listed, showing where it goes, until the page is
 * saved.
 */
export function ChannelsSection({
  workspace,
  inbox,
  inboxes,
  canManage,
  draft,
}: {
  workspace: string;
  inbox: Inbox;
  inboxes: Inbox[];
  canManage: boolean;
  draft: ChannelsDraft;
}) {
  const { data: widgets = [], isPending: loadingWidgets } = useQuery(widgetSettingsQuery(workspace));
  const { data: addresses = [], isPending: loadingAddresses } = useQuery(emailAddressesQuery(workspace));
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
            inbox={draft.widgets[w.id] ?? inbox.id}
            inboxes={moving}
            onMove={(to) => draft.moveWidget(w.id, inbox.id, to)}
          />
        </Item>
      ))}
      {addresses.filter(here).map((a) => (
        <Item key={a.id} mark={<Glyph icon={RiMailLine} />} title={a.address} detail="Email">
          <MovePick
            label={`Inbox of ${a.address}`}
            inbox={draft.addresses[a.address] ?? inbox.id}
            inboxes={moving}
            onMove={(to) => draft.moveAddress(a.address, inbox.id, to)}
          />
        </Item>
      ))}
      <Item
        mark={<Glyph icon={RiPlugLine} />}
        title="API"
        detail="Conversations started in this inbox through the REST API."
      />
    </Section>
  );
}
