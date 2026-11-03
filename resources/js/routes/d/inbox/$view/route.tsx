import { createFileRoute, notFound } from "@tanstack/react-router";

import { InboxPage } from "@/features/inbox/inbox-page";
import { type InboxView, inboxViews } from "@/features/shell/navigation";

export const Route = createFileRoute("/d/inbox/$view")({
  beforeLoad: ({ params }) => {
    if (!(params.view in inboxViews)) {
      throw notFound();
    }
  },
  component: InboxRoute,
});

function InboxRoute() {
  const { view } = Route.useParams();

  return <InboxPage view={view as InboxView} />;
}
