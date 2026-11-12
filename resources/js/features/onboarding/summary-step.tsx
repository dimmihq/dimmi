/*
 * The end of setup: what the workspace now has, each part changeable in
 * settings, and the way into the app. Finishing is saved before the app
 * opens, so the workspace never asks again.
 */
import { RiArrowRightLine, RiCheckboxCircleFill, RiCheckboxCircleLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { Skeleton } from "@/components/base/skeleton/skeleton";
import { invitationsQuery } from "@/lib/api/invitations";
import { sourcesQuery } from "@/lib/api/knowledge-sources";
import { tagsQuery } from "@/lib/api/tags";
import { updateOnboarding, workspaceQuery } from "@/lib/api/workspace";
import { StepActions } from "./frame";

function Row({ label, value, loading = false }: { label: string; value: string | null; loading?: boolean }) {
  const Icon = value ? RiCheckboxCircleFill : RiCheckboxCircleLine;

  return (
    <li
      aria-busy={loading || undefined}
      className="flex min-h-[52px] items-center gap-3 rounded-2xl bg-background-secondary-default px-3 py-2.5"
    >
      {loading ? (
        <Skeleton shape="circle" className="size-4 shrink-0" />
      ) : (
        <Icon aria-hidden className={value ? "size-4 text-accent-600" : "size-4 text-foreground-icon-quaternary"} />
      )}
      <span className="w-44 shrink-0 text-body-2-medium text-text-primary">{label}</span>
      {loading ? (
        <Skeleton shape="text" className="h-3 w-32" />
      ) : (
        <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-secondary">{value ?? "None yet"}</span>
      )}
    </li>
  );
}

export function SummaryStep({
  workspace,
  name,
  practiced,
  onBack,
}: {
  workspace: string;
  name: string;
  practiced: boolean;
  onBack: () => void;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: tags } = useQuery(tagsQuery(workspace));
  const { data: invitations } = useQuery(invitationsQuery(workspace));
  const { data: current } = useQuery(workspaceQuery(workspace));
  const { data: sources } = useQuery(sourcesQuery(workspace));
  const finish = useMutation({
    mutationFn: () => updateOnboarding(workspace, "done", true),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["workspace", workspace] });
      await navigate({ to: "/d" });
    },
  });
  const tagNames = tags?.map((tag) => tag.name).join(", ");
  const invited = invitations?.length ?? 0;

  return (
    <div>
      <div className="flex items-center gap-3">
        <span className="font-mono text-caption-1-regular text-text-tertiary">Setup complete</span>
        <span aria-hidden className="h-px flex-1 bg-background-tertiary-default" />
      </div>
      <h1 className="mt-5 text-title-1-medium text-text-primary">{name} is ready</h1>
      <p className="mt-2 text-body-regular text-text-secondary">Everything below can be changed in settings.</p>
      <ul className="mt-8 flex flex-col gap-2">
        <Row label="Workspace" value={name} />
        <Row label="Logo" value={current?.logo_url ? "Added" : null} loading={!current} />
        <Row label="Tags" value={tagNames || null} loading={!tags} />
        <Row label="Team" value={invited > 0 ? `${invited} invited` : null} loading={!invitations} />
        <Row
          label="Knowledge"
          value={sources?.length ? sources.map((source) => source.name).join(", ") : null}
          loading={!sources}
        />
        <Row label="Example conversations" value={practiced ? "5 conversations" : null} />
      </ul>
      <div className="mt-8">
        <Button size="small" trailingIcon={RiArrowRightLine} loading={finish.isPending} onClick={() => finish.mutate()}>
          Go to your workspace
        </Button>
      </div>
      <StepActions onBack={onBack} />
    </div>
  );
}
