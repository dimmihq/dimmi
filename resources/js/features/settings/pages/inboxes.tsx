/*
 * The workspace's inboxes, on the server: every live inbox, the default
 * one marked, and adding a new one. Each inbox opens on its own page for
 * its settings, its automatic assignment and deleting it. Adding is
 * offered only to people the server lets manage inboxes. Whether the
 * messenger names the teammate who read a customer's message is a
 * workspace setting, changed by those who may change settings.
 */
import { RiInbox2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { createInbox, inboxesQuery } from "@/lib/api/inboxes";
import { updateWorkspace, workspaceQuery } from "@/lib/api/workspace";

import { Action, Toggle } from "../controls";
import { assignmentModes, usePermission } from "../inboxes";
import { Glyph, Item, Row, Section, SettingsPage } from "../layout";
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
 * MessengerSection says whether the customer's "Seen" names the teammate
 * who read their message, saved as soon as it flips.
 */
function MessengerSection() {
  const workspace = useAccount().workspace.id;
  const queryClient = useQueryClient();
  const canChange = usePermission(workspace, "settings:write");
  const { data } = useQuery(workspaceQuery(workspace));
  const save = useMutation({
    mutationFn: (on: boolean) => updateWorkspace(workspace, { seen_receipt_shows_name: on }),
    onSuccess: (updated) => queryClient.setQueryData(workspaceQuery(workspace).queryKey, updated),
  });

  if (!data) {
    return <Section title="Messenger" loading rows={1} />;
  }

  return (
    <Section title="Messenger">
      <Row label="Name in seen receipts" description="Show customers which teammate read their message.">
        <Toggle
          label="Name in seen receipts"
          on={data.seen_receipt_shows_name}
          disabled={!canChange}
          onChange={(on) => save.mutate(on)}
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
  const newInbox = canManage ? <Action onPress={() => setCreating(true)}>New inbox</Action> : undefined;

  return (
    <SettingsPage
      title="Inboxes"
      description="Every conversation lands in one inbox, picked by the channel it came from."
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
      <MessengerSection />
      <NewInboxDialog isOpen={creating} onClose={() => setCreating(false)} />
    </SettingsPage>
  );
}
