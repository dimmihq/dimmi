/*
 * One of Operator's tabs at its own address, such as
 * /d/operator/routines. The overview lives at /d/operator; an address
 * naming it, or a tab that does not exist, lands there. The page around
 * it is drawn by the operator route, so changing tab keeps the header.
 */
import { createFileRoute, redirect } from "@tanstack/react-router";

import { operatorTab } from "@/features/operator/operator-tabs";

export const Route = createFileRoute("/d/operator/$tab")({
  beforeLoad: ({ params }) => {
    if (operatorTab(params.tab) === "overview") {
      throw redirect({ to: "/d/operator", replace: true });
    }
  },
  component: () => null,
});
