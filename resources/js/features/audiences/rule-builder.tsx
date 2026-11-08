/*
 * The rule builder: who an audience starts from, then conditions and
 * groups of conditions, each read as a sentence. Only fields a saved
 * audience may read are offered, grouped as people think of them; a
 * condition the server refused is marked where it is. A rule deeper than
 * the builder shows, written through the API, is described and left
 * alone.
 */
import { RiAddLine, RiCloseLine, RiFolderAddLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Select, SelectItem, SelectSection } from "@/components/base/select/select";
import type { AudienceField, Condition, Visitors } from "@/lib/api/audiences";

import {
  type ConditionItem,
  type Form,
  fieldGroup,
  fieldGroups,
  fieldName,
  type Item,
  type Match,
  newId,
  opNames,
  visitorChoices,
  withField,
  withOp,
  withWindow,
} from "./rules";
import { type Choice, ValueInput, WindowInput } from "./value-input";

/*
 * Lookups are the named things conditions pick from: tags and saved
 * audiences by id.
 */
export interface Lookups {
  tags: Choice[];
  audiences: Choice[];
}

const matchChoices: [Match, string][] = [
  ["all", "all"],
  ["any", "any"],
  ["none", "none"],
];

function choicesFor(field: AudienceField | undefined, lookups: Lookups): Choice[] | undefined {
  if (!field) {
    return undefined;
  }
  if (field.type === "set") {
    return lookups.tags;
  }
  if (field.type === "saved_audience") {
    return lookups.audiences;
  }
  if (field.options) {
    return field.options.map((o) => [o, o.charAt(0).toUpperCase() + o.slice(1)]);
  }

  return undefined;
}

function MatchPick({ label, value, onChange }: { label: string; value: Match; onChange: (m: Match) => void }) {
  return (
    <Select
      aria-label={label}
      size="sm"
      className="w-24"
      selectedKey={value}
      onSelectionChange={(key) => key !== null && onChange(key as Match)}
    >
      {matchChoices.map(([id, name]) => (
        <SelectItem key={id} id={id}>
          {name}
        </SelectItem>
      ))}
    </Select>
  );
}

function FieldPick({
  fields,
  value,
  onChange,
}: {
  fields: AudienceField[];
  value: string;
  onChange: (f: AudienceField) => void;
}) {
  return (
    <Select
      aria-label="Field"
      size="sm"
      className="w-44"
      popoverClassName="w-64"
      selectedKey={value || null}
      placeholder="Pick a field"
      onSelectionChange={(key) => {
        const picked = fields.find((f) => f.key === key);
        if (picked) {
          onChange(picked);
        }
      }}
    >
      {fieldGroups.map((group) => {
        const inGroup = fields
          .filter((f) => fieldGroup(f.key) === group)
          .sort((a, b) => (a.label ?? fieldName(a.key)).localeCompare(b.label ?? fieldName(b.key)));
        return inGroup.length === 0 ? null : (
          <SelectSection key={group} title={group}>
            {inGroup.map((f) => (
              <SelectItem key={f.key} id={f.key} textValue={f.label ?? fieldName(f.key)}>
                {f.label ?? fieldName(f.key)}
              </SelectItem>
            ))}
          </SelectSection>
        );
      })}
    </Select>
  );
}

/*
 * blank reports whether a condition's value is still empty, so a
 * problem with it waits until the person tries to save.
 */
function blank(value: unknown): boolean {
  return value === "" || (Array.isArray(value) && value.length === 0);
}

function ConditionRow({
  item,
  fields,
  lookups,
  problem,
  quiet,
  onChange,
  onRemove,
}: {
  item: ConditionItem;
  fields: AudienceField[];
  lookups: Lookups;
  problem: string | undefined;
  quiet: boolean;
  onChange: (c: Condition) => void;
  onRemove: () => void;
}) {
  const { condition } = item;
  const field = fields.find((f) => f.key === condition.field);
  const ops = field?.ops ?? [condition.op];

  return (
    <li className="flex flex-col gap-1">
      <div className="flex flex-wrap items-center gap-2">
        <FieldPick fields={fields} value={condition.field} onChange={(f) => onChange(withField(f))} />
        <Select
          aria-label="Comparison"
          size="sm"
          className="w-40"
          selectedKey={condition.op}
          onSelectionChange={(key) => key !== null && onChange(withOp(condition, String(key), field?.type))}
        >
          {ops.map((op) => (
            <SelectItem key={op} id={op}>
              {opNames[op] ?? op}
            </SelectItem>
          ))}
        </Select>
        <ValueInput
          key={`${condition.field}-${condition.op}`}
          field={field}
          condition={condition}
          choices={choicesFor(field, lookups)}
          onChange={(value) => onChange({ ...condition, value })}
        />
        {field?.window ? (
          <WindowInput days={condition.within_days} onChange={(days) => onChange(withWindow(condition, days))} />
        ) : null}
        <IconButton size="small" icon={RiCloseLine} aria-label="Remove condition" onClick={onRemove} />
      </div>
      {field?.where === "client" ? (
        <p className="text-caption-1-regular text-text-tertiary">Checked in the browser, so its value is public.</p>
      ) : null}
      {problem && !(quiet && blank(condition.value)) ? (
        <p className="text-body-2-regular text-text-error-primary">{problem}</p>
      ) : null}
    </li>
  );
}

/*
 * starter is the condition a new row starts with.
 */
function starter(fields: AudienceField[]): ConditionItem {
  const first = fields.find((f) => f.key === "contact.email_domain") ?? fields[0];
  const condition: Condition = first ? withField(first) : { field: "contact.kind", op: "eq", value: "user" };

  return { kind: "condition", id: newId(), condition };
}

function Conditions({
  items,
  fields,
  lookups,
  problems,
  quiet,
  pathOf,
  onChange,
}: {
  items: ConditionItem[];
  fields: AudienceField[];
  lookups: Lookups;
  problems: Record<string, string>;
  quiet: boolean;
  pathOf: (index: number) => string;
  onChange: (items: ConditionItem[]) => void;
}) {
  return (
    <ul className="flex flex-col gap-3">
      {items.map((item, index) => (
        <ConditionRow
          key={item.id}
          item={item}
          fields={fields}
          lookups={lookups}
          problem={problems[pathOf(index)]}
          quiet={quiet}
          onChange={(condition) => onChange(items.map((i) => (i.id === item.id ? { ...item, condition } : i)))}
          onRemove={() => onChange(items.filter((i) => i.id !== item.id))}
        />
      ))}
    </ul>
  );
}

/*
 * pathKey is the server's path of a top-level item or a condition in a
 * group, so a refused condition is marked where it is.
 */
function pathKey(match: Match, index: number, inner?: { match: Match; index: number }): string {
  const step = (m: Match, i: number) => (m === "none" ? `.not.any[${i}]` : `.${m}[${i}]`);
  return `rules${step(match, index)}${inner ? step(inner.match, inner.index) : ""}`;
}

export function RuleBuilder({
  form,
  fields,
  lookups,
  problems = {},
  quiet = true,
  onChange,
}: {
  form: Form;
  fields: AudienceField[];
  lookups: Lookups;
  problems?: Record<string, string>;
  quiet?: boolean;
  onChange: (form: Form) => void;
}) {
  const setItems = (items: Item[]) => onChange({ ...form, items });
  const replace = (item: Item) => setItems(form.items.map((i) => (i.id === item.id ? item : i)));

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-body-regular text-text-primary">Who</span>
        <Select
          aria-label="Who"
          size="sm"
          className="w-48"
          selectedKey={form.visitors}
          onSelectionChange={(key) => key !== null && onChange({ ...form, visitors: key as Visitors })}
        >
          {visitorChoices.map(([id, name]) => (
            <SelectItem key={id} id={id}>
              {name}
            </SelectItem>
          ))}
        </Select>
      </div>
      {form.visitors === "anonymous" || form.visitors === "anonymous_and_leads" ? (
        <p className="-mt-3 text-body-2-regular text-text-secondary">
          Visitors have no profile: only page, device and language conditions apply to them.
        </p>
      ) : null}
      {form.items.length > 0 ? (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-body-regular text-text-primary">Matching</span>
          <MatchPick label="Match" value={form.match} onChange={(match) => onChange({ ...form, match })} />
          <span className="text-body-regular text-text-primary">of these</span>
        </div>
      ) : null}
      <ul className="flex flex-col gap-3">
        {form.items.map((item, index) =>
          item.kind === "condition" ? (
            <ConditionRow
              key={item.id}
              item={item}
              fields={fields}
              lookups={lookups}
              problem={problems[pathKey(form.match, index)]}
              quiet={quiet}
              onChange={(condition) => replace({ ...item, condition })}
              onRemove={() => setItems(form.items.filter((i) => i.id !== item.id))}
            />
          ) : (
            <li key={item.id} className="flex flex-col gap-3 rounded-xl border border-separator-border p-3">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-body-2-medium text-text-secondary">Group matching</span>
                <MatchPick label="Group match" value={item.match} onChange={(match) => replace({ ...item, match })} />
                <span className="flex-1" />
                <IconButton
                  size="small"
                  icon={RiCloseLine}
                  aria-label="Remove group"
                  onClick={() => setItems(form.items.filter((i) => i.id !== item.id))}
                />
              </div>
              <Conditions
                items={item.conditions}
                fields={fields}
                lookups={lookups}
                problems={problems}
                quiet={quiet}
                pathOf={(inner) => pathKey(form.match, index, { match: item.match, index: inner })}
                onChange={(conditions) => replace({ ...item, conditions })}
              />
              <div>
                <Button
                  variant="secondary"
                  size="small"
                  leadingIcon={RiAddLine}
                  onClick={() => replace({ ...item, conditions: [...item.conditions, starter(fields)] })}
                >
                  Add to group
                </Button>
              </div>
            </li>
          ),
        )}
      </ul>
      <div className="flex flex-wrap gap-2">
        <Button
          variant="secondary"
          size="small"
          leadingIcon={RiAddLine}
          onClick={() => setItems([...form.items, starter(fields)])}
        >
          Add condition
        </Button>
        <Button
          variant="secondary"
          size="small"
          leadingIcon={RiFolderAddLine}
          onClick={() =>
            setItems([...form.items, { kind: "group", id: newId(), match: "any", conditions: [starter(fields)] }])
          }
        >
          Add group
        </Button>
      </div>
    </div>
  );
}
