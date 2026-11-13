/*
 * The workspace's inboxes, on the server: every live inbox, the default
 * one marked, and adding a new one. Each inbox opens on its own page for
 * its settings, its automatic assignment and deleting it. Adding is
 * offered only to people the server lets manage inboxes. Whether the
 * messenger names the teammate who read a customer's message is a
 * workspace setting, changed by those who may change settings and saved
 * from the page's save bar.
 */
import { RiInbox2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { createInbox, inboxesQuery } from "@/lib/api/inboxes";
import { updateWorkspace, workspaceQuery } from "@/lib/api/workspace";

import { Action, Toggle } from "../controls";
import { assignmentModes, usePermission } from "../inboxes";
import { Glyph, Item, Row, Section, SettingsPage } from "../layout";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

/*
 * NewInboxDialog asks for the name of a new inbox and opens its page once
 * it exists.
 */
function NewInboxDialog({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [name, setName] = useState("");

  const create = useMutation({
    mutationFn: () => createInbox(workspace, name),
    onSuccess: async (inbox) => {
      setName("");
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "inboxes"] });
      await navigate({ to: "/d/settings/inboxes/$inbox", params: { inbox: inbox.id } });
    },
  });

  return (
    <AlertDialog
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) {
          create.reset();
          onClose();
        }
      }}
      variant="default"
      icon={false}
      title="New inbox"
      description="Conversations land in an inbox, picked by the channel they came from."
      confirmLabel="Create inbox"
      isConfirmDisabled={name.trim() === ""}
      onConfirm={() => create.mutateAsync()}
    >
      <Input
        label="Inbox name"
        value={name}
        onChange={setName}
        isInvalid={create.error !== null}
        errorMessage={failureText(create.error)}
      />
    </AlertDialog>
  );
}

/*
 * Receipts are the messenger's read receipt settings.
 */
type Receipts = { seen_receipts?: boolean; seen_receipt_shows_name?: boolean };

/*
 * useReceiptsDraft holds the read receipt switches flipped but not saved
 * yet. Save sends them in one change; Discard puts them back.
 */
function useReceiptsDraft(workspace: string) {
  const queryClient = useQueryClient();
  const { data } = useQuery(workspaceQuery(workspace));
  const [pending, setPending] = useState<Receipts>({});
  const save = useMutation({
    mutationFn: () => updateWorkspace(workspace, pending),
    onSuccess: (updated) => {
      queryClient.setQueryData(workspaceQuery(workspace).queryKey, updated);
      setPending({});
      toast.success("Messenger settings saved");
    },
  });
  const set = (key: keyof Receipts, on: boolean) =>
    setPending(({ [key]: _, ...rest }) => (data && data[key] === on ? rest : { ...rest, [key]: on }));

  return {
    data: data ? { ...data, ...pending } : undefined,
    dirty: Object.keys(pending).length > 0,
    save,
    set,
    discard: () => setPending({}),
  };
}

type ReceiptsDraft = ReturnType<typeof useReceiptsDraft>;

/*
 * MessengerSection holds whether customers see that the team read their
 * message, and whether it names the teammate who read it. Flips wait
 * for the page's save bar.
 */
function MessengerSection({ draft }: { draft: ReceiptsDraft }) {
  const workspace = useAccount().workspace.id;
  const canChange = usePermission(workspace, "settings:write");
  const data = draft.data;

  if (!data) {
    return <Section title="Messenger" loading rows={1} />;
  }

  return (
    <Section title="Messenger">
      <Row label="Show read receipts to customers" description="Customers see Seen once the team read their message.">
        <Toggle
          key={`receipts-${data.seen_receipts}`}
          label="Show read receipts to customers"
          on={data.seen_receipts}
          disabled={!canChange}
          onChange={(on) => draft.set("seen_receipts", on)}
        />
      </Row>
      <Row label="Name in seen receipts" description="Show customers which teammate read their message.">
        <Toggle
          key={`name-${data.seen_receipt_shows_name}`}
          label="Name in seen receipts"
          on={data.seen_receipt_shows_name}
          disabled={!canChange || !data.seen_receipts}
          onChange={(on) => draft.set("seen_receipt_shows_name", on)}
        />
      </Row>
    </Section>
  );
}

export function InboxesSettings() {
  const workspace = useAccount().workspace.id;
  const navigate = useNavigate();
  const { data: inboxes } = useQuery(inboxesQuery(workspace));
  const canManage = usePermission(workspace, "inboxes:manage");
  const [creating, setCreating] = useState(false);
  const receipts = useReceiptsDraft(workspace);
  useBlocker({
    shouldBlockFn: () => receipts.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => receipts.dirty,
  });
  const newInbox = canManage ? <Action onPress={() => setCreating(true)}>New inbox</Action> : undefined;

  return (
    <SettingsPage
      title="Inboxes"
      description="Every conversation lands in one inbox, picked by the channel it came from."
      footer={
        <SaveBar
          dirty={receipts.dirty}
          saving={receipts.save.isPending}
          error={receipts.save.error ? failureText(receipts.save.error) : null}
          onSave={() => receipts.save.mutate()}
          onDiscard={receipts.discard}
        />
      }
    >
      <Section title="Inboxes" action={newInbox} loading={!inboxes} rows={2}>
        {inboxes?.map((inbox) => (
          <Item
            key={inbox.id}
            mark={<Glyph icon={RiInbox2Line} />}
            title={inbox.name}
            detail={`${assignmentModes[inbox.settings.assignment.mode]} assignment`}
          >
            {inbox.default ? (
              <Chip variant="caption" color="neutral">
                Default
              </Chip>
            ) : null}
            <Action
              size="xs"
              onPress={() => navigate({ to: "/d/settings/inboxes/$inbox", params: { inbox: inbox.id } })}
            >
              {canManage ? "Manage" : "View"}
            </Action>
          </Item>
        ))}
      </Section>
      <MessengerSection draft={receipts} />
      <NewInboxDialog isOpen={creating} onClose={() => setCreating(false)} />
    </SettingsPage>
  );
}
