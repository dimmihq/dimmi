import { createFileRoute } from "@tanstack/react-router";

import { InboxSettingsPage } from "@/features/settings/pages/inbox-detail";

export const Route = createFileRoute("/d/settings/inboxes/$inbox")({
  component: InboxRoute,
});

function InboxRoute() {
  const { inbox } = Route.useParams();

  return <InboxSettingsPage key={inbox} inbox={inbox} />;
}
