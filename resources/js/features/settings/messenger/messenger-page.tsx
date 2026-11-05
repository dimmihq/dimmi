/*
 * One messenger's settings, with the preview messenger in a column beside
 * them on screens wide enough for both. Every change shows on it at once,
 * before anything is saved: changes collect in a draft, a bar at the
 * foot says they are unsaved, Discard puts the page back as saved and
 * Save sends them in one update. Leaving with unsaved changes asks first.
 * Someone who may only read settings sees them without changing them.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { updateWidget, type WidgetChange, type WidgetSettings, widgetQuery } from "@/lib/api/widget-settings";

import { usePermission } from "../inboxes";
import { PageSkeleton, SettingsPage } from "../layout";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import { changed, withChange, withDraft } from "./draft";
import { PreviewPane } from "./preview-pane";
import {
  AppearanceSection,
  DangerSection,
  GeneralSection,
  InstallSection,
  TabsSection,
  TextsSection,
  VisitorsSection,
} from "./sections";
import { usePreviewConfig } from "./use-preview-config";

function BackToMessengers() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "messenger" } })}
      >
        Messengers
      </Button>
    </div>
  );
}

/*
 * useMessengerDraft holds the unsaved changes and saves them.
 */
function useMessengerDraft(workspace: string, saved: WidgetSettings) {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState<WidgetChange>({});
  const [generation, setGeneration] = useState(0);
  const save = useMutation({
    mutationFn: () => updateWidget(workspace, saved.id, draft),
    onSuccess: async (updated) => {
      queryClient.setQueryData(widgetQuery(workspace, saved.id).queryKey, updated);
      setDraft({});
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "widget-settings"] });
    },
  });

  return {
    draft,
    shown: withDraft(saved, draft),
    dirty: changed(draft),
    generation,
    set: (change: WidgetChange) => setDraft((before) => withChange(before, change)),
    discard: () => {
      save.reset();
      setDraft({});
      setGeneration((n) => n + 1);
    },
    save,
  };
}

function SaveBar({ edit }: { edit: ReturnType<typeof useMessengerDraft> }) {
  if (!(edit.dirty || edit.save.error)) {
    return null;
  }

  return (
    <div className="flex items-center gap-3 rounded-2xl border border-border-button-default bg-background-primary-default py-2 ps-4 pe-2 shadow-dropdown">
      <span
        role={edit.save.error ? "alert" : "status"}
        className={
          edit.save.error
            ? "min-w-0 flex-1 truncate text-body-2-regular text-text-error-primary"
            : "min-w-0 flex-1 truncate text-body-2-regular text-text-secondary"
        }
      >
        {edit.save.error ? failureText(edit.save.error) : "Unsaved changes"}
      </span>
      <Button size="small" variant="secondary" disabled={edit.save.isPending} onClick={edit.discard}>
        Discard
      </Button>
      <Button size="small" loading={edit.save.isPending} disabled={!edit.dirty} onClick={() => edit.save.mutate()}>
        Save
      </Button>
    </div>
  );
}

function Editor({ workspace, saved }: { workspace: string; saved: WidgetSettings }) {
  const edit = useMessengerDraft(workspace, saved);
  const locked = !usePermission(workspace, "settings:write");
  const preview = usePreviewConfig(workspace, saved.id, edit.draft);
  const locales = (preview.data?.locales as string[] | undefined) ?? ["en"];
  useBlocker({
    shouldBlockFn: () => edit.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => edit.dirty,
  });
  const props = { value: edit.shown, set: edit.set, locked };

  return (
    <div className="flex h-full min-w-0 flex-1 gap-3">
      <SettingsPage
        title={saved.name}
        description={`Conversations land in ${saved.inbox.name}.`}
        footer={<SaveBar edit={edit} />}
      >
        <BackToMessengers />
        <div key={edit.generation} className="contents">
          <InstallSection value={edit.shown} />
          <GeneralSection {...props} locales={locales} />
          <AppearanceSection {...props} />
          <TextsSection {...props} />
          <VisitorsSection {...props} />
          <TabsSection {...props} />
          {locked ? null : <DangerSection workspace={workspace} value={edit.shown} />}
        </div>
        {preview.error ? (
          <p role="alert" className="text-body-2-regular text-text-error-primary">
            {failureText(preview.error)}
          </p>
        ) : null}
      </SettingsPage>
      <PreviewPane config={preview.data} />
    </div>
  );
}

export function MessengerPage({ widget }: { widget: string }) {
  const workspace = useAccount().workspace.id;
  const { data, isError } = useQuery(widgetQuery(workspace, widget));

  if (isError) {
    return (
      <SettingsPage title="Messenger not found" description="It was deleted, or it belongs to another workspace.">
        <BackToMessengers />
      </SettingsPage>
    );
  }

  if (!data) {
    return <PageSkeleton />;
  }

  return <Editor workspace={workspace} saved={data} />;
}
