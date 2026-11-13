/*
 * One ticket type's editor. Its settings are a draft saved in one go
 * from a bar that floats over the page while anything changed; leaving
 * with unsaved changes asks first. Fields save on their own, as they are
 * added, edited, archived or dragged. Beside the editor, on wide
 * screens, the customer's form updates live as the type changes; on
 * narrower ones it follows the editor. Back-office types, trackers and
 * incidents have no customer form, so their editor shows no customer
 * settings and no preview.
 */
import { RiArrowLeftLine, RiCoupon3Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import { PageSkeleton, SettingsPage } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel, ScrollArea } from "@/features/shell/page";
import { inboxesQuery } from "@/lib/api/inboxes";
import { teamsQuery } from "@/lib/api/teams";
import { type TicketType, ticketKeys, ticketStatesQuery, ticketTypesQuery, updateTicketType } from "@/lib/api/tickets";

import { CustomerPreview } from "./customer-preview";
import { FieldBuilder } from "./field-builder";
import { draftOf, type TypeDraft, typeChanges } from "./form";
import { StatusPreview } from "./status-preview";
import { CustomerSection, DetailsSection, RoutingSection } from "./type-sections";
import { TypeStatesSection } from "./type-states";

function BackToTickets() {
  const navigate = useNavigate();

  return (
    <div>
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiArrowLeftLine}
        onClick={() => navigate({ to: "/d/settings/$section", params: { section: "tickets" } })}
      >
        Tickets
      </Button>
    </div>
  );
}

/*
 * useTypeDraft keeps the unsaved settings of the type and saves them,
 * with where the tickets of each state taken off the type move.
 */
function useTypeDraft(workspace: string, type: TicketType) {
  const client = useQueryClient();
  const [saved, setSaved] = useState(() => draftOf(type));
  const [draft, setDraft] = useState(saved);
  const [moves, setMoves] = useState<Record<string, string>>({});
  const changes = typeChanges(saved, draft);
  const dirty = Object.keys(changes).length > 0;
  const save = useMutation({
    mutationFn: () => {
      const move_tickets = Object.entries(moves)
        .filter(([from, to]) => !draft.state_ids.includes(from) && draft.state_ids.includes(to))
        .map(([from_state_id, to_state_id]) => ({ from_state_id, to_state_id }));

      return updateTicketType(
        workspace,
        type.id,
        move_tickets.length > 0 && changes.state_ids ? { ...changes, move_tickets } : changes,
      );
    },
    onSuccess: async (next) => {
      const fresh = draftOf(next);
      setSaved(fresh);
      setDraft(fresh);
      setMoves({});
      await client.invalidateQueries({ queryKey: ticketKeys.all(workspace) });
      toast.success("Type saved");
    },
  });

  return {
    draft,
    dirty,
    save,
    moves,
    setMoves,
    set: (change: Partial<TypeDraft>) => setDraft((was) => ({ ...was, ...change })),
    discard: () => {
      setDraft(saved);
      setMoves({});
    },
  };
}

function SaveBar({ edit }: { edit: ReturnType<typeof useTypeDraft> }) {
  if (!(edit.dirty || edit.save.error)) {
    return null;
  }

  return (
    <div className="flex items-center gap-3 rounded-2xl border border-border-button-default bg-background-primary-default py-2 ps-4 pe-2 shadow-dropdown">
      <span
        role={edit.save.error ? "alert" : "status"}
        className={
          edit.save.error
            ? "min-w-0 flex-1 truncate text-body-2-regular text-text-error-primary"
            : "min-w-0 flex-1 truncate text-body-2-regular text-text-secondary"
        }
      >
        {edit.save.error ? edit.save.error.message : "Unsaved changes"}
      </span>
      <Button size="small" variant="secondary" disabled={edit.save.isPending} onClick={edit.discard}>
        Discard
      </Button>
      <Button size="small" loading={edit.save.isPending} disabled={!edit.dirty} onClick={() => edit.save.mutate()}>
        Save
      </Button>
    </div>
  );
}

function Editor({ workspace, type }: { workspace: string; type: TicketType }) {
  const edit = useTypeDraft(workspace, type);
  const locked = !usePermission(workspace, "settings:write");
  const { data: states = [] } = useQuery(ticketStatesQuery(workspace));
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));
  const [previewId, setPreviewId] = useState<string | null>(null);
  const previewState = states.find((s) => s.id === (previewId ?? edit.draft.default_state_id));
  const { data: inboxes = [] } = useQuery({ ...inboxesQuery(workspace), enabled: workspace !== "" });
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: workspace !== "" });
  useBlocker({
    shouldBlockFn: () => edit.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => edit.dirty,
  });
  const props = { draft: edit.draft, set: edit.set, locked };
  const customer = type.kind === "customer";
  const preview = (
    <div className="flex flex-col gap-8">
      <CustomerPreview
        name={edit.draft.name}
        description={edit.draft.description}
        fields={type.fields}
        shown={edit.draft.customer_visible}
      />
      <StatusPreview typeName={edit.draft.name} state={previewState} />
    </div>
  );

  return (
    <div className="flex h-full min-w-0 flex-1 gap-3">
      <SettingsPage
        title={type.name}
        description={type.archived_at ? "Archived. Old tickets keep this type; new ones can't use it." : undefined}
        footer={<SaveBar edit={edit} />}
      >
        <BackToTickets />
        <DetailsSection {...props} kind={type.kind} />
        <FieldBuilder workspace={workspace} type={type} canChange={!locked} />
        <RoutingSection {...props} inboxes={inboxes} teams={teams} />
        <TypeStatesSection
          {...props}
          workspace={workspace}
          type={type}
          types={types}
          states={states}
          moves={edit.moves}
          setMoves={edit.setMoves}
          previewId={previewId}
          onPreview={setPreviewId}
        />
        {customer ? <CustomerSection {...props} /> : null}
        {customer ? <div className="xl:hidden">{preview}</div> : null}
      </SettingsPage>
      {customer ? (
        <Panel label="Customer form preview" ground className="hidden h-full w-[400px] shrink-0 xl:flex">
          <ScrollArea ground>
            <div className="px-5 pt-10 pb-16">{preview}</div>
          </ScrollArea>
        </Panel>
      ) : null}
    </div>
  );
}

export function TicketTypePage({ type: id }: { type: string }) {
  const workspace = useAccount().workspace.id;
  const { data: types, isPending } = useQuery(ticketTypesQuery(workspace));
  const type = types?.find((t) => t.id === id);

  if (isPending) {
    return <PageSkeleton />;
  }
  if (!type) {
    return (
      <Panel label="Ticket type" className="h-full flex-1">
        <EmptyState
          icon={RiCoupon3Line}
          title="This ticket type does not exist"
          description="It may belong to another workspace."
        />
      </Panel>
    );
  }

  return <Editor key={type.id} workspace={workspace} type={type} />;
}
