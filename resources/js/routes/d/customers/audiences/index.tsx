import { createFileRoute } from "@tanstack/react-router";

import { AudienceList } from "@/features/audiences/audience-list";

export const Route = createFileRoute("/d/customers/audiences/")({
  component: AudienceList,
});
