import { createFileRoute } from "@tanstack/react-router";

import { VerifyEmailPage } from "@/features/auth/link-pages";
import { fragment } from "@/lib/location";

export const Route = createFileRoute("/_auth/verify-email")({
  component: VerifyEmailRoute,
});

function VerifyEmailRoute() {
  return <VerifyEmailPage id={fragment("id")} token={fragment("token")} />;
}
