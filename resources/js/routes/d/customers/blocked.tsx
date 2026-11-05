import { createFileRoute } from "@tanstack/react-router";

import { PeoplePage } from "@/features/customers/people-page";

export const Route = createFileRoute("/d/customers/blocked")({
  component: () => <PeoplePage blocked />,
});
