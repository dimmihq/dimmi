import { createFileRoute, notFound, redirect } from "@tanstack/react-router";

import { settingsViews } from "@/features/settings/views";
import { type SettingsSection, settingsPages } from "@/features/shell/navigation";

export const Route = createFileRoute("/d/settings/$section")({
  beforeLoad: ({ params }) => {
    // Operator's settings moved onto its own page, as a tab.
    if (params.section === "operator") {
      throw redirect({ to: "/d/operator/$tab", params: { tab: "settings" }, replace: true });
    }
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
