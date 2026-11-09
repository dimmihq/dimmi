/*
 * The custom attributes the workspace's contacts and companies carry,
 * each with its type and when it last took a value. Keys the workspace
 * already sends are listed as seen in data; a type never changes once
 * set. New attribute, at the top, adds one in a modal. Archiving stops new values and keeps old ones; only an archived
 * attribute can be deleted, which removes its values. A contact
 * attribute can be opened to the messenger, which may otherwise write
 * none. Values that did not fit are listed at the bottom, so nothing is
 * dropped silently.
 */
import { RiAddLine, RiBuilding2Line, RiErrorWarningLine, RiFlashlightLine, RiUserLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
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
import { type EventDefinition, eventsQuery, updateEvent } from "@/lib/api/events";
import { definitionLine, eventLine, problemNames } from "../attributes";
import { Action, Danger, Toggle } from "../controls";
import { Glyph, Item, Section, SettingsPage } from "../layout";
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

function DefinitionRow({
  workspace,
  definition,
  canManage,
}: {
  workspace: string;
  definition: AttributeDefinition;
  canManage: boolean;
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
            label={`Messenger may write ${definition.label}`}
            on={definition.client_writable}
            onChange={(on) => change.mutate({ client_writable: on })}
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

function EventRow({ workspace, event, canManage }: { workspace: string; event: EventDefinition; canManage: boolean }) {
  const refresh = useRefresh(workspace);
  const queryClient = useQueryClient();
  const archived = event.status === "archived";
  const change = useMutation({
    mutationFn: () => updateEvent(workspace, event.id, { status: archived ? "active" : "archived" }),
    onSuccess: () => {
      refresh();
      void queryClient.invalidateQueries({ queryKey: eventsQuery(workspace).queryKey });
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <Item mark={<Glyph icon={RiFlashlightLine} />} title={event.label} detail={eventLine(event)}>
      {canManage ? (
        <Action loading={change.isPending} onPress={() => change.mutate()}>
          {archived ? "Restore" : "Archive"}
        </Action>
      ) : null}
    </Item>
  );
}

function Events({ workspace, canManage }: { workspace: string; canManage: boolean }) {
  const { data, isPending } = useQuery(eventsQuery(workspace));

  return (
    <Section
      title="Events"
      description="What customers did in your product, sent to POST /api/v1/contact-events. Archived events take no new ones."
      loading={isPending}
    >
      {data?.length === 0 ? (
        <Item
          mark={<Glyph icon={RiFlashlightLine} />}
          title="No events yet"
          detail="Send one from your backend and it shows up here."
        />
      ) : null}
      {(data ?? []).map((e) => (
        <EventRow key={e.id} workspace={workspace} event={e} canManage={canManage} />
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

  return (
    <SettingsPage
      title="Attributes"
      description="The fields your contacts and companies carry, and their types."
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
            <DefinitionRow key={d.id} workspace={workspace} definition={d} canManage={canManage} />
          ))}
        </Section>
      ))}
      <Events workspace={workspace} canManage={canManage} />
      <Problems workspace={workspace} />
      {adding ? <NewAttributeModal workspace={workspace} onClose={() => setAdding(false)} /> : null}
    </SettingsPage>
  );
}
