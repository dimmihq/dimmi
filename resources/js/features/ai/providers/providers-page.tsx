/*
 * The AI providers page: the model connections the agent
 * can write with. dimmi's own models come first when the install offers
 * them, then every key the workspace added, each with how its last check
 * went. Adding, testing, renaming and removing are for people the server
 * lets manage keys; everyone else reads the list.
 */
import { RiAddLine, RiCpuLine } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Section, SettingsPage } from "@/features/settings/layout";

import { useAgent } from "../use-agent";
import { AddProviderModal } from "./add-provider-modal";
import { BuiltInRow, KeyRow } from "./key-row";

/*
 * NoProviders fills the list before the first connection, with the way
 * to add one.
 */
function NoProviders({ canKeys, onAdd }: { canKeys: boolean; onAdd: () => void }) {
  return (
    <div className="flex flex-col items-center gap-3 py-10 pe-3 text-center">
      <span
        aria-hidden
        className="flex size-10 items-center justify-center rounded-full bg-background-primary-default shadow-card"
      >
        <RiCpuLine className="size-5 text-foreground-icon-secondary" />
      </span>
      <div className="flex max-w-sm flex-col gap-1">
        <p className="text-body-medium text-text-primary">No providers yet</p>
        <p className="text-body-2-regular text-text-secondary">
          Connect Anthropic, OpenAI or your own server so the agent has a model to answer with.
        </p>
      </div>
      {canKeys ? (
        <Button size="small" leadingIcon={RiAddLine} onClick={onAdd}>
          Add provider
        </Button>
      ) : (
        <p className="text-body-2-regular text-text-tertiary">Ask an admin to connect one.</p>
      )}
    </div>
  );
}

export function ProvidersSettings() {
  const { workspace, view, canKeys } = useAgent();
  const [adding, setAdding] = useState(false);
  const keys = view.data?.keys ?? [];
  const builtIn = view.data?.managed ? keys.find((key) => key.provider === "dimmi") : undefined;
  const own = keys.filter((key) => key.provider !== "dimmi");
  const empty = view.data !== undefined && builtIn === undefined && own.length === 0;

  return (
    <SettingsPage
      title="AI providers"
      description="The models the agent answers with. Your own keys are billed by the provider, at their prices."
    >
      <Section
        title="Connections"
        description={canKeys ? undefined : "Only admins can add or change connections."}
        action={
          canKeys && !empty ? (
            <Button size="small" variant="secondary" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
              Add provider
            </Button>
          ) : undefined
        }
        loading={!view.data}
        rows={2}
      >
        {empty ? <NoProviders canKeys={canKeys} onAdd={() => setAdding(true)} /> : null}
        {builtIn ? <BuiltInRow connection={builtIn} /> : null}
        {own.map((connection) => (
          <KeyRow key={connection.id} workspace={workspace} connection={connection} manage={canKeys} />
        ))}
      </Section>
      <AddProviderModal
        workspace={workspace}
        providers={view.data?.providers ?? []}
        isOpen={adding}
        onClose={() => setAdding(false)}
      />
    </SettingsPage>
  );
}
