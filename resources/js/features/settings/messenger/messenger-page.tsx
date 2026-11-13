/*
 * One messenger's settings, with the preview messenger in a column beside
 * them on screens wide enough for both. Every change shows on it at once,
 * before anything is saved: changes collect in a draft, a bar at the
 * foot says they are unsaved, Discard puts the page back as saved and
 * Save sends them in one update, then who sees it in another. Leaving with unsaved changes asks first.
 * Someone who may only read settings sees them without changing them.
 * Picking a language for the texts shows the preview in that language.
 */
import { RiArrowLeftLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import {
  saveMessengerAudiences,
  updateWidget,
  type WidgetChange,
  type WidgetSettings,
  widgetQuery,
} from "@/lib/api/widget-settings";

import { usePermission } from "../inboxes";
import { PageSkeleton, SettingsPage } from "../layout";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import {
  type AudienceChoices,
  AudiencesSection,
  audienceChange,
  audienceProblem,
  type Slot,
  savedChoice,
} from "./audiences-section";
import { changed, withChange, withDraft } from "./draft";
import { forPreview, linkProblems } from "./links";
import { LinksSection } from "./links-section";
import { allLanguages } from "./parts";
import { PreviewPane } from "./preview-pane";
import {
  AppearanceSection,
  DangerSection,
  GeneralSection,
  InstallSection,
  TabsSection,
  VisitorsSection,
} from "./sections";
import { TextsSection } from "./texts-section";
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
 * useMessengerDraft holds the unsaved changes and saves them: the
 * settings first, then the audiences picked. When the audiences are
 * refused the settings stay saved and the picks stay pending, with the
 * server's reason in the bar.
 */
function useMessengerDraft(workspace: string, saved: WidgetSettings) {
  const queryClient = useQueryClient();
  const key = widgetQuery(workspace, saved.id).queryKey;
  const [draft, setDraft] = useState<WidgetChange>({});
  const [audiences, setAudiences] = useState<AudienceChoices>({});
  const [generation, setGeneration] = useState(0);
  const save = useMutation({
    mutationFn: async () => {
      if (changed(draft)) {
        queryClient.setQueryData(key, await updateWidget(workspace, saved.id, draft));
        setDraft({});
      }
      if (Object.keys(audiences).length > 0) {
        try {
          const answer = await saveMessengerAudiences(workspace, saved.id, audienceChange(audiences));
          queryClient.setQueryData(key, (before) => before && { ...before, audiences: answer.rules });
          setAudiences({});
        } catch (error) {
          throw new Error(audienceProblem(error));
        }
      }
    },
    onSuccess: () => toast.success("Messenger saved"),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "widget-settings"] }),
  });

  return {
    draft,
    audiences,
    shown: withDraft(saved, draft),
    dirty: changed(draft) || Object.keys(audiences).length > 0,
    generation,
    set: (change: WidgetChange) => setDraft((before) => withChange(before, change)),
    pickAudience: (slot: Slot, choice: string) =>
      setAudiences(({ [slot]: _, ...rest }) =>
        choice === savedChoice(saved.audiences, slot) ? rest : { ...rest, [slot]: choice },
      ),
    discard: () => {
      save.reset();
      setDraft({});
      setAudiences({});
      setGeneration((n) => n + 1);
    },
    save,
  };
}

function Editor({ workspace, saved }: { workspace: string; saved: WidgetSettings }) {
  const edit = useMessengerDraft(workspace, saved);
  const locked = !usePermission(workspace, "settings:write");
  const sent = forPreview(edit.draft);
  const [language, setLanguage] = useState(allLanguages);
  const preview = usePreviewConfig(workspace, saved.id, sent.change, language === allLanguages ? undefined : language);
  const locales = (preview.data?.locales as string[] | undefined) ?? ["en"];
  useBlocker({
    shouldBlockFn: () => edit.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => edit.dirty,
  });
  const props = { value: edit.shown, set: edit.set, locked };
  const problems = { ...linkProblems(preview.error, sent.rows), ...linkProblems(edit.save.error) };

  return (
    <div className="flex h-full min-w-0 flex-1 gap-3">
      <SettingsPage
        title={saved.name}
        description={`Conversations land in ${saved.inbox.name}.`}
        footer={
          <SaveBar
            dirty={edit.dirty}
            saving={edit.save.isPending}
            error={edit.save.error ? failureText(edit.save.error) : null}
            onSave={() => edit.save.mutate()}
            onDiscard={edit.discard}
          />
        }
      >
        <BackToMessengers />
        <div key={edit.generation} className="contents">
          <InstallSection value={edit.shown} />
          <GeneralSection {...props} locales={locales} />
          <AppearanceSection {...props} />
          <TextsSection {...props} locales={locales} language={language} onLanguage={setLanguage} />
          <VisitorsSection {...props} />
          <LinksSection {...props} problems={problems} />
          <TabsSection {...props} />
          <AudiencesSection
            workspace={workspace}
            value={edit.shown}
            locked={locked}
            choices={edit.audiences}
            onPick={edit.pickAudience}
          />
          {locked ? null : <DangerSection workspace={workspace} value={edit.shown} />}
        </div>
        {preview.error && Object.keys(linkProblems(preview.error)).length === 0 ? (
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
