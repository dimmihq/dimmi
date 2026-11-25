/*
 * MadeUpPanel is the add menu's view for a made-up customer: a drawn
 * name and email that can be drawn again, whether they are signed in,
 * and the attributes the workspace defines for customers and for their
 * company, each set or left empty. A choice from a fixed list is picked
 * from a dropdown. Using it puts the customer in the composer.
 */
import { RiArrowLeftSLine, RiShuffleLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Avatar } from "@/components/base/avatar/avatar";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { avatarColor } from "@/features/shell/avatar-color";
import { type AttributeDefinition, attributesQuery } from "@/lib/api/attributes";
import type { SimPersona } from "@/lib/api/procedures";

import { madeUpPerson } from "./made-up";

type Values = Record<string, string>;

/* The choice that leaves an attribute unset. */
const none = "__none";

/*
 * typed turns a value as typed into the attribute's own type, or leaves
 * it out when empty.
 */
function typed(def: AttributeDefinition, raw: string): unknown {
  if (def.data_type === "number") {
    const n = Number(raw);
    return Number.isFinite(n) ? n : undefined;
  }
  if (def.data_type === "boolean") {
    return raw === "true";
  }

  return raw;
}

function Field({
  def,
  value,
  onChange,
}: {
  def: AttributeDefinition;
  value: string;
  onChange: (value: string) => void;
}) {
  if (def.data_type === "boolean") {
    return (
      <Switch
        size="sm"
        aria-label={def.label || def.key}
        isSelected={value === "true"}
        onChange={(on) => onChange(on ? "true" : "")}
      />
    );
  }
  if (def.options.length > 0) {
    return (
      <Select
        aria-label={def.label || def.key}
        size="sm"
        placeholder="Not set"
        className="w-32"
        selectedKey={value || none}
        onSelectionChange={(key) => onChange(key === none || key === null ? "" : String(key))}
        popoverClassName="w-40"
      >
        <SelectItem id={none} textValue="Not set">
          Not set
        </SelectItem>
        {def.options.map((option) => (
          <SelectItem key={option} id={option} textValue={option}>
            {option}
          </SelectItem>
        ))}
      </Select>
    );
  }

  return (
    <Input
      aria-label={def.label || def.key}
      size="small"
      className="w-32"
      fieldClassName="bg-background-secondary-default"
      inputMode={def.data_type === "number" ? "decimal" : undefined}
      placeholder="Not set"
      value={value}
      onChange={onChange}
    />
  );
}

export function MadeUpPanel({
  workspace,
  onBack,
  onPick,
}: {
  workspace: string;
  onBack: () => void;
  onPick: (persona: SimPersona) => void;
}) {
  const [person, setPerson] = useState(madeUpPerson);
  const [signedIn, setSignedIn] = useState(true);
  const [values, setValues] = useState<Values>({});
  const defs = (useQuery({ ...attributesQuery(workspace), retry: false }).data ?? []).filter(
    (d) => d.status === "active",
  );
  const contact = defs.filter((d) => d.of === "contact");
  const company = defs.filter((d) => d.of === "company");
  const name = person.name ?? "";

  const set = (list: AttributeDefinition[]) => {
    const out: Record<string, unknown> = {};
    for (const def of list) {
      const raw = values[def.id]?.trim() ?? "";
      const value = raw === "" ? undefined : typed(def, raw);
      if (value !== undefined) {
        out[def.key] = value;
      }
    }

    return out;
  };
  const use = () => {
    const firm = set(company);
    onPick({
      ...person,
      verified: signedIn,
      channel: "widget",
      attributes: set(contact),
      ...(Object.keys(firm).length > 0 ? { company: firm } : {}),
    });
  };
  const rows = (list: AttributeDefinition[]) =>
    list.map((def) => (
      <div key={def.id} className="flex min-h-10 items-center justify-between gap-3 px-1">
        <span className="min-w-0 truncate text-body-2-regular text-text-primary">{def.label || def.key}</span>
        <Field
          def={def}
          value={values[def.id] ?? ""}
          onChange={(value) => setValues((was) => ({ ...was, [def.id]: value }))}
        />
      </div>
    ));

  return (
    <div className="flex h-full min-h-0 flex-col gap-3">
      <div className="flex shrink-0 items-center gap-1">
        <button
          type="button"
          aria-label="Back"
          onClick={onBack}
          className="flex size-7 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors hover:bg-background-primary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
        >
          <RiArrowLeftSLine aria-hidden className="size-5" />
        </button>
        <span className="text-body-medium text-text-primary">Made-up customer</span>
      </div>
      <div className="flex shrink-0 items-center gap-2.5 px-1">
        <Avatar size="md" color={avatarColor(name)} initials={name.charAt(0)} />
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-body-medium text-text-primary">{name}</span>
          <span className="truncate text-body-2-regular text-text-secondary">{person.email}</span>
        </span>
        <IconButton
          variant="quiet"
          size="small"
          icon={RiShuffleLine}
          aria-label="Draw another"
          onClick={() => setPerson(madeUpPerson())}
        />
      </div>
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <div className="flex min-h-10 items-center justify-between gap-3 px-1">
          <span className="text-body-2-regular text-text-primary">Signed in</span>
          <Switch size="sm" aria-label="Signed in" isSelected={signedIn} onChange={setSignedIn} />
        </div>
        {rows(contact)}
        {company.length > 0 ? (
          <>
            <span className="px-1 pt-3 pb-1 text-body-2-medium text-text-secondary">Company</span>
            {rows(company)}
          </>
        ) : null}
      </div>
      <Button size="small" className="shrink-0" onClick={use}>
        Test as this customer
      </Button>
    </div>
  );
}
