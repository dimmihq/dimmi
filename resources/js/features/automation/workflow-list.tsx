/*
 * The workflows page: every workflow of the workspace with its trigger
 * and whether it is live, in the order they run when two share a
 * trigger. A new workflow begins by picking its trigger; it is made on
 * the server at once, as a draft named so it never clashes with another,
 * and the builder opens on it. Teammates who may only read workflows see
 * the list without the button.
 */
import { RiAddLine, RiFlowChart } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import { Item, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { createWorkflow, type Workflow, workflowsQuery } from "@/lib/api/workflows";

import { freshName } from "./fresh-name";
import { KindMark, ListPage, OpenRow, StatusChip } from "./parts";
import { TriggerPicker } from "./trigger-picker";
import { type TriggerKind, tintOf, triggers } from "./triggers";

function detailOf(workflow: Workflow): string {
  const name = triggers[workflow.trigger as TriggerKind]?.name ?? workflow.trigger;

  return workflow.status !== "draft" && workflow.has_unpublished_changes ? `${name} · Unpublished changes` : name;
}

export function WorkflowList() {
  const workspace = useAccount().workspace.id;
  const canManage = usePermission(workspace, "workflows:manage");
  const [picking, setPicking] = useState(false);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: workflows, isPending } = useQuery(workflowsQuery(workspace));
  const create = useMutation({
    mutationFn: (trigger: TriggerKind) =>
      createWorkflow(workspace, freshName((workflows ?? []).map((workflow) => workflow.name)), trigger),
    onSuccess: (workflow) => {
      void queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "workflows"] });
      void navigate({ to: "/d/workflows/$workflow", params: { workflow: workflow.id } });
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <ListPage
      title="Workflows"
      description="Route conversations, ask the right questions and keep the team's work tidy."
      action={
        canManage ? (
          <Button size="small" leadingIcon={RiAddLine} onClick={() => setPicking(true)}>
            New workflow
          </Button>
        ) : null
      }
    >
      <TriggerPicker
        isOpen={picking}
        onOpenChange={setPicking}
        onPick={(trigger) => {
          setPicking(false);
          create.mutate(trigger);
        }}
      />
      <Section
        title="Workflows"
        description="When two workflows share a trigger, the higher one runs first."
        loading={isPending}
      >
        {workflows?.length === 0 ? (
          <Item
            mark={<KindMark icon={RiFlowChart} tint="neutral" />}
            title="No workflows yet"
            detail="Make one to route conversations or ask customers what they need."
          />
        ) : null}
        {(workflows ?? []).map((workflow) => {
          const trigger = workflow.trigger as TriggerKind;
          const look = triggers[trigger];

          return (
            <OpenRow
              key={workflow.id}
              target={{ to: "/d/workflows/$workflow", params: { workflow: workflow.id } }}
              mark={look ? <KindMark icon={look.icon} tint={tintOf(trigger)} /> : null}
              title={workflow.name}
              detail={detailOf(workflow)}
            >
              <StatusChip status={workflow.status} />
            </OpenRow>
          );
        })}
      </Section>
    </ListPage>
  );
}
