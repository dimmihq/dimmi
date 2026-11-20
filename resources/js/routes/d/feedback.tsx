import { createFileRoute } from "@tanstack/react-router";

import { PageOutlet } from "@/features/layout/page-outlet";

/*
 * The section's pages swap inside it with the app's page reveal, so
 * opening a record from its list animates in like any other page.
 */
export const Route = createFileRoute("/d/feedback")({
  component: () => <PageOutlet className="flex h-full min-w-0 flex-1" />,
});
