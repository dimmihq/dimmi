import { createFileRoute } from "@tanstack/react-router";

import { ContactProfile } from "@/features/customers/contact-profile";

export const Route = createFileRoute("/d/customers/$contact")({
  validateSearch: (search: Record<string, unknown>): { duplicates?: boolean } =>
    search.duplicates === true || search.duplicates === "true" ? { duplicates: true } : {},
  component: ContactRoute,
});

function ContactRoute() {
  const { contact } = Route.useParams();

  return <ContactProfile key={contact} id={contact} />;
}
