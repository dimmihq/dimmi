import { createFileRoute } from "@tanstack/react-router";

import { AudienceList } from "@/features/audiences/audience-list";
import { audienceSearch } from "@/features/customers/people-params";

export const Route = createFileRoute("/d/customers/_lists/audiences/")({
  validateSearch: audienceSearch,
  component: AudienceList,
});
