/*
 * One ticket type's editor. Its settings are a draft saved in one go
 * from a bar that floats over the page while anything changed; leaving
 * with unsaved changes asks first. Fields save on their own, as they are
 * added, edited, archived or dragged. Beside the editor, on wide
 * screens, the real messenger shows the type as customers meet it: its
 * form, or a request in the state picked in its states, live as the type
 * changes. Back-office types and trackers have no customer form, so
 * their editor shows no customer settings and no preview.
 */
import { RiArrowLeftLine, RiCoupon3Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import { PageSkeleton, SettingsPage } from "@/features/settings/layout";
import { SaveBar } from "@/features/settings/save-bar";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { inboxesQuery } from "@/lib/api/inboxes";
import { teamsQuery } from "@/lib/api/teams";
import { type TicketType, ticketKeys, ticketStatesQuery, ticketTypesQuery, updateTicketType } from "@/lib/api/tickets";

import { FieldBuilder } from "./field-builder";
import { draftOf, type TypeDraft, typeChanges } from "./form";
import { MessengerPreview, type PreviewView } from "./messenger-preview";
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

function Editor({ workspace, type }: { workspace: string; type: TicketType }) {
  const edit = useTypeDraft(workspace, type);
  const locked = !usePermission(workspace, "settings:write");
  const { data: states = [] } = useQuery(ticketStatesQuery(workspace));
  const { data: types = [] } = useQuery(ticketTypesQuery(workspace));
  const [previewId, setPreviewId] = useState<string | null>(null);
  const [view, setView] = useState<PreviewView>("form");
  const previewState = states.find((s) => s.id === (previewId ?? edit.draft.default_state_id));
  const { data: inboxes = [] } = useQuery({ ...inboxesQuery(workspace), enabled: workspace !== "" });
  const { data: teams = [] } = useQuery({ ...teamsQuery(workspace), enabled: workspace !== "" });
  useBlocker({
    shouldBlockFn: () => edit.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => edit.dirty,
  });
  const props = { draft: edit.draft, set: edit.set, locked };
  const customer = type.kind === "customer";
  return (
    <div className="flex h-full min-w-0 flex-1 gap-3">
      <SettingsPage
        title={type.name}
        description={type.archived_at ? "Archived. Old tickets keep this type; new ones can't use it." : undefined}
        footer={
          <SaveBar
            dirty={edit.dirty}
            saving={edit.save.isPending}
            error={edit.save.error?.message}
            onSave={() => edit.save.mutate()}
            onDiscard={edit.discard}
          />
        }
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
          onPreview={(id) => {
            setPreviewId(id);
            setView("messages");
          }}
        />
        {customer ? <CustomerSection {...props} /> : null}
      </SettingsPage>
      {customer ? (
        <Panel label="Customer preview" className="hidden h-full w-[500px] shrink-0 xl:flex">
          <MessengerPreview
            workspace={workspace}
            inbox={edit.draft.inbox_id}
            typeId={type.id}
            name={edit.draft.name}
            description={edit.draft.description}
            icon={edit.draft.icon}
            fields={type.fields}
            state={previewState}
            view={view}
            onView={setView}
          />
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
