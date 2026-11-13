/*
 * Settings → Tickets: the workspace's ticket types, which open their
 * editor, and its states, one card per group.
 */
import { useBlocker } from "@tanstack/react-router";

import { SettingsPage } from "@/features/settings/layout";
import { SaveBar } from "@/features/settings/save-bar";
import { useAccount } from "@/features/settings/use-account";

import { StatesBoard, useNotifyDraft } from "./states-board";
import { TypesSection } from "./types-section";

export function TicketsSettings() {
  const workspace = useAccount().workspace.id;
  const draft = useNotifyDraft(workspace);
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  return (
    <SettingsPage
      title="Tickets"
      description="Turn conversations that can't be settled in the chat into tracked work, with fields and a state the customer sees."
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error?.message}
          onSave={() => draft.save.mutate()}
          onDiscard={draft.discard}
        />
      }
    >
      <TypesSection workspace={workspace} />
      <StatesBoard workspace={workspace} draft={draft} />
    </SettingsPage>
  );
}
