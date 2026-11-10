import { createFileRoute } from "@tanstack/react-router";

import { EmailPreferencesPage } from "@/features/broadcasts/email-preferences";

interface PreferencesSearch {
  t?: string;
}

export const Route = createFileRoute("/_auth/email-preferences")({
  validateSearch: (search: Record<string, unknown>): PreferencesSearch => ({
    t: typeof search.t === "string" ? search.t : undefined,
  }),
  component: EmailPreferencesRoute,
});

function EmailPreferencesRoute() {
  const { t } = Route.useSearch();

  return <EmailPreferencesPage token={t ?? ""} />;
}
