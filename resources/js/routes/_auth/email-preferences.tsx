import { createFileRoute } from "@tanstack/react-router";

import { EmailPreferencesPage } from "@/features/broadcasts/email-preferences";
import { NotificationUnsubscribePage } from "@/features/notifications/notification-unsubscribe";

/*
 * t is a broadcast email's preference link; n is the unsubscribe link
 * of a teammate notification email.
 */
interface PreferencesSearch {
  t?: string;
  n?: string;
}

export const Route = createFileRoute("/_auth/email-preferences")({
  validateSearch: (search: Record<string, unknown>): PreferencesSearch => ({
    t: typeof search.t === "string" ? search.t : undefined,
    n: typeof search.n === "string" ? search.n : undefined,
  }),
  component: EmailPreferencesRoute,
});

function EmailPreferencesRoute() {
  const { t, n } = Route.useSearch();
  if (n !== undefined) {
    return <NotificationUnsubscribePage token={n} />;
  }

  return <EmailPreferencesPage token={t ?? ""} />;
}
