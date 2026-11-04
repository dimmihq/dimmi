import { createFileRoute } from "@tanstack/react-router";

import { ConfirmEmailChangePage } from "@/features/auth/link-pages";
import { fragment } from "@/lib/location";

export const Route = createFileRoute("/_auth/email/confirm")({
  component: ConfirmEmailRoute,
});

function ConfirmEmailRoute() {
  return <ConfirmEmailChangePage id={fragment("id")} token={fragment("token")} />;
}
