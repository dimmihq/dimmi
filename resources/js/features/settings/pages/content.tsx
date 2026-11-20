/*
 * What the workspace publishes and learns from: help centers, the
 * knowledge dimmi reads, the changelog, and the feature requests
 * portal's boards and statuses.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Switch } from "@/components/base/switch/switch";
import { type HelpCenter, helpCentersQuery, updateHelpCenter } from "@/lib/api/help-centers";

import { FeedbackBoardsSection } from "../feedback-boards";
import { FeedbackStatusesSection } from "../feedback-statuses";
import { Row, Section, SettingsPage } from "../layout";
import { rowLink } from "../row-link";
import { SaveBar } from "../save-bar";
import { useAccount } from "../use-account";

export { KnowledgeSettings } from "@/features/knowledge/source-list";
export { ChangelogSettings } from "../changelog-settings";
export { HelpCentersSettings } from "../help-center-settings";

/*
 * PortalSection opens the feature requests portal to customers. The
 * switch waits in the page's draft until its save bar sends it.
 */
function PortalSection({
  center,
  open,
  onChange,
}: {
  center: HelpCenter;
  open: boolean;
  onChange: (open: boolean) => void;
}) {
  return (
    <Section title="Portal">
      <Row
        label="Open to customers"
        description={
          center.settings.feedback_board ? (
            <a href={`${center.url}/feedback`} target="_blank" rel="noopener noreferrer" className={rowLink}>
              Customers post and vote at {center.url.replace(/^https?:\/\//, "")}/feedback
            </a>
          ) : (
            "Customers can post and vote on your help center once this is on."
          )
        }
      >
        <Switch aria-label="Open to customers" isSelected={open} onChange={onChange} />
      </Row>
    </Section>
  );
}

export function FeedbackSettings() {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const { data: centers = [], isPending } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers.find((c) => c.is_default) ?? centers[0];
  const saved = center?.settings.feedback_board ?? false;
  const [draft, setDraft] = useState<boolean | null>(null);
  const open = draft ?? saved;
  const save = useMutation({
    mutationFn: () =>
      center
        ? updateHelpCenter(workspace, center.id, { settings: { ...center.settings, feedback_board: open } })
        : Promise.resolve(undefined),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: helpCentersQuery(workspace).queryKey });
      setDraft(null);
    },
  });

  return (
    <SettingsPage
      title="Feature requests"
      description="Where customers post and vote on feature requests."
      footer={
        <SaveBar
          dirty={open !== saved}
          saving={save.isPending}
          error={save.error?.message}
          onSave={() => save.mutate()}
          onDiscard={() => setDraft(null)}
        />
      }
    >
      {center ? <PortalSection center={center} open={open} onChange={setDraft} /> : null}
      {center ? (
        <FeedbackBoardsSection workspace={workspace} helpCenter={center.id} />
      ) : (
        <Section title="Boards" loading={isPending} rows={2} />
      )}
      <FeedbackStatusesSection />
    </SettingsPage>
  );
}
