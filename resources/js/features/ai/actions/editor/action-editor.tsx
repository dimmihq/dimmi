/*
 * The action editor, a page of its own. On the left the action as dimmi
 * reads it: its basics, the request it sends (or the MCP tool it runs),
 * what the agent fills in, what of the answer it keeps, who must agree
 * before it runs and where. On the right a test panel stays beside it on
 * wide screens and opens as a drawer on narrow ones. The header says
 * whether customers get a live version and whether the draft has
 * changes they do not have yet; Save keeps the draft, Publish makes it
 * live as a new version, and the versions drawer restores an old one.
 * People who may not manage actions read it without changing it.
 */
import { RiArrowLeftSLine, RiFlaskLine, RiHistoryLine, RiMore2Line } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Drawer } from "@/components/base/drawer/drawer";
import { Dropdown, DropdownItem, DropdownPopover, DropdownTrigger } from "@/components/base/dropdown/dropdown";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import { PageSkeleton } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel, ScrollArea } from "@/features/shell/page";
import { useWide } from "@/features/shell/use-wide";

import { refusal } from "../../use-agent";
import {
  type ActionConnection,
  type ActionTestResult,
  type AiAction,
  actionQuery,
  actionsKey,
  connectionsQuery,
  deleteAction,
  setActionStatus,
} from "../api";
import { ActionStateChip, ConnectionMark } from "../parts";
import { BasicsSection } from "./basics-section";
import { InputsSection } from "./inputs-section";
import { ChannelsSection, PermissionsSection } from "./permissions-section";
import { RequestSection } from "./request-section";
import { ResponseSection } from "./response-section";
import { TestPanel } from "./test-panel";
import { ToolSection } from "./tool-section";
import { type Editor, useEditor } from "./use-editor";
import { VersionsDrawer } from "./versions-drawer";

const backLink =
  "flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring";

function Back() {
  return (
    <Link to="/d/settings/$section" params={{ section: "actions" }} aria-label="Back to actions" className={backLink}>
      <RiArrowLeftSLine aria-hidden className="size-5 rtl:-scale-x-100" />
    </Link>
  );
}

/*
 * MoreMenu turns a published action off or on again, and deletes it.
 */
function MoreMenu({ workspace, editor }: { workspace: string; editor: Editor }) {
  const client = useQueryClient();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const saved = editor.saved;
  if (!saved) {
    return null;
  }
  const off = saved.status === "disabled";
  const toggle = async () => {
    setOpen(false);
    try {
      editor.setSaved(await setActionStatus(workspace, saved.id, off ? "published" : "disabled"));
      toast.success(off ? "Action turned on" : "Action turned off", {
        description: off ? undefined : "dimmi stops using it from the next message.",
      });
      await client.invalidateQueries({ queryKey: actionsKey(workspace) });
    } catch (error) {
      toast.error("Not changed", { description: refusal(error, "Try again.") });
    }
  };
  const remove = async () => {
    await deleteAction(workspace, saved.id);
    toast.success(`${saved.title || saved.name} deleted`);
    await client.invalidateQueries({ queryKey: actionsKey(workspace) });
    await navigate({ to: "/d/settings/$section", params: { section: "actions" } });
  };

  return (
    <>
      <Dropdown isOpen={open} onOpenChange={setOpen}>
        <DropdownTrigger
          aria-label="More"
          className="flex size-8 items-center justify-center rounded-lg hover:bg-background-secondary-hover"
        >
          <RiMore2Line aria-hidden className="size-4 text-foreground-icon-secondary" />
        </DropdownTrigger>
        <DropdownPopover aria-label="More" placement="bottom end" className="w-48">
          {saved.published_version ? (
            <DropdownItem onSelect={() => void toggle()}>
              <span className="text-body-2-regular text-text-primary">{off ? "Turn on" : "Turn off"}</span>
            </DropdownItem>
          ) : null}
          <DropdownItem
            onSelect={() => {
              setOpen(false);
              setDeleting(true);
            }}
          >
            <span className="text-body-2-regular text-text-error-primary">Delete action</span>
          </DropdownItem>
        </DropdownPopover>
      </Dropdown>
      <AlertDialog
        isOpen={deleting}
        onOpenChange={setDeleting}
        title={`Delete ${saved.title || saved.name}?`}
        description="dimmi stops using it right away. Its past calls stay in conversations."
        confirmLabel="Delete"
        pendingLabel="Deleting…"
        onConfirm={remove}
      />
    </>
  );
}

/*
 * SaveButtons keep the draft and make it live. Publish is offered while
 * customers do not have the draft yet.
 */
function SaveButtons({ editor }: { editor: Editor }) {
  const { saved, dirty, busy } = editor;
  const live = saved?.published_version && saved.status === "published";
  const pending = dirty || !saved || saved.has_unpublished_changes || !saved.published_version;

  return (
    <>
      <Button
        variant="secondary"
        size="small"
        loading={busy === "save"}
        disabled={!dirty || busy !== null}
        onClick={() => void editor.save()}
      >
        Save draft
      </Button>
      <Button
        size="small"
        loading={busy === "publish"}
        disabled={!pending || busy !== null}
        onClick={() => void editor.publish()}
      >
        {live ? <span className="hidden sm:inline">Publish changes</span> : null}
        <span className={live ? "sm:hidden" : undefined}>Publish</span>
      </Button>
    </>
  );
}

function Header({
  workspace,
  editor,
  manage,
  onTest,
  onVersions,
}: {
  workspace: string;
  editor: Editor;
  manage: boolean;
  onTest?: () => void;
  onVersions: () => void;
}) {
  const { saved, dirty } = editor;

  return (
    <header className="flex min-h-12 shrink-0 flex-wrap items-center justify-between gap-x-3 gap-y-2 ps-2.5 pe-2.5 pt-[7px] pb-1">
      <div className="flex min-w-0 items-center gap-2">
        <Back />
        <h1 className="min-w-0 truncate text-body-medium text-text-primary">
          {editor.draft.title || editor.draft.name || "New action"}
        </h1>
        {saved ? (
          <ActionStateChip action={saved} />
        ) : (
          <Chip variant="caption" color="neutral">
            New
          </Chip>
        )}
        {dirty && saved ? (
          <span className="hidden text-body-2-regular text-text-tertiary md:inline">Unsaved changes</span>
        ) : null}
      </div>
      <div className="ms-auto flex flex-wrap items-center justify-end gap-2">
        {onTest ? (
          <Button variant="secondary" size="small" leadingIcon={RiFlaskLine} onClick={onTest}>
            Test
          </Button>
        ) : null}
        {saved ? (
          <Button
            variant="secondary"
            size="small"
            leadingIcon={RiHistoryLine}
            onClick={onVersions}
            aria-label="Versions"
          >
            <span className="hidden sm:inline">Versions</span>
          </Button>
        ) : null}
        {manage ? <MoreMenu workspace={workspace} editor={editor} /> : null}
        {manage ? <SaveButtons editor={editor} /> : null}
      </div>
    </header>
  );
}

function Body({ editor, connection, sample }: { editor: Editor; connection: ActionConnection; sample: unknown }) {
  return (
    <div className="mx-auto flex w-full max-w-[680px] flex-col gap-8 px-4 pt-6 pb-16 sm:px-6">
      <div className="flex items-center gap-2.5 px-3">
        <ConnectionMark kind={connection.kind} size="sm" />
        <span className="truncate text-body-2-regular text-text-secondary">
          Calls through <span className="text-text-primary">{connection.name}</span>
        </span>
      </div>
      <BasicsSection editor={editor} fromMcp={connection.kind === "mcp"} />
      {connection.kind === "mcp" ? (
        <ToolSection editor={editor} />
      ) : (
        <RequestSection editor={editor} connection={connection} />
      )}
      <InputsSection editor={editor} />
      <ResponseSection editor={editor} sample={sample} />
      <PermissionsSection editor={editor} />
      <ChannelsSection editor={editor} />
    </div>
  );
}

function Loaded({
  workspace,
  action,
  connection,
  manage,
}: {
  workspace: string;
  action: AiAction | null;
  connection: ActionConnection;
  manage: boolean;
}) {
  const editor = useEditor(workspace, action, connection);
  const beside = useWide(1280);
  const [testing, setTesting] = useState(false);
  const [versions, setVersions] = useState(false);
  const [result, setResult] = useState<ActionTestResult | null>(null);
  const panel = (onClose?: () => void, className?: string) => (
    <TestPanel
      workspace={workspace}
      editor={editor}
      kind={connection.kind}
      result={result}
      onResult={setResult}
      onClose={onClose}
      className={className}
    />
  );

  return (
    <div className="flex h-full min-h-0 min-w-0 flex-1 gap-3">
      <Panel label="Action" ground className="h-full flex-1">
        <Header
          workspace={workspace}
          editor={editor}
          manage={manage}
          onTest={beside ? undefined : () => setTesting(true)}
          onVersions={() => setVersions(true)}
        />
        <ScrollArea ground>
          <fieldset disabled={!manage} className="contents">
            <Body editor={editor} connection={connection} sample={result?.response?.body} />
          </fieldset>
        </ScrollArea>
      </Panel>
      {beside ? (
        panel(undefined, "h-full w-[380px] shrink-0")
      ) : (
        <Drawer isOpen={testing} onOpenChange={setTesting} placement="end" size="md" aria-label="Test" className="p-0">
          {panel(() => setTesting(false), "h-full")}
        </Drawer>
      )}
      {editor.saved ? (
        <VersionsDrawer
          workspace={workspace}
          action={editor.saved}
          isOpen={versions}
          onOpenChange={setVersions}
          onRestore={async (version) => {
            await editor.restore(version);
            setVersions(false);
          }}
        />
      ) : null}
    </div>
  );
}

function Missing({ title, description }: { title: string; description: string }) {
  return (
    <Panel label={title} ground className="h-full flex-1">
      <header className="flex h-12 shrink-0 items-center ps-2.5 pt-[7px]">
        <Back />
      </header>
      <EmptyState icon={RiFlaskLine} title={title} description={description} />
    </Panel>
  );
}

/*
 * ActionEditorPage loads an action, or starts a new one on the named
 * connection, and opens the editor once everything it needs arrived.
 */
export function ActionEditorPage({ action: id, connection: connectionId }: { action: string; connection?: string }) {
  const workspace = useAccount().workspace.id;
  const ready = workspace !== "";
  const fresh = id === "new";
  const connections = useQuery({ ...connectionsQuery(workspace), enabled: ready });
  const action = useQuery({ ...actionQuery(workspace, id), enabled: ready && !fresh });
  const manage = usePermission(workspace, "ai_actions:manage");

  if (connections.isError || action.isError) {
    return <Missing title="Action not found" description="It may have been deleted. Go back to see every action." />;
  }
  if (!connections.data || (!fresh && !action.data)) {
    return <PageSkeleton />;
  }
  const connection = connections.data.find((c) => c.id === (fresh ? connectionId : action.data?.connection_id));
  if (!connection) {
    return <Missing title="Connection not found" description="Pick a connection on the actions page first." />;
  }

  return (
    <Loaded
      key={action.data?.id ?? "new"}
      workspace={workspace}
      action={fresh ? null : (action.data ?? null)}
      connection={connection}
      manage={manage}
    />
  );
}
