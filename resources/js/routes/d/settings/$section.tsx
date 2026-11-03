import { createFileRoute, notFound } from "@tanstack/react-router";

import { settingsViews } from "@/features/settings/views";
import { type SettingsSection, settingsPages } from "@/features/shell/navigation";

export const Route = createFileRoute("/d/settings/$section")({
  beforeLoad: ({ params }) => {
    if (!(params.section in settingsPages)) {
      throw notFound();
    }
  },
  component: SettingsRoute,
});

function SettingsRoute() {
  const View = settingsViews[Route.useParams().section as SettingsSection];

  return <View />;
}
