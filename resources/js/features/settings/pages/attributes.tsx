/*
 * The custom attributes the workspace's contacts and companies carry,
 * each with its type and when it last took a value. Keys the workspace
 * already sends are listed as seen in data; a type never changes once
 * set. Archiving stops new values and keeps old ones; only an archived
 * attribute can be deleted, which removes its values. A contact
 * attribute can be opened to the messenger, which may otherwise write
 * none. Values that did not fit are listed at the bottom, so nothing is
 * dropped silently.
 */
import { RiBuilding2Line, RiErrorWarningLine, RiUserLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { AlertDialog } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import { usePermission } from "@/features/settings/inboxes";
import {
  type AttributeDefinition,
  type AttributeOf,
  type AttributeType,
  attributeProblemsQuery,
  attributesQuery,
  defineAttribute,
  deleteAttribute,
  updateAttribute,
} from "@/lib/api/attributes";
import { definitionLine, problemNames, typeNames } from "../attributes";
import { Action, Choice, Danger, Field, Toggle } from "../controls";
import { Glyph, Item, Row, Section, SettingsPage } from "../layout";
import { useAccount } from "../use-account";

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

function NewAttribute({ workspace }: { workspace: string }) {
  const refresh = useRefresh(workspace);
  const [of, setOf] = useState<AttributeOf>("contact");
  const [key, setKey] = useState("");
  const [label, setLabel] = useState("");
  const [type, setType] = useState<AttributeType>("string");
  const [options, setOptions] = useState("");
  const define = useMutation({
    mutationFn: () =>
      defineAttribute(workspace, {
        object: of,
        key: key.trim(),
        label: label.trim() || key.trim(),
        data_type: type,
        options:
          type === "list"
            ? options
                .split(",")
                .map((o) => o.trim())
                .filter(Boolean)
            : undefined,
      }),
    onSuccess: () => {
      setKey("");
      setLabel("");
      setOptions("");
      refresh();
    },
  });
  const typeEntries = Object.entries(typeNames) as [AttributeType, string][];

  return (
    <Section title="Add an attribute" description="Keys are how your code sends the value: plan, seats, trial_ends_at.">
      <Row label="Belongs to">
        <Choice
          label="Belongs to"
          options={Object.values(objects)}
          value={objects[of]}
          onChange={(name) => setOf(name === objects.company ? "company" : "contact")}
        />
      </Row>
      <Row label="Key">
        <Field label="Key" placeholder="plan" value={key} onChange={setKey} />
      </Row>
      <Row label="Label">
        <Field label="Label" placeholder="Plan" value={label} onChange={setLabel} />
      </Row>
      <Row label="Type" description="A type never changes once set.">
        <Choice
          label="Type"
          options={typeEntries.map(([, name]) => name)}
          value={typeNames[type]}
          onChange={(name) => setType(typeEntries.find(([, n]) => n === name)?.[0] ?? "string")}
        />
      </Row>
      {type === "list" ? (
        <Row label="Values" description="Separate them with commas.">
          <Field label="List values" placeholder="free, pro, team" value={options} onChange={setOptions} />
        </Row>
      ) : null}
      <Row label={define.error ? define.error.message : "New attribute"}>
        <Action loading={define.isPending} onPress={() => key.trim() && define.mutate()}>
          Add attribute
        </Action>
      </Row>
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
  const of = (object: AttributeOf) => (data ?? []).filter((d) => d.of === object);

  return (
    <SettingsPage title="Attributes" description="The fields your contacts and companies carry, and their types.">
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
              detail="Send one through the API or add it below."
            />
          ) : null}
          {of(object).map((d) => (
            <DefinitionRow key={d.id} workspace={workspace} definition={d} canManage={canManage} />
          ))}
        </Section>
      ))}
      {canManage ? <NewAttribute workspace={workspace} /> : null}
      <Problems workspace={workspace} />
    </SettingsPage>
  );
}
