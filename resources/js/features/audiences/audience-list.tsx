/*
 * The saved audiences: each with what it says in a line and how many
 * people it matched when last counted. A new audience is made on the
 * server at once, matching everyone and named so it never clashes, and
 * opens in the editor.
 */
import { RiAddLine, RiGroupLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import { EmptyState } from "@/features/shell/page";
import { TableSkeleton } from "@/features/shell/table-skeleton";
import { type Audience, audiencesQuery, createAudience } from "@/lib/api/audiences";
import { tagsQuery } from "@/lib/api/tags";

import { CustomersFrame } from "../customers/frame";
import { useWorkspace } from "../customers/names";
import { freshAudienceName } from "./fresh-name";
import { summary } from "./rules";

function size(a: Audience): string {
  const e = a.last_estimate;
  if (!e) {
    return "Not counted yet";
  }
  const people = e.count === 1 ? "1 person" : `${e.count.toLocaleString()} people`;

  return e.approximate ? `About ${people}` : people;
}

export function AudienceList() {
  const workspace = useWorkspace();
  const canManage = usePermission(workspace, "audiences:manage");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data, isPending } = useQuery({ ...audiencesQuery(workspace), enabled: workspace !== "" });
  const tags = useQuery({ ...tagsQuery(workspace), enabled: workspace !== "" }).data ?? [];
  const names = Object.fromEntries([...tags.map((t) => [t.id, t.name]), ...(data ?? []).map((a) => [a.id, a.name])]);
  const create = useMutation({
    mutationFn: () => createAudience(workspace, { name: freshAudienceName((data ?? []).map((a) => a.name)) }),
    onSuccess: (a) => {
      void queryClient.invalidateQueries({ queryKey: audiencesQuery(workspace).queryKey });
      void navigate({ to: "/d/customers/audiences/$audience", params: { audience: a.id } });
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <CustomersFrame
      actions={
        canManage ? (
          <Button size="small" leadingIcon={RiAddLine} loading={create.isPending} onClick={() => create.mutate()}>
            New audience
          </Button>
        ) : null
      }
    >
      {isPending ? <TableSkeleton rows={4} /> : null}
      {data?.length === 0 ? (
        <EmptyState
          icon={RiGroupLine}
          title="No audiences yet"
          description="Name a group of customers once, then use it in workflows and everywhere else that asks who."
        />
      ) : null}
      {data && data.length > 0 ? (
        <ul className="flex flex-col px-2 pb-6">
          {data.map((a) => (
            <li key={a.id}>
              <Link
                to="/d/customers/audiences/$audience"
                params={{ audience: a.id }}
                className="flex items-center gap-3 rounded-xl px-3 py-2.5 outline-none transition-colors hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
              >
                <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-background-secondary-default">
                  <RiGroupLine aria-hidden className="size-4 text-foreground-icon-secondary" />
                </span>
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate text-body-regular text-text-primary">{a.name}</span>
                  <span className="truncate text-body-2-regular text-text-secondary">
                    {a.description || summary(a.rules, names)}
                  </span>
                </span>
                <span className="shrink-0 text-body-2-regular text-text-tertiary">{size(a)}</span>
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </CustomersFrame>
  );
}
