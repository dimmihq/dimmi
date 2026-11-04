import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

import { takeInvitationToken } from "@/features/auth/invitation-link";
import { InvitationPage } from "@/features/auth/invitation-page";
import { authConfigQuery } from "@/lib/api/auth";

interface InvitationSearch {
  error?: string;
}

export const Route = createFileRoute("/_auth/invitation")({
  validateSearch: (search: Record<string, unknown>): InvitationSearch => ({
    error: typeof search.error === "string" ? search.error : undefined,
  }),
  component: InvitationRoute,
});

function InvitationRoute() {
  const search = Route.useSearch();
  const { data: config } = useSuspenseQuery(authConfigQuery);
  const [token] = useState(takeInvitationToken);

  return <InvitationPage token={token} config={config} error={search.error} />;
}
