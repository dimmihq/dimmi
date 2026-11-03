import { createFileRoute, notFound } from "@tanstack/react-router";

import { ProcedureEditor } from "@/features/automation/procedure-editor";
import { findProcedure } from "@/features/automation/procedures";

export const Route = createFileRoute("/d/procedures/$procedure")({
  loader: ({ params }) => {
    const procedure = findProcedure(params.procedure);
    if (!procedure) {
      throw notFound();
    }

    return procedure;
  },
  component: ProcedureRoute,
});

function ProcedureRoute() {
  const procedure = Route.useLoaderData();

  return <ProcedureEditor key={procedure.id} procedure={procedure} />;
}
