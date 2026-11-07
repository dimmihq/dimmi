/*
 * One connection on the actions page and the actions that call through
 * it. Its head carries the connection's mark, name, host and how its
 * last test went; its menu tests it again, syncs an MCP server's tools,
 * changes or removes it. Each action is a row that opens its editor,
 * saying what it does to data, who must agree before it runs and
 * whether it is live. Removing asks first, since every action of the
 * connection stops working.
 */
import { RiAddLine, RiArrowRightSLine, RiMore2Line, RiRefreshLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";
import { cx } from "@/utils/cx";

import { refusal } from "../use-agent";
import {
  type ActionConnection,
  type AiAction,
  actionsKey,
  deleteConnection,
  syncTools,
  testConnection,
  updateConnection,
} from "./api";
import { kindInfo } from "./connection-values";
import { authLabels, connectionLook, hostOf, permissionSummary, syncLine, testReason } from "./format";
import { ActionStateChip, ConnectionMark, SideEffectChip } from "./parts";

function ActionRow({ action }: { action: AiAction }) {
  return (
    <Link
      to="/d/settings/actions/$action"
      params={{ action: action.id }}
      className={cx(
        "flex min-h-[56px] items-center gap-3 border-t border-separator-border py-2.5 ps-3 pe-2.5 outline-none",
        "transition-colors duration-150 last:rounded-b-2xl hover:bg-background-secondary-hover",
        "focus-visible:ring-2 focus-visible:ring-border-focus-ring focus-visible:ring-inset",
      )}
    >
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-regular text-text-primary">{action.title || action.name}</span>
        <span className="truncate text-body-2-regular text-text-secondary">
          <span className="font-mono" dir="ltr">
            {action.name}
          </span>
          {` · ${permissionSummary(action)}`}
        </span>
      </span>
      <span className="hidden shrink-0 items-center gap-1.5 sm:flex">
        <SideEffectChip side={action.side_effect} />
        <ActionStateChip action={action} />
      </span>
      <RiArrowRightSLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary rtl:-scale-x-100" />
    </Link>
  );
}

/*
 * useConnectionMoves are what a connection's menu does: test it, sync an
 * MCP server's tools, turn it off or on, and remove it.
 */
function useConnectionMoves(workspace: string, connection: ActionConnection) {
  const client = useQueryClient();
  const refresh = () => client.invalidateQueries({ queryKey: actionsKey(workspace) });
  const test = useMutation({
    mutationFn: () => testConnection(workspace, connection.id),
    onSuccess: async (result) => {
      if (result.status === "ok") {
        const tools = typeof result.tools_count === "number" ? ` · ${result.tools_count} tools` : "";
        toast.success("Connection works", { description: `${connection.name} answered${tools}.` });
      } else {
        toast.error("Connection failed", { description: testReason(result.status_reason) });
      }
      await refresh();
    },
    onError: (error) => toast.error("Connection failed", { description: refusal(error, "Try again.") }),
  });
  const sync = useMutation({
    mutationFn: () => syncTools(workspace, connection.id),
    onSuccess: async (result) => {
      toast.success("Tools synced", { description: syncLine(result) });
      await refresh();
    },
    onError: (error) => toast.error("Tools not synced", { description: refusal(error, "Try again.") }),
  });
  const toggle = useMutation({
    mutationFn: () =>
      updateConnection(workspace, connection.id, {
        status: connection.status === "disabled" ? "active" : "disabled",
      }),
    onSuccess: refresh,
    onError: (error) => toast.error("Not changed", { description: refusal(error, "Try again.") }),
  });
  const remove = async (force: boolean): Promise<"removed" | "in_use"> => {
    try {
      await deleteConnection(workspace, connection.id, force);
    } catch (error) {
      if (error instanceof ApiError && error.code === "connection_in_use") {
        return "in_use";
      }
      toast.error("Not removed", { description: refusal(error, "Try again.") });
      throw error;
    }
    toast.success(`${connection.name} removed`);
    await refresh();
    return "removed";
  };

  return { test, sync, toggle, remove };
}

function Detail({ connection }: { connection: ActionConnection }) {
  const parts = [kindInfo[connection.kind].name, authLabels[connection.auth_type]];
  if (connection.secret_hint) {
    parts.push(connection.secret_hint);
  }

  return (
    <span className="truncate text-body-2-regular text-text-secondary">
      <bdi dir="ltr">{hostOf(connection.base_url)}</bdi>
      {` · ${parts.join(" · ")}`}
    </span>
  );
}

type Moves = ReturnType<typeof useConnectionMoves>;

/*
 * ConnectionMenu is what can be done with the connection.
 */
function ConnectionMenu({
  connection,
  moves,
  onChange,
  onRemove,
}: {
  connection: ActionConnection;
  moves: Moves;
  onChange: () => void;
  onRemove: () => void;
}) {
  const [menu, setMenu] = useState(false);
  const label = `${connection.name} options`;
  const pick = (action: () => void) => () => {
    setMenu(false);
    action();
  };

  return (
    <Dropdown isOpen={menu} onOpenChange={setMenu}>
      <DropdownTrigger
        aria-label={label}
        className="flex size-8 items-center justify-center rounded-lg hover:bg-background-secondary-hover"
      >
        <RiMore2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
      </DropdownTrigger>
      <DropdownPopover aria-label={label} placement="bottom end" className="w-52">
        <DropdownItem onSelect={pick(() => moves.test.mutate())}>
          <span className="text-body-2-regular text-text-primary">Test connection</span>
        </DropdownItem>
        {connection.kind === "mcp" ? (
          <DropdownItem onSelect={pick(() => moves.sync.mutate())}>
            <span className="text-body-2-regular text-text-primary">Sync tools</span>
          </DropdownItem>
        ) : null}
        <DropdownItem onSelect={pick(onChange)}>
          <span className="text-body-2-regular text-text-primary">Change details</span>
        </DropdownItem>
        <DropdownItem onSelect={pick(() => moves.toggle.mutate())}>
          <span className="text-body-2-regular text-text-primary">
            {connection.status === "disabled" ? "Turn on" : "Turn off"}
          </span>
        </DropdownItem>
        <DropdownItem onSelect={pick(onRemove)}>
          <span className="text-body-2-regular text-text-error-primary">Remove</span>
        </DropdownItem>
      </DropdownPopover>
    </Dropdown>
  );
}

/*
 * Footer adds to the connection: a new action for a REST API, the
 * server's tools again for an MCP server.
 */
function Footer({ connection, moves }: { connection: ActionConnection; moves: Moves }) {
  const navigate = useNavigate();

  return (
    <footer className="flex flex-wrap gap-2 border-t border-separator-border p-2.5">
      {connection.kind === "mcp" ? (
        <Button
          variant="secondary"
          size="xs"
          leadingIcon={RiRefreshLine}
          loading={moves.sync.isPending}
          onClick={() => moves.sync.mutate()}
        >
          Sync tools
        </Button>
      ) : (
        <Button
          variant="secondary"
          size="xs"
          leadingIcon={RiAddLine}
          onClick={() =>
            navigate({
              to: "/d/settings/actions/$action",
              params: { action: "new" },
              search: { connection: connection.id },
            })
          }
        >
          New action
        </Button>
      )}
    </footer>
  );
}

function removeText(count: number): string {
  if (count === 0) {
    return "dimmi forgets its address and secret.";
  }
  const stop = count === 1 ? "action stops" : `${count} actions stop`;

  return `Its ${stop} working right away. dimmi forgets its address and secret.`;
}

/*
 * ConnectionCard is the connection's card with its actions.
 */
export function ConnectionCard({
  workspace,
  connection,
  actions,
  manage,
  onChange,
}: {
  workspace: string;
  connection: ActionConnection;
  actions: AiAction[];
  manage: boolean;
  onChange: () => void;
}) {
  const [removing, setRemoving] = useState(false);
  const [inUse, setInUse] = useState(false);
  const moves = useConnectionMoves(workspace, connection);
  const look = connectionLook(connection);
  const testing = moves.test.isPending;

  return (
    <section aria-label={connection.name} className="flex flex-col rounded-2xl bg-background-secondary-default">
      <header className="flex min-h-[64px] items-center gap-3 py-3 ps-3 pe-2.5">
        <ConnectionMark kind={connection.kind} />
        <div className="flex min-w-0 flex-1 flex-col">
          <h3 className="truncate text-body-medium text-text-primary">{connection.name}</h3>
          <Detail connection={connection} />
        </div>
        <Chip variant="caption" color={testing ? "neutral" : look.tone} title={connection.last_error ?? ""}>
          {testing ? "Testing…" : look.label}
        </Chip>
        {manage ? (
          <ConnectionMenu
            connection={connection}
            moves={moves}
            onChange={onChange}
            onRemove={() => setRemoving(true)}
          />
        ) : null}
      </header>
      {actions.map((action) => (
        <ActionRow key={action.id} action={action} />
      ))}
      {actions.length === 0 ? (
        <p className="border-t border-separator-border py-3 ps-3 pe-3 text-body-2-regular text-text-tertiary">
          {connection.kind === "mcp"
            ? "No tools yet. Sync the server's tools to turn them into draft actions."
            : "No actions yet. Add the first one, such as looking up a customer's subscription."}
        </p>
      ) : null}
      {manage ? <Footer connection={connection} moves={moves} /> : null}
      <AlertDialog
        isOpen={removing}
        onOpenChange={setRemoving}
        title={`Remove ${connection.name}?`}
        description={removeText(actions.length)}
        confirmLabel="Remove"
        pendingLabel="Removing…"
        onConfirm={async () => {
          if ((await moves.remove(false)) === "in_use") {
            setInUse(true);
          }
        }}
      />
      <AlertDialog
        isOpen={inUse}
        onOpenChange={setInUse}
        title={`Remove ${connection.name} and its actions?`}
        description="Actions still call through this connection. Removing it deletes them too and cancels their waiting calls."
        confirmLabel="Remove all"
        pendingLabel="Removing…"
        onConfirm={() => moves.remove(true)}
      />
    </section>
  );
}
