/*
 * Settings → Tickets: the workspace's ticket types, which open their
 * editor, and its states, one card per group.
 */
import { SettingsPage } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";

import { StatesBoard } from "./states-board";
import { TypesSection } from "./types-section";

export function TicketsSettings() {
  const workspace = useAccount().workspace.id;

  return (
    <SettingsPage
      title="Tickets"
      description="Turn conversations that can't be settled in the chat into tracked work, with fields and a state the customer sees."
    >
      <TypesSection workspace={workspace} />
      <StatesBoard workspace={workspace} />
    </SettingsPage>
  );
}
