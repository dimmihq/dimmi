import { createFileRoute, useNavigate } from "@tanstack/react-router";

import { OperatorPage } from "@/features/operator/operator-page";
import { operatorAddress } from "@/features/operator/operator-tabs";

function OperatorRoute() {
  const navigate = useNavigate();

  return <OperatorPage tab="overview" onTab={(next) => void navigate(operatorAddress(next))} />;
}

export const Route = createFileRoute("/d/operator/")({
  component: OperatorRoute,
});
