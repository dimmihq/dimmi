import { createFileRoute } from "@tanstack/react-router";

import { CustomersLists } from "@/features/customers/frame";

/*
 * The customer lists share one panel, header and tab strip, which stay
 * mounted while the tabs switch pages.
 */
export const Route = createFileRoute("/d/customers/_lists")({
  component: CustomersLists,
});
