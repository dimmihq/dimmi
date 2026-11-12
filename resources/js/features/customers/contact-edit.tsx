/*
 * Editing one customer in a modal opened from their row: their name,
 * email and phone, the tags they carry, the attributes the workspace
 * keeps on people and any others they have, and the team's notes. People
 * who look like the same person are listed first, to open and compare.
 * Everything is saved together on Save, and a field the server refuses
 * says why under it. Members who may only read see the same form without
 * a way to save.
 */
import { RiAddLine, RiCloseLine, RiUserUnfollowLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { Textarea } from "@/components/base/textarea/textarea";
import { TagChip } from "@/features/inbox/tags/tag-chip";
import { TagPicker } from "@/features/inbox/tags/tag-picker";
import { usePermission } from "@/features/settings/inboxes";
import { EmptyState } from "@/features/shell/page";
import { ProfileSkeleton } from "@/features/shell/table-skeleton";
import { type AttributeDefinition, attributesQuery } from "@/lib/api/attributes";
import { ApiError } from "@/lib/api/client";
import { type Contact, contactQuery, updateContact } from "@/lib/api/customers";
import { type Tag, tagsQuery } from "@/lib/api/tags";

import { Activity, Duplicates } from "./contact-edit-extras";
import { changesOf, definedOn, type EditState, editStateOf } from "./contact-edit-state";
import { personName, useWorkspace } from "./names";

const modalHeight = "h-[min(640px,calc(100dvh-3rem))]";

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section aria-label={title} className="flex flex-col gap-3">
      <h3 className="text-body-medium text-text-primary">{title}</h3>
      {children}
    </section>
  );
}

type Patch = (change: Partial<EditState>) => void;

function Details({ state, set, fields }: { state: EditState; set: Patch; fields: Record<string, string> }) {
  return (
    <Section title="Details">
      <Input
        label="Name"
        value={state.name}
        onChange={(name) => set({ name })}
        isInvalid={"name" in fields}
        errorMessage={fields.name}
      />
      <div className="flex flex-col gap-3">
        <Input
          label="Email"
          type="email"
          inputDir="ltr"
          value={state.email}
          onChange={(email) => set({ email })}
          isInvalid={"email" in fields}
          errorMessage={fields.email}
        />
        <Input
          label="Phone"
          type="tel"
          inputDir="ltr"
          value={state.phone}
          onChange={(phone) => set({ phone })}
          isInvalid={"phone" in fields}
          errorMessage={fields.phone}
        />
      </div>
    </Section>
  );
}

/*
 * pseudoTag stands in for a tag the member named that the workspace does
 * not have yet; Save creates it.
 */
function pseudoTag(name: string): Tag {
  return { id: `new:${name}`, object: "tag", name, color: null, icon: null };
}

function Tags({ state, set }: { state: EditState; set: Patch }) {
  const workspace = useWorkspace();
  const known = useQuery({ ...tagsQuery(workspace), enabled: workspace !== "" }).data ?? [];
  const add = (name: string) => {
    const tag = known.find((t) => t.name === name) ?? pseudoTag(name);
    if (!state.tags.some((t) => t.id === tag.id)) {
      set({ tags: [...state.tags, tag] });
    }
  };
  const remove = (tag: Tag) => set({ tags: state.tags.filter((t) => t.id !== tag.id) });

  return (
    <Section title="Tags">
      <div className="flex flex-wrap items-center gap-1.5">
        {state.tags.map((tag) => (
          <TagChip key={tag.id} tag={tag} onRemove={() => remove(tag)} />
        ))}
        <TagPicker workspace={workspace} selected={state.tags} onAdd={add} onRemove={remove} />
      </div>
    </Section>
  );
}

function ChoiceInput({
  label,
  value,
  choices,
  onChange,
}: {
  label: string;
  value: string;
  choices: [string, string][];
  onChange: (value: string) => void;
}) {
  return (
    <Select
      label={label}
      placeholder="Not set"
      selectedKey={value === "" ? "unset" : value}
      onSelectionChange={(key) => onChange(key === "unset" || key === null ? "" : String(key))}
    >
      {[["unset", "Not set"] as [string, string], ...choices].map(([id, name]) => (
        <SelectItem key={id} id={id} textValue={name}>
          {name}
        </SelectItem>
      ))}
    </Select>
  );
}

function TypedInput({
  definition,
  value,
  onChange,
}: {
  definition: AttributeDefinition;
  value: string;
  onChange: (value: string) => void;
}) {
  const label = definition.label || definition.key;
  if (definition.data_type === "boolean") {
    const yesNo: [string, string][] = [
      ["true", "Yes"],
      ["false", "No"],
    ];

    return <ChoiceInput label={label} value={value} choices={yesNo} onChange={onChange} />;
  }
  if (definition.data_type === "list") {
    const options = definition.options.map((o): [string, string] => [o, o]);

    return <ChoiceInput label={label} value={value} choices={options} onChange={onChange} />;
  }
  const types = { number: "number", date: "date", string: "text" } as const;

  return <Input label={label} type={types[definition.data_type]} value={value} onChange={onChange} />;
}

function Attributes({ state, set, defined }: { state: EditState; set: Patch; defined: AttributeDefinition[] }) {
  const row = (id: string, change: Partial<EditState["extra"][number]>) =>
    set({ extra: state.extra.map((r) => (r.id === id ? { ...r, ...change } : r)) });

  return (
    <Section title="Attributes">
      {defined.length > 0 ? (
        <div className="flex flex-col gap-3">
          {defined.map((d) => (
            <TypedInput
              key={d.key}
              definition={d}
              value={state.typed[d.key] ?? ""}
              onChange={(value) => set({ typed: { ...state.typed, [d.key]: value } })}
            />
          ))}
        </div>
      ) : null}
      {state.extra.map((r) => (
        <div
          key={r.id}
          className="flex flex-col gap-2 rounded-2xl border border-border-button-default bg-background-secondary-default p-3"
        >
          <div className="flex items-center justify-between gap-2">
            <span className="truncate text-body-2-medium text-text-secondary">{r.key || "New attribute"}</span>
            <IconButton
              size="small"
              icon={RiCloseLine}
              aria-label={`Remove ${r.key || "attribute"}`}
              onClick={() => set({ extra: state.extra.filter((other) => other.id !== r.id) })}
            />
          </div>
          <Input
            aria-label="Attribute name"
            inputDir="ltr"
            placeholder="plan"
            value={r.key}
            onChange={(key) => row(r.id, { key })}
          />
          <Input
            aria-label={`Value of ${r.key || "attribute"}`}
            placeholder="pro"
            value={r.value}
            onChange={(value) => row(r.id, { value })}
          />
        </div>
      ))}
      <Button
        variant="secondary"
        size="small"
        leadingIcon={RiAddLine}
        className="self-start"
        onClick={() => set({ extra: [...state.extra, { id: crypto.randomUUID(), key: "", value: "" }] })}
      >
        Add attribute
      </Button>
    </Section>
  );
}

function EditForm({
  contact,
  defined,
  onClose,
}: {
  contact: Contact;
  defined: AttributeDefinition[];
  onClose: () => void;
}) {
  const workspace = useWorkspace();
  const client = useQueryClient();
  const canWrite = usePermission(workspace, "contacts:write");
  const [state, setState] = useState(() => editStateOf(contact, defined));
  const set: Patch = (change) => setState((before) => ({ ...before, ...change }));
  const changes = changesOf(contact, defined, state);
  const save = useMutation({
    mutationFn: () => updateContact(workspace, contact.id, changes),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: ["workspace", workspace] });
      onClose();
    },
  });
  const fields = save.error instanceof ApiError ? save.error.fields : {};
  const refused = save.error && Object.keys(fields).length === 0 ? save.error.message : null;

  return (
    <>
      <ModalBody className="pb-4">
        <fieldset disabled={!canWrite} className="flex min-w-0 flex-col gap-6">
          <Duplicates contact={contact} canMerge={canWrite} />
          <Details state={state} set={set} fields={fields} />
          <Tags state={state} set={set} />
          <Attributes state={state} set={set} defined={defined} />
          <Section title="Notes">
            <Textarea
              aria-label="Notes"
              placeholder="Add a note only the team sees"
              rows={3}
              maxLength={10000}
              value={state.notes}
              onChange={(notes) => set({ notes })}
            />
          </Section>
          <Activity contact={contact} />
        </fieldset>
        {refused ? <p className="pt-3 text-body-2-regular text-text-error-primary">{refused}</p> : null}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        {canWrite ? (
          <Button loading={save.isPending} disabled={Object.keys(changes).length === 0} onClick={() => save.mutate()}>
            Save
          </Button>
        ) : null}
      </ModalFooter>
    </>
  );
}

/*
 * ContactEditModal edits the customer with the given id, loading them
 * fresh so the form starts from what the server holds now.
 */
export function ContactEditModal({ id, onClose }: { id: string; onClose: () => void }) {
  const workspace = useWorkspace();
  const contact = useQuery({ ...contactQuery(workspace, id), retry: false, enabled: workspace !== "" });
  const attributes = useQuery({ ...attributesQuery(workspace), enabled: workspace !== "" });
  const ready = contact.data && attributes.data;

  return (
    <Modal size="md" className={modalHeight} isOpen onOpenChange={(open) => (open ? undefined : onClose())}>
      <ModalHeader
        title={contact.data ? `Edit ${personName(contact.data)}` : "Edit customer"}
        description={contact.data ? subtitleOf(contact.data) : undefined}
      />
      {ready ? (
        <EditForm
          key={contact.data.updated_at}
          contact={contact.data}
          defined={definedOn(attributes.data)}
          onClose={onClose}
        />
      ) : (
        <ModalBody className="pb-4">
          {contact.isError ? (
            <EmptyState
              icon={RiUserUnfollowLine}
              title="Customer not found"
              description="They may have been removed, or they belong to another workspace."
            />
          ) : (
            <ProfileSkeleton />
          )}
        </ModalBody>
      )}
    </Modal>
  );
}

function subtitleOf(contact: Contact): string {
  const kind = contact.kind === "user" ? "User" : "Lead";

  return contact.company ? `${kind} at ${contact.company.name}` : kind;
}
