/*
 * The actions page: the workspace's own systems dimmi's agent may call,
 * each connection with the actions it serves. Before the first
 * connection, the page explains what actions are, with an example of
 * each kind and two clear ways to start, a REST API or an MCP server.
 * Adding and changing connections and actions is for people the server
 * lets manage actions; everyone else reads the list.
 */
import { RiAddLine, RiArrowRightSLine, RiFlashlightLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { usePermission } from "@/features/settings/inboxes";
import { RowsSkeleton, SettingsPage } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";

import { AddConnectionModal } from "./add-connection-modal";
import { type ActionConnection, actionsQuery, CONNECTION_KINDS, type ConnectionKind, connectionsQuery } from "./api";
import { ConnectionCard } from "./connection-card";
import { kindInfo } from "./connection-values";
import { ConnectionMark } from "./parts";

const examples = [
  { title: "Check a subscription", detail: "Reads the plan and renewal date", side: "Reads" },
  { title: "Resend an invoice", detail: "Customer confirms first", side: "Changes data" },
  { title: "Refund a payment", detail: "Customer confirms · Teammate approves", side: "Changes data" },
];

function Start({ kind, onPick }: { kind: ConnectionKind; onPick: () => void }) {
  const info = kindInfo[kind];

  return (
    <button
      type="button"
      onClick={onPick}
      className="flex w-full cursor-pointer items-center gap-3 rounded-xl bg-background-primary-default p-3 text-start shadow-card outline-none transition-colors duration-150 hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <ConnectionMark kind={kind} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="text-body-medium text-text-primary">
          Connect {kind === "mcp" ? "an" : "a"} {info.name}
        </span>
        <span className="text-body-2-regular text-text-secondary">{info.blurb}</span>
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </button>
  );
}

/*
 * NoConnections explains actions before the first connection.
 */
function NoConnections({ manage, onStart }: { manage: boolean; onStart: (kind: ConnectionKind) => void }) {
  return (
    <section
      aria-label="Get started"
      className="flex flex-col gap-5 rounded-2xl bg-background-secondary-default p-4 sm:p-5"
    >
      <div className="flex flex-col items-start gap-3">
        <span
          aria-hidden
          className="flex size-10 items-center justify-center rounded-full bg-background-primary-default shadow-card"
        >
          <RiFlashlightLine className="size-5 text-foreground-icon-secondary" />
        </span>
        <div className="flex flex-col gap-1">
          <h2 className="text-headline-medium text-text-primary">Let dimmi act in your systems</h2>
          <p className="max-w-lg text-body-regular text-text-secondary">
            Connect an API you already run and describe what dimmi may do with it. dimmi calls it mid-conversation, only
            for the signed-in customer, and asks before anything changes.
          </p>
        </div>
      </div>
      <ul aria-label="Examples" className="flex flex-col rounded-xl ps-3 ring-1 ring-border-button-default ring-inset">
        {examples.map((example) => (
          <li
            key={example.title}
            className="flex items-center justify-between gap-3 border-b border-separator-border py-2.5 pe-3 last:border-b-0"
          >
            <span className="flex min-w-0 flex-col">
              <span className="truncate text-body-regular text-text-primary">{example.title}</span>
              <span className="truncate text-body-2-regular text-text-secondary">{example.detail}</span>
            </span>
            <span className="shrink-0 text-body-2-regular text-text-tertiary">{example.side}</span>
          </li>
        ))}
      </ul>
      {manage ? (
        <div className="grid gap-2 sm:grid-cols-2">
          {CONNECTION_KINDS.map((kind) => (
            <Start key={kind} kind={kind} onPick={() => onStart(kind)} />
          ))}
        </div>
      ) : (
        <p className="text-body-2-regular text-text-tertiary">Ask an admin to connect your first system.</p>
      )}
    </section>
  );
}

type Dialog = { start: ConnectionKind | null; connection?: ActionConnection } | null;

export function ActionsSettings() {
  const workspace = useAccount().workspace.id;
  const ready = workspace !== "";
  const connections = useQuery({ ...connectionsQuery(workspace), enabled: ready });
  const actions = useQuery({ ...actionsQuery(workspace), enabled: ready });
  const manage = usePermission(workspace, "ai_actions:manage");
  const [dialog, setDialog] = useState<Dialog>(null);
  const [last, setLast] = useState<Dialog>(null);
  if (dialog !== null && dialog !== last) {
    setLast(dialog);
  }
  const shown = dialog ?? last;
  const loading = connections.isPending || actions.isPending;
  const list = connections.data ?? [];
  const failed = connections.isError && !connections.data;

  return (
    <SettingsPage
      title="Actions"
      description="Let dimmi look things up and make changes in your own systems, with the permissions you choose."
    >
      <section aria-label="Connections" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-end justify-between gap-x-4 gap-y-2 px-3">
          <div className="flex min-w-0 flex-col gap-0.5">
            <h2 className="text-body-2-medium text-text-secondary">Connections</h2>
            {manage ? null : (
              <p className="text-body-2-regular text-text-tertiary">Only admins can add or change actions.</p>
            )}
          </div>
          {manage && list.length > 0 ? (
            <Button size="small" variant="secondary" leadingIcon={RiAddLine} onClick={() => setDialog({ start: null })}>
              Add connection
            </Button>
          ) : null}
        </div>
        {loading && !failed ? (
          <div aria-busy className="flex flex-col rounded-2xl bg-background-secondary-default ps-3">
            <RowsSkeleton rows={3} />
          </div>
        ) : null}
        {failed ? (
          <p className="rounded-2xl bg-background-secondary-default p-4 text-body-2-regular text-text-secondary">
            Actions could not be loaded. Reload the page to try again.
          </p>
        ) : null}
        {!loading && !failed && list.length === 0 ? (
          <NoConnections manage={manage} onStart={(start) => setDialog({ start })} />
        ) : null}
        {list.map((connection) => (
          <ConnectionCard
            key={connection.id}
            workspace={workspace}
            connection={connection}
            actions={(actions.data ?? []).filter((action) => action.connection_id === connection.id)}
            manage={manage}
            onChange={() => setDialog({ start: connection.kind, connection })}
          />
        ))}
      </section>
      <AddConnectionModal
        workspace={workspace}
        isOpen={dialog !== null}
        start={shown?.start ?? null}
        connection={shown?.connection}
        onClose={() => setDialog(null)}
      />
    </SettingsPage>
  );
}
