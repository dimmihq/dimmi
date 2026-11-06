/*
 * One connection on the providers page: its provider's mark, its name,
 * the last four of its key, how many models it offers and how its last
 * check went. Its menu tests it again, renames it or removes it, and
 * removing asks first, since the agent stops answering if it was
 * writing with that key. dimmi's built-in models show as a row of their
 * own that needs no key and has nothing to manage.
 */
import { RiMore2Line } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";
import { ago } from "@/features/inbox/conversations";
import { Item } from "@/features/settings/layout";
import { type AiProviderKey, aiKey, deleteAiKey, testAiKey, updateAiKey } from "@/lib/api/ai";

import { type KeyStatus, keyStatus, maskedHint, modelCount, statusLabels } from "../format";
import { providerName } from "../provider-info";
import { ProviderMark } from "../provider-mark";
import { refusal } from "../use-agent";

const chipColors: Record<KeyStatus, "lime" | "rose" | "neutral"> = {
  ok: "lime",
  error: "rose",
  untested: "neutral",
};

/*
 * StatusChip says how the connection's last check went, with when it
 * ran and, for a failure, the provider's reason on hover.
 */
function StatusChip({ connection }: { connection: AiProviderKey }) {
  const status = keyStatus(connection);
  const checked = connection.last_validated_at ? `Checked ${ago(connection.last_validated_at).toLowerCase()}` : null;
  const reason = status === "error" ? connection.status_reason : null;
  const chip = (
    <Chip variant="caption" color={chipColors[status]}>
      {statusLabels[status]}
    </Chip>
  );

  if (!reason && !checked) {
    return chip;
  }

  return (
    <TooltipTrigger>
      <button
        type="button"
        className="rounded-md outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        {chip}
      </button>
      <Tooltip>{[reason, checked].filter(Boolean).join(" · ")}</Tooltip>
    </TooltipTrigger>
  );
}

function detailOf(connection: AiProviderKey): string {
  const parts = [providerName(connection.provider)];
  const hint = maskedHint(connection.key_hint);
  if (hint) {
    parts.push(hint);
  }
  if (connection.base_url) {
    parts.push(connection.base_url.replace(/^https?:\/\//, ""));
  }
  parts.push(modelCount(connection.models));

  return parts.join(" · ");
}

/*
 * BuiltInRow is dimmi's own models, ready without a key.
 */
export function BuiltInRow({ connection }: { connection: AiProviderKey }) {
  return (
    <Item
      mark={<ProviderMark provider="dimmi" />}
      title="dimmi"
      detail={`Included with dimmi · no key needed · ${modelCount(connection.models)}`}
    >
      <Chip variant="caption" color="lime">
        Ready
      </Chip>
    </Item>
  );
}

function useRefresh(workspace: string) {
  const client = useQueryClient();

  return () => client.invalidateQueries({ queryKey: aiKey(workspace) });
}

function RenameDialog({
  workspace,
  connection,
  onClose,
}: {
  workspace: string;
  connection: AiProviderKey;
  onClose: () => void;
}) {
  const refresh = useRefresh(workspace);
  const [name, setName] = useState(connection.name);
  const rename = useMutation({
    mutationFn: () => updateAiKey(workspace, connection.id, { name: name.trim() }),
    onSuccess: async () => {
      toast.success("Renamed");
      await refresh();
    },
  });

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      variant="default"
      icon={false}
      title="Rename connection"
      description="The name shows in the model picker and in traces."
      confirmLabel="Save"
      isConfirmDisabled={name.trim() === "" || name.trim() === connection.name}
      onConfirm={() => rename.mutateAsync()}
    >
      <Input
        label="Name"
        value={name}
        onChange={setName}
        autoFocus
        isInvalid={rename.error !== null}
        errorMessage={rename.error ? refusal(rename.error, "Not saved.") : undefined}
      />
    </AlertDialog>
  );
}

function RemoveDialog({
  workspace,
  connection,
  onClose,
}: {
  workspace: string;
  connection: AiProviderKey;
  onClose: () => void;
}) {
  const refresh = useRefresh(workspace);
  const remove = useMutation({
    mutationFn: () => deleteAiKey(workspace, connection.id),
    onSuccess: async () => {
      toast.success(`${connection.name} removed`);
      await refresh();
    },
    onError: (error) => toast.error("Not removed", { description: refusal(error, "Try again.") }),
  });

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={`Remove ${connection.name}?`}
      description="The key is deleted from dimmi. If the agent answers with one of its models, choose another model first."
      confirmLabel="Remove"
      pendingLabel="Removing…"
      onConfirm={() => remove.mutateAsync()}
    />
  );
}

/*
 * KeyRow is one connection the workspace added. manage shows its menu.
 */
export function KeyRow({
  workspace,
  connection,
  manage,
}: {
  workspace: string;
  connection: AiProviderKey;
  manage: boolean;
}) {
  const refresh = useRefresh(workspace);
  const [menu, setMenu] = useState(false);
  const [dialog, setDialog] = useState<"rename" | "remove" | null>(null);
  const label = `${connection.name} actions`;
  const test = useMutation({
    mutationFn: () => testAiKey(workspace, connection.id),
    onSuccess: async (checked) => {
      const tested = checked as Partial<AiProviderKey>;
      if (keyStatus({ status: tested.status ?? "", provider: connection.provider }) === "ok") {
        toast.success("Connection works", { description: `${connection.name} answered the check.` });
      } else {
        toast.error("Connection failed", {
          description: tested.status_reason ?? `${providerName(connection.provider)} refused the check.`,
        });
      }
      await refresh();
    },
    onError: (error) => toast.error("Connection failed", { description: refusal(error, "Try again.") }),
  });
  const pick = (action: () => void) => () => {
    setMenu(false);
    action();
  };

  return (
    <Item mark={<ProviderMark provider={connection.provider} />} title={connection.name} detail={detailOf(connection)}>
      {test.isPending ? (
        <Chip variant="caption" color="neutral">
          Testing…
        </Chip>
      ) : (
        <StatusChip connection={connection} />
      )}
      {manage ? (
        <Dropdown isOpen={menu} onOpenChange={setMenu}>
          <DropdownTrigger
            aria-label={label}
            className="flex size-8 items-center justify-center rounded-lg hover:bg-background-secondary-hover"
          >
            <RiMore2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
          </DropdownTrigger>
          <DropdownPopover aria-label={label} placement="bottom end" className="w-48">
            <DropdownItem onSelect={pick(() => test.mutate())}>
              <span className="text-body-2-regular text-text-primary">Test connection</span>
            </DropdownItem>
            <DropdownItem onSelect={pick(() => setDialog("rename"))}>
              <span className="text-body-2-regular text-text-primary">Rename</span>
            </DropdownItem>
            <DropdownItem onSelect={pick(() => setDialog("remove"))}>
              <span className="text-body-2-regular text-text-error-primary">Remove</span>
            </DropdownItem>
          </DropdownPopover>
        </Dropdown>
      ) : null}
      {dialog === "rename" ? (
        <RenameDialog workspace={workspace} connection={connection} onClose={() => setDialog(null)} />
      ) : null}
      {dialog === "remove" ? (
        <RemoveDialog workspace={workspace} connection={connection} onClose={() => setDialog(null)} />
      ) : null}
    </Item>
  );
}
