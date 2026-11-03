import { createFileRoute } from "@tanstack/react-router";

import { MagicLinkPage } from "@/features/auth/link-pages";
import { useSignedIn } from "@/features/auth/use-signed-in";
import { fragment } from "@/lib/location";

export const Route = createFileRoute("/_auth/login_/magic")({
  component: MagicLinkRoute,
});

function MagicLinkRoute() {
  return <MagicLinkPage token={fragment("token")} onSignedIn={useSignedIn("/d")} />;
}
