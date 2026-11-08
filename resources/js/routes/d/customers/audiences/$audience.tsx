import { RiGroupLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import { Spinner } from "@/components/base/spinner/spinner";
import { AudienceEditor } from "@/features/audiences/audience-editor";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { audienceQuery } from "@/lib/api/audiences";

export const Route = createFileRoute("/d/customers/audiences/$audience")({
  component: AudienceRoute,
});

/*
 * AudienceRoute opens the editor once the audience is freshly read, so
 * its first save names the lock version the server holds now, and says
 * so when the workspace has no such audience.
 */
function AudienceRoute() {
  const { audience: id } = Route.useParams();
  const workspace = useAccount().workspace.id;
  const { data, isError, isFetchedAfterMount } = useQuery({
    ...audienceQuery(workspace, id),
    refetchOnWindowFocus: false,
  });

  if (isError) {
    return (
      <Panel label="Audience" ground className="h-full flex-1">
        <EmptyState icon={RiGroupLine} title="No such audience" description="It may have been deleted.">
          <Link to="/d/customers/audiences" className="text-body-medium text-text-primary underline">
            Back to audiences
          </Link>
        </EmptyState>
      </Panel>
    );
  }

  if (!data || !isFetchedAfterMount) {
    return (
      <Panel label="Audience" ground className="h-full flex-1 items-center justify-center">
        <Spinner />
      </Panel>
    );
  }

  return <AudienceEditor key={data.id} workspace={workspace} audience={data} />;
}
