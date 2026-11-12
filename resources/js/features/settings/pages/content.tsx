/*
 * What the workspace publishes and learns from: help centers, the
 * knowledge dimmi reads, the changelog, and the feedback portal's boards
 * and statuses.
 */
import { useQuery } from "@tanstack/react-query";

import { helpCentersQuery } from "@/lib/api/help-centers";

import { FeedbackBoardsSection } from "../feedback-boards";
import { FeedbackStatusesSection } from "../feedback-statuses";
import { Section, SettingsPage } from "../layout";
import { useAccount } from "../use-account";

export { KnowledgeSettings } from "@/features/knowledge/source-list";
export { ChangelogSettings } from "../changelog-settings";
export { HelpCentersSettings } from "../help-center-settings";

export function FeedbackSettings() {
  const workspace = useAccount().workspace.id;
  const { data: centers = [], isPending } = useQuery({ ...helpCentersQuery(workspace), enabled: workspace !== "" });
  const center = centers.find((c) => c.is_default) ?? centers[0];

  return (
    <SettingsPage
      title="Feedback"
      description="Where customers post and vote on requests. Open the portal from the Feedback page."
    >
      {center ? (
        <FeedbackBoardsSection workspace={workspace} helpCenter={center.id} />
      ) : (
        <Section title="Boards" loading={isPending} rows={2} />
      )}
      <FeedbackStatusesSection />
    </SettingsPage>
  );
}
