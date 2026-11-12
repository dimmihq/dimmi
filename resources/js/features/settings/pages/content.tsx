/*
 * What the workspace publishes and learns from: help centers, the
 * knowledge dimmi reads, and the order of the feedback statuses.
 */
import { FeedbackStatusesSection } from "../feedback-statuses";
import { SettingsPage } from "../layout";

export { KnowledgeSettings } from "@/features/knowledge/source-list";
export { HelpCentersSettings } from "../help-center-settings";

export function FeedbackSettings() {
  return (
    <SettingsPage
      title="Feedback"
      description="Turn the portal on and set who approves new requests from the Feedback page."
    >
      <FeedbackStatusesSection />
    </SettingsPage>
  );
}
