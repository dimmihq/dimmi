/*
 * The procedures page: the step-by-step instructions dimmi follows for
 * particular requests, when each applies, how often dimmi followed it in
 * the last 30 days and whether it is live. A new procedure is made on the
 * server at once, as an empty draft named so it never clashes, and opens
 * in the editor. Teammates who may only read procedures see the list
 * without the button.
 */
import { RiAddLine, RiListOrdered2 } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import { Item, Section } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { createProcedure, type Procedure, proceduresQuery } from "@/lib/api/procedures";

import { KindMark, ListPage, OpenRow, StatusChip } from "./parts";

/*
 * freshProcedureName is the first "Untitled procedure" name no procedure
 * has.
 */
function freshProcedureName(taken: string[]): string {
  const names = new Set(taken.map((name) => name.toLowerCase()));
  for (let count = 1; ; count += 1) {
    const name = count === 1 ? "Untitled procedure" : `Untitled procedure ${count}`;
    if (!names.has(name.toLowerCase())) {
      return name;
    }
  }
}

function detailOf(p: Procedure): string {
  const when = p.definition.description.trim() || "No description yet";
  const used = p.uses === 1 ? "Used in 1 conversation" : `Used in ${p.uses} conversations`;

  return p.status === "live" ? `${when} · ${used} in 30 days` : when;
}

export function ProcedureList() {
  const workspace = useAccount().workspace.id;
  const canManage = usePermission(workspace, "ai_agent:manage");
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data, isPending } = useQuery(proceduresQuery(workspace));
  const create = useMutation({
    mutationFn: () => createProcedure(workspace, freshProcedureName((data ?? []).map((p) => p.name))),
    onSuccess: (p) => {
      void queryClient.invalidateQueries({ queryKey: proceduresQuery(workspace).queryKey });
      void navigate({ to: "/d/procedures/$procedure", params: { procedure: p.id } });
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <ListPage
      title="Procedures"
      description="Teach dimmi how your team handles a request, step by step, in plain words."
      action={
        canManage ? (
          <Button size="small" leadingIcon={RiAddLine} loading={create.isPending} onClick={() => create.mutate()}>
            New procedure
          </Button>
        ) : null
      }
    >
      <Section
        title="Procedures"
        description="dimmi picks the live procedure whose description matches what the customer asks."
        loading={isPending}
      >
        {data?.length === 0 ? (
          <Item
            mark={<KindMark icon={RiListOrdered2} tint="neutral" />}
            title="No procedures yet"
            detail="Write one for a request your team handles the same way every time."
          />
        ) : null}
        {(data ?? []).map((p) => (
          <OpenRow
            key={p.id}
            target={{ to: "/d/procedures/$procedure", params: { procedure: p.id } }}
            mark={<KindMark icon={RiListOrdered2} tint="lime" />}
            title={p.name}
            detail={detailOf(p)}
          >
            <StatusChip status={p.status} />
          </OpenRow>
        ))}
      </Section>
    </ListPage>
  );
}
