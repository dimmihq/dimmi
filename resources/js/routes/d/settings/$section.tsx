import { createFileRoute, notFound } from "@tanstack/react-router";

import { type SettingsSection, settingsPages } from "@/features/shell/navigation";
import { FeaturePage } from "@/features/shell/page";

export const Route = createFileRoute("/d/settings/$section")({
  beforeLoad: ({ params }) => {
    if (!(params.section in settingsPages)) {
      throw notFound();
    }
  },
  component: SettingsRoute,
});

function SettingsRoute() {
  const page = settingsPages[Route.useParams().section as SettingsSection];

  return <FeaturePage title={page.label} icon={page.icon} description={page.description} />;
}
