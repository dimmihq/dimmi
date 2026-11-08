import { RiListOrdered2 } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import { Spinner } from "@/components/base/spinner/spinner";
import { ProcedureEditor } from "@/features/automation/procedure-editor";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { procedureQuery } from "@/lib/api/procedures";

export const Route = createFileRoute("/d/procedures/$procedure")({
  component: ProcedureRoute,
});

/*
 * ProcedureRoute opens the editor once the procedure is freshly read, so
 * the editor's first save names the lock version the server holds now,
 * and says so when the workspace has no such procedure.
 */
function ProcedureRoute() {
  const { procedure: id } = Route.useParams();
  const workspace = useAccount().workspace.id;
  const { data, isError, isFetchedAfterMount } = useQuery({
    ...procedureQuery(workspace, id),
    refetchOnWindowFocus: false,
  });

  if (isError) {
    return (
      <Panel label="Procedure" ground className="h-full flex-1">
        <EmptyState icon={RiListOrdered2} title="No such procedure" description="It may have been deleted.">
          <Link to="/d/procedures" className="text-body-medium text-text-primary underline">
            Back to procedures
          </Link>
        </EmptyState>
      </Panel>
    );
  }

  if (!data || !isFetchedAfterMount) {
    return (
      <Panel label="Procedure" ground className="h-full flex-1 items-center justify-center">
        <Spinner />
      </Panel>
    );
  }

  return <ProcedureEditor key={data.id} workspace={workspace} procedure={data} />;
}
