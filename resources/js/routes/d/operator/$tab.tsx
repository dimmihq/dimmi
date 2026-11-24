/*
 * One of Operator's tabs at its own address, such as
 * /d/operator/routines. The overview lives at /d/operator; an address
 * naming it, or a tab that does not exist, lands there.
 */
import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";

import { OperatorPage } from "@/features/operator/operator-page";
import { operatorAddress, operatorTab } from "@/features/operator/operator-tabs";

function OperatorTabRoute() {
  const navigate = useNavigate();
  const { tab } = Route.useParams();

  return <OperatorPage tab={operatorTab(tab)} onTab={(next) => void navigate(operatorAddress(next))} />;
}

export const Route = createFileRoute("/d/operator/$tab")({
  beforeLoad: ({ params }) => {
    if (operatorTab(params.tab) === "overview") {
      throw redirect({ to: "/d/operator", replace: true });
    }
  },
  component: OperatorTabRoute,
});
