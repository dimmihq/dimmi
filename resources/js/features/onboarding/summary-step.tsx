/*
 * The end of setup: what the workspace now has, each part changeable in
 * settings, and the way into the app. Finishing is saved before the app
 * opens, so the workspace never asks again.
 */
import { RiArrowRightLine, RiCheckboxCircleFill, RiSubtractLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { invitationsQuery } from "@/lib/api/invitations";
import { tagsQuery } from "@/lib/api/tags";
import { updateOnboarding } from "@/lib/api/workspace";

import { StepActions } from "./frame";

function Row({ label, value }: { label: string; value: string | null }) {
  const Icon = value ? RiCheckboxCircleFill : RiSubtractLine;

  return (
    <li className="flex min-h-[52px] items-center gap-3 rounded-2xl bg-background-secondary-default px-3 py-2.5">
      <Icon aria-hidden className={value ? "size-4 text-accent-600" : "size-4 text-foreground-icon-quaternary"} />
      <span className="w-44 shrink-0 text-body-2-medium text-text-primary">{label}</span>
      <span className="min-w-0 flex-1 truncate text-body-2-regular text-text-secondary">{value ?? "None yet"}</span>
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
        <Row label="Logo" value={null} />
        <Row label="Tags" value={tagNames || null} />
        <Row label="Team" value={invited > 0 ? `${invited} invited` : null} />
        <Row label="Knowledge" value={null} />
        <Row label="Example conversations" value={practiced ? "5 conversations" : null} />
      </ul>
      <div className="mt-8">
        <Button
          size="small"
          trailingIcon={RiArrowRightLine}
          disabled={finish.isPending}
          onClick={() => finish.mutate()}
        >
          Go to your workspace
        </Button>
      </div>
      <StepActions onBack={onBack} />
    </div>
  );
}
