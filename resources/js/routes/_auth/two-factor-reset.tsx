import { createFileRoute } from "@tanstack/react-router";

import { TwoFactorResetPage } from "@/features/auth/two-factor-reset-page";
import { fragment } from "@/lib/location";

export const Route = createFileRoute("/_auth/two-factor-reset")({
  component: TwoFactorResetRoute,
});

function TwoFactorResetRoute() {
  return <TwoFactorResetPage id={fragment("id")} token={fragment("token")} />;
}
