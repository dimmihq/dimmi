/*
 * One knowledge source's page. It says how the source stands and what it
 * feeds, starts a sync now, and lists its syncs with what each did. Its
 * documents are listed by status, live, failed or excluded, and each can
 * be synced again, set aside or let back in. Each group of settings is
 * its own section, edited where it stands; the changes wait in one draft
 * that the bar at the bottom saves or discards, and leaving with changes
 * unsaved asks first. A synced source can be
 * turned into help center articles; and
 * the source is removed only after a confirmation, since its documents
 * go with it.
 */
import { RiArrowLeftLine, RiRefreshLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { Action, Danger } from "@/features/settings/controls";
import { usePermission } from "@/features/settings/inboxes";
import { Row, RowsSkeleton, Section, SettingsPage } from "@/features/settings/layout";
import { SaveBar } from "@/features/settings/save-bar";
import { useAccount } from "@/features/settings/use-account";
import { ApiError } from "@/lib/api/client";
import {
  cancelRun,
  deleteSource,
  type KnowledgeSource,
  sourceQuery,
  sourceTypesQuery,
  syncSource,
} from "@/lib/api/knowledge-sources";

import { ContentSection } from "./content-section";
import { ConvertModal } from "./convert-modal";
import { DocumentsSection } from "./documents-section";
import { ago, broken, hasTree, inTime, scheduleLabels, sourceState } from "./model";
import { RunsSection } from "./runs-section";
import { SettingsSections } from "./settings-sections";
import { useSourceDraft } from "./source-draft";
import { BrokenBanner, StateChip } from "./source-list";

export function SourcePage({ source: id }: { source: string }) {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const navigate = useNavigate();
  const canManage = usePermission(workspace, "knowledge:manage");
  const canSync = usePermission(workspace, "knowledge:sync");
  const query = useQuery(sourceQuery(workspace, id));
  const types = useQuery(sourceTypesQuery(workspace));
  const [removing, setRemoving] = useState(false);
  const [converting, setConverting] = useState(false);
  const kind = types.data?.find((t) => t.type === query.data?.type);
  const draft = useSourceDraft(workspace, query.data, kind);
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  const refresh = () => client.invalidateQueries({ queryKey: ["workspace", workspace, "knowledge"] });
  const fail = (err: unknown) => toast.error(err instanceof ApiError ? err.message : "That did not work. Try again.");

  const sync = useMutation({
    mutationFn: () => syncSource(workspace, id),
    onSuccess: async () => {
      toast.success("Sync started.");
      await refresh();
    },
    onError: fail,
  });
  const cancel = useMutation({
    mutationFn: (run: string) => cancelRun(workspace, id, run),
    onSuccess: refresh,
    onError: fail,
  });

  useRefreshAfterSync(workspace, id, query.data?.status === "syncing");

  const s = query.data;
  if (!s) {
    return (
      <SettingsPage title={null}>
        <Section title="" loading>
          <RowsSkeleton rows={4} />
        </Section>
      </SettingsPage>
    );
  }

  const syncing = s.status === "syncing";

  return (
    <SettingsPage
      title={s.name}
      description={`${kind?.name ?? s.type} · ${s.mode === "import" ? "Imported into the help center" : "Synced copy"}`}
      actions={
        canSync ? (
          <SyncButton
            syncing={syncing}
            pending={sync.isPending || cancel.isPending}
            onSync={() => sync.mutate()}
            onCancel={() => cancel.mutate(s.last_run?.id ?? "")}
          />
        ) : null
      }
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error ? failure(draft.save.error) : null}
          canSave={draft.canSave}
          hidden={!canManage}
          onSave={() => draft.save.mutate()}
          onDiscard={draft.discard}
        />
      }
    >
      <BackToSources />
      {broken(s) ? <BrokenBanner sources={[s]} /> : null}
      <Section title="Overview">
        <Row label="Status">
          <StateChip state={sourceState(s)} />
        </Row>
        <Row label="Documents" description="Live documents dimmi reads from this source.">
          <span className="text-body-regular text-text-primary">{s.document_count}</span>
        </Row>
        <Row label="Last sync" description={scheduleLine(s)}>
          <span className="text-body-regular text-text-secondary">{ago(s.last_synced_at)}</span>
        </Row>
      </Section>
      {kind ? (
        <SettingsSections workspace={workspace} source={s} kind={kind} canManage={canManage} draft={draft} />
      ) : null}
      {kind && hasTree(kind) ? (
        <ContentSection workspace={workspace} source={s} kind={kind} canManage={canManage} />
      ) : null}
      <RunsSection workspace={workspace} source={s} />
      <DocumentsSection workspace={workspace} source={s} canManage={canManage} canSync={canSync} />
      {canManage && s.mode === "sync" && kind?.targets.includes("help_center") ? (
        <Section title="Turn into articles">
          <Row label="Turn into articles" description="Copy these documents into a help center as editable articles.">
            <Action onPress={() => setConverting(true)}>Turn into articles</Action>
          </Row>
        </Section>
      ) : null}
      {canManage ? (
        <Section title="Remove">
          <Row label="Remove this source" description="Its documents leave dimmi's knowledge. Articles it made stay.">
            <Danger onPress={() => setRemoving(true)}>Remove</Danger>
          </Row>
        </Section>
      ) : null}
      <AlertDialog
        isOpen={removing}
        onOpenChange={setRemoving}
        title={`Remove ${s.name}?`}
        description="dimmi stops reading it and forgets its documents. This cannot be undone."
        confirmLabel="Remove source"
        onConfirm={async () => {
          try {
            await deleteSource(workspace, id);
            await refresh();
            await navigate({ to: "/d/settings/$section", params: { section: "knowledge" } });
          } catch (err) {
            fail(err);
            throw err;
          }
        }}
      />
      {converting ? <ConvertModal workspace={workspace} source={s} onClose={() => setConverting(false)} /> : null}
    </SettingsPage>
  );
}

function SyncButton({
  syncing,
  pending,
  onSync,
  onCancel,
}: {
  syncing: boolean;
  pending: boolean;
  onSync: () => void;
  onCancel: () => void;
}) {
  if (syncing) {
    return (
      <Button variant="secondary" size="small" loading={pending} onClick={onCancel}>
        Cancel sync
      </Button>
    );
  }

  return (
    <Button size="small" leadingIcon={RiRefreshLine} loading={pending} onClick={onSync}>
      Sync now
    </Button>
  );
}

/*
 * useRefreshAfterSync refreshes the source's history and documents once
 * a sync ends, since they stop polling when it does.
 */
function useRefreshAfterSync(workspace: string, id: string, syncing: boolean) {
  const client = useQueryClient();
  const was = useRef(syncing);

  useEffect(() => {
    if (was.current && !syncing) {
      void client.invalidateQueries({ queryKey: ["workspace", workspace, "knowledge", "sources", id] });
    }
    was.current = syncing;
  }, [syncing, client, workspace, id]);
}

/*
 * failure says why a save did not work, in the server's words when it
 * gave them.
 */
function failure(err: Error): string {
  return err instanceof ApiError ? err.message : "The change could not be saved.";
}

function scheduleLine(s: KnowledgeSource): string {
  return s.schedule === "manual" ? scheduleLabels.manual : `${scheduleLabels[s.schedule]} · ${inTime(s.next_sync_at)}`;
}

/*
 * BackToSources returns to the list of every knowledge source.
 */
function BackToSources() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "knowledge" } })}
      >
        All sources
      </Button>
    </div>
  );
}
