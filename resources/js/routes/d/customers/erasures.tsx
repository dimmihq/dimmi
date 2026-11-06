import { createFileRoute } from "@tanstack/react-router";

import { ErasuresPage } from "@/features/customers/erasures-page";

export const Route = createFileRoute("/d/customers/erasures")({
  component: ErasuresPage,
});
