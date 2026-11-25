/*
 * Operator's page frame: its header and tabs stay put while a tab
 * changes, and only the tab's content below them changes. The tab comes
 * from the address; /d/operator is the overview.
 */
import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";

import { OperatorPage } from "@/features/operator/operator-page";
import { operatorAddress, operatorTab } from "@/features/operator/operator-tabs";

function OperatorLayout() {
  const navigate = useNavigate();
  const { tab } = useParams({ strict: false });

  return <OperatorPage tab={operatorTab(tab)} onTab={(next) => void navigate(operatorAddress(next))} />;
}

export const Route = createFileRoute("/d/operator")({
  component: OperatorLayout,
});
