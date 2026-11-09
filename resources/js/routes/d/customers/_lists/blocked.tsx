import { createFileRoute } from "@tanstack/react-router";

import { PeoplePage } from "@/features/customers/people-page";
import { peopleSearch } from "@/features/customers/people-params";

export const Route = createFileRoute("/d/customers/_lists/blocked")({
  validateSearch: peopleSearch,
  component: () => <PeoplePage blocked />,
});
