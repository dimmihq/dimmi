/*
 * The custom attributes the workspace's contacts and companies carry,
 * each with its type and when it last took a value. Keys the workspace
 * already sends are listed as seen in data; a type never changes once
 * set. New attribute, at the top, adds one in a modal. Archiving stops new values and keeps old ones; only an archived
 * attribute can be deleted, which removes its values. A contact
 * attribute can be opened to the messenger, which may otherwise write
 * none. An event can be opened to the messenger the same way, or the
 * messenger allowed to send any event. Those messenger switches wait in
 * one draft for the page's save bar; archiving, restoring and deleting
 * happen at once. Values that did not fit are listed at the bottom, so
 * nothing is dropped silently.
 */
import {
  RiAddLine,
  RiBuilding2Line,
  RiChat3Line,
  RiErrorWarningLine,
  RiFlashlightLine,
  RiUserLine,
} from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import {
  type AttributeDefinition,
  type AttributeOf,
  attributeProblemsQuery,
  attributesQuery,
  deleteAttribute,
  updateAttribute,
} from "@/lib/api/attributes";
import {
  type EventDefinition,
  eventSettingsQuery,
  eventsQuery,
  updateEvent,
  updateEventSettings,
} from "@/lib/api/events";
import { definitionLine, eventLine, problemNames } from "../attributes";
import { Action, Danger, Toggle } from "../controls";
import { Glyph, Item, Section, SettingsPage } from "../layout";
import { SaveBar } from "../save-bar";
import { failureText } from "../teams";
import { useAccount } from "../use-account";
import { NewAttributeModal } from "./new-attribute-modal";

const objects: Record<AttributeOf, string> = { contact: "Person", company: "Company" };

function useRefresh(workspace: string) {
  const queryClient = useQueryClient();

  return () => {
    void queryClient.invalidateQueries({ queryKey: attributesQuery(workspace).queryKey });
    void queryClient.invalidateQueries({ queryKey: attributeProblemsQuery(workspace).queryKey });
  };
}

/*
 * Writable is what the messenger may write, flipped but not saved yet:
 * per attribute, per event, and whether it may send any event.
 */
type Writable = { attributes: Record<string, boolean>; events: Record<string, boolean>; anyEvent?: boolean };

const nothing: Writable = { attributes: {}, events: {} };

/*
 * flipped drops a flip that puts a switch back where it was saved.
 */
function flipped(map: Record<string, boolean>, id: string, on: boolean, saved: boolean) {
  const { [id]: _, ...rest } = map;

  return on === saved ? rest : { ...rest, [id]: on };
}

/*
 * useWritableDraft holds the page's messenger switches until Save, which
 * sends every flip together and then reloads the lists; Discard puts
 * them back.
 */
function useWritableDraft(workspace: string) {
  const queryClient = useQueryClient();
  const refresh = useRefresh(workspace);
  const [pending, setPending] = useState<Writable>(nothing);
  const save = useMutation({
    mutationFn: async (sent: Writable) => {
      await Promise.all([
        ...Object.entries(sent.attributes).map(([id, on]) => updateAttribute(workspace, id, { client_writable: on })),
        ...Object.entries(sent.events).map(([id, on]) => updateEvent(workspace, id, { client_writable: on })),
        ...(sent.anyEvent === undefined
          ? []
          : [
              updateEventSettings(workspace, { messenger_new_events: sent.anyEvent }).then((saved) =>
                queryClient.setQueryData(eventSettingsQuery(workspace).queryKey, saved),
              ),
            ]),
      ]);
    },
    onSuccess: () => {
      setPending(nothing);
      toast.success("Attributes saved");
    },
    onSettled: () => {
      refresh();
      void queryClient.invalidateQueries({ queryKey: eventsQuery(workspace).queryKey });
      void queryClient.invalidateQueries({ queryKey: eventSettingsQuery(workspace).queryKey });
    },
  });

  return {
    pending,
    dirty:
      Object.keys(pending.attributes).length > 0 ||
      Object.keys(pending.events).length > 0 ||
      pending.anyEvent !== undefined,
    save,
    attribute: (definition: AttributeDefinition, on: boolean) =>
      setPending((p) => ({ ...p, attributes: flipped(p.attributes, definition.id, on, definition.client_writable) })),
    event: (event: EventDefinition, on: boolean) =>
      setPending((p) => ({ ...p, events: flipped(p.events, event.id, on, event.client_writable) })),
    anyEvent: (on: boolean, saved: boolean) =>
      setPending(({ anyEvent: _, ...rest }) => (on === saved ? rest : { ...rest, anyEvent: on })),
    discard: () => {
      setPending(nothing);
      save.reset();
    },
  };
}

type WritableDraft = ReturnType<typeof useWritableDraft>;

function DefinitionRow({
  workspace,
  definition,
  canManage,
  draft,
}: {
  workspace: string;
  definition: AttributeDefinition;
  canManage: boolean;
  draft: WritableDraft;
}) {
  const refresh = useRefresh(workspace);
  const [confirm, setConfirm] = useState(false);
  const change = useMutation({
    mutationFn: (changes: Parameters<typeof updateAttribute>[2]) => updateAttribute(workspace, definition.id, changes),
    onSuccess: refresh,
    onError: (error) => toast.error(error.message),
  });
  const remove = useMutation({ mutationFn: () => deleteAttribute(workspace, definition.id), onSuccess: refresh });
  const archived = definition.status === "archived";
  const writable = draft.pending.attributes[definition.id] ?? definition.client_writable;

  return (
    <Item
      mark={<Glyph icon={definition.of === "contact" ? RiUserLine : RiBuilding2Line} />}
      title={definition.label}
      detail={definitionLine(definition)}
    >
      {canManage && definition.of === "contact" && !archived ? (
        <span className="flex items-center gap-2">
          <span className="text-body-2-regular text-text-tertiary">Messenger</span>
          <Toggle
            key={`writable-${writable}`}
            label={`Messenger may write ${definition.label}`}
            on={writable}
            onChange={(on) => draft.attribute(definition, on)}
          />
        </span>
      ) : null}
      {canManage ? (
        <Action loading={change.isPending} onPress={() => change.mutate({ status: archived ? "active" : "archived" })}>
          {archived ? "Restore" : "Archive"}
        </Action>
      ) : null}
      {canManage && archived ? <Danger onPress={() => setConfirm(true)}>Delete</Danger> : null}
      <AlertDialog
        isOpen={confirm}
        onOpenChange={(open) => {
          setConfirm(open);
          if (!open) {
            remove.reset();
          }
        }}
        title={`Delete ${definition.label}?`}
        confirmLabel="Delete attribute"
        onConfirm={() => remove.mutateAsync()}
      >
        <p className="text-body-2-regular text-text-secondary">
          {remove.error
            ? remove.error.message
            : `Every ${objects[definition.of].toLowerCase()} loses its ${definition.key} value. This cannot be undone.`}
        </p>
      </AlertDialog>
    </Item>
  );
}

function EventRow({
  workspace,
  event,
  canManage,
  draft,
}: {
  workspace: string;
  event: EventDefinition;
  canManage: boolean;
  draft: WritableDraft;
}) {
  const refresh = useRefresh(workspace);
  const queryClient = useQueryClient();
  const archived = event.status === "archived";
  const change = useMutation({
    mutationFn: (changes: Parameters<typeof updateEvent>[2]) => updateEvent(workspace, event.id, changes),
    onSuccess: () => {
      refresh();
      void queryClient.invalidateQueries({ queryKey: eventsQuery(workspace).queryKey });
    },
    onError: (error) => toast.error(error.message),
  });
  const writable = draft.pending.events[event.id] ?? event.client_writable;

  return (
    <Item mark={<Glyph icon={RiFlashlightLine} />} title={event.label} detail={eventLine(event)}>
      {canManage && !archived ? (
        <span className="flex items-center gap-2">
          <span className="text-body-2-regular text-text-tertiary">Messenger</span>
          <Toggle
            key={`writable-${writable}`}
            label={`Messenger may send ${event.label}`}
            on={writable}
            onChange={(on) => draft.event(event, on)}
          />
        </span>
      ) : null}
      {canManage ? (
        <Action loading={change.isPending} onPress={() => change.mutate({ status: archived ? "active" : "archived" })}>
          {archived ? "Restore" : "Archive"}
        </Action>
      ) : null}
    </Item>
  );
}

/*
 * AnyMessengerEvent lets the messenger send event names nobody opened to
 * it, defining new ones as they arrive. The flip waits for the save bar.
 */
function AnyMessengerEvent({
  workspace,
  canManage,
  draft,
}: {
  workspace: string;
  canManage: boolean;
  draft: WritableDraft;
}) {
  const { data } = useQuery(eventSettingsQuery(workspace));
  const saved = data?.messenger_new_events ?? false;
  const on = draft.pending.anyEvent ?? saved;

  return (
    <Item
      mark={<Glyph icon={RiChat3Line} />}
      title="Accept any event from the messenger"
      detail="Off, the messenger may send only the events switched on below. On, it may send any name, and new ones are added here."
    >
      {canManage && data ? (
        <Toggle
          key={`any-${on}`}
          label="Accept any event from the messenger"
          on={on}
          onChange={(next) => draft.anyEvent(next, saved)}
        />
      ) : null}
    </Item>
  );
}

function Events({ workspace, canManage, draft }: { workspace: string; canManage: boolean; draft: WritableDraft }) {
  const { data, isPending } = useQuery(eventsQuery(workspace));

  return (
    <Section
      title="Events"
      description={
        'What customers did in your product, sent to POST /api/v1/contact-events or from the page with Dimmi("track"). ' +
        "Switched on, the messenger may send an event too, though anyone can forge it. Archived events take no new ones."
      }
      loading={isPending}
    >
      <AnyMessengerEvent workspace={workspace} canManage={canManage} draft={draft} />
      {data?.length === 0 ? (
        <Item
          mark={<Glyph icon={RiFlashlightLine} />}
          title="No events yet"
          detail="Send one from your backend and it shows up here."
        />
      ) : null}
      {(data ?? []).map((e) => (
        <EventRow key={e.id} workspace={workspace} event={e} canManage={canManage} draft={draft} />
      ))}
    </Section>
  );
}

function Problems({ workspace }: { workspace: string }) {
  const { data } = useQuery(attributeProblemsQuery(workspace));
  if (!data || data.length === 0) {
    return null;
  }

  return (
    <Section title="Dropped values" description="Values from the last 30 days that did not fit their attribute.">
      {data.map((p) => (
        <Item
          key={`${p.of}-${p.key}-${p.code}`}
          mark={<Glyph icon={RiErrorWarningLine} />}
          title={`${objects[p.of]} ${p.key}: ${problemNames[p.code]}`}
          detail={`${p.count === 1 ? "Once" : `${p.count} times`}${p.sample ? `, last one ${p.sample}` : ""}`}
        />
      ))}
    </Section>
  );
}

export function AttributesSettings() {
  const workspace = useAccount().workspace.id;
  const canManage = usePermission(workspace, "attributes:manage");
  const { data, isPending } = useQuery(attributesQuery(workspace));
  const [adding, setAdding] = useState(false);
  const of = (object: AttributeOf) => (data ?? []).filter((d) => d.of === object);
  const draft = useWritableDraft(workspace);
  useBlocker({
    shouldBlockFn: () => draft.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => draft.dirty,
  });

  return (
    <SettingsPage
      title="Attributes"
      description="The fields your contacts and companies carry, and their types."
      footer={
        <SaveBar
          dirty={draft.dirty}
          saving={draft.save.isPending}
          error={draft.save.error ? failureText(draft.save.error) : null}
          onSave={() => draft.save.mutate(draft.pending)}
          onDiscard={draft.discard}
        />
      }
      actions={
        canManage ? (
          <Button size="small" leadingIcon={RiAddLine} onClick={() => setAdding(true)}>
            New attribute
          </Button>
        ) : null
      }
    >
      {(["contact", "company"] as const).map((object) => (
        <Section
          key={object}
          title={object === "contact" ? "People" : "Companies"}
          description={
            object === "contact"
              ? "Switched on, the messenger may write the attribute too. Its value is then not trusted."
              : undefined
          }
          loading={isPending}
        >
          {of(object).length === 0 ? (
            <Item
              mark={<Glyph icon={object === "contact" ? RiUserLine : RiBuilding2Line} />}
              title="No attributes yet"
              detail="Send one through the API or add it with New attribute."
            />
          ) : null}
          {of(object).map((d) => (
            <DefinitionRow key={d.id} workspace={workspace} definition={d} canManage={canManage} draft={draft} />
          ))}
        </Section>
      ))}
      <Events workspace={workspace} canManage={canManage} draft={draft} />
      <Problems workspace={workspace} />
      {adding ? <NewAttributeModal workspace={workspace} onClose={() => setAdding(false)} /> : null}
    </SettingsPage>
  );
}
