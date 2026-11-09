/*
 * An audience's rule, edited in place: who it starts from, then groups
 * of conditions, each a card. A card matches all, any or none of its
 * conditions; when there are several cards, people match all or any of
 * them. Every condition is one row of fixed columns (field, comparison,
 * value, remove), so choosing a field or a comparison never moves the
 * page: the comparison waits disabled until a field is chosen, and the
 * value keeps its place even when a comparison needs none. Only an event
 * count adds a line under its row, for the days it counts over. Words in the
 * gutter say how rows and cards join. A row still missing its field is
 * kept on screen and left out of the rule.
 */
import { RiAddLine, RiCloseLine, RiDeleteBinLine } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Input } from "@/components/base/input/input";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Select, SelectItem, SelectSection } from "@/components/base/select/select";
import type { AudienceField, Condition } from "@/lib/api/audiences";

import {
  type Form,
  fieldGroup,
  fieldGroups,
  fieldName,
  type Match,
  newId,
  opNames,
  shapeOf,
  typeWords,
  visitorChoices,
  withField,
  withOp,
  withWindow,
} from "./rules";
import { type Choice, ValueInput, WindowInput } from "./value-input";

/*
 * Lookups names the tags and saved audiences a condition can pick.
 */
export interface Lookups {
  tags: Choice[];
  audiences: Choice[];
}

interface Row {
  id: string;
  condition: Condition | null;
}

interface Card {
  id: string;
  match: Match;
  rows: Row[];
}

interface State {
  visitors: Form["visitors"];
  top: Match;
  cards: Card[];
}

const matchChoices: { id: Match; label: string }[] = [
  { id: "all", label: "All" },
  { id: "any", label: "Any" },
  { id: "none", label: "None" },
];

const joins: Record<Match, string> = { all: "and", any: "or", none: "or" };

function emptyCard(): Card {
  return { id: newId(), match: "all", rows: [{ id: newId(), condition: null }] };
}

/*
 * fromForm lays a rule out as cards: loose conditions become the first
 * card, each group its own.
 */
function fromForm(form: Form): State {
  const loose: Row[] = [];
  const cards: Card[] = [];
  for (const item of form.items) {
    if (item.kind === "condition") {
      loose.push({ id: item.id, condition: item.condition });
    } else {
      cards.push({
        id: item.id,
        match: item.match,
        rows: item.conditions.map((c) => ({ id: c.id, condition: c.condition })),
      });
    }
  }
  if (cards.length === 0) {
    return {
      visitors: form.visitors,
      top: "all",
      cards: [loose.length > 0 ? { id: newId(), match: form.match, rows: loose } : emptyCard()],
    };
  }
  if (loose.length > 0) {
    cards.unshift({ id: newId(), match: form.match === "all" ? "all" : "any", rows: loose });
  }

  return { visitors: form.visitors, top: form.match, cards };
}

/*
 * toForm writes the cards back as a rule: one card is the rule itself,
 * several are groups joined all, any or none. Rows without a field and
 * empty cards are left out.
 */
function toForm(state: State): Form {
  const cards = state.cards
    .map((card) => ({
      ...card,
      rows: card.rows.filter((r): r is { id: string; condition: Condition } => r.condition !== null),
    }))
    .filter((card) => card.rows.length > 0);
  const conditions = (card: (typeof cards)[number]) =>
    card.rows.map((r) => ({ kind: "condition" as const, id: r.id, condition: r.condition }));
  if (cards.length === 0) {
    return { visitors: state.visitors, match: "all", items: [] };
  }
  if (cards.length === 1 && cards[0]) {
    return { visitors: state.visitors, match: cards[0].match, items: conditions(cards[0]) };
  }

  return {
    visitors: state.visitors,
    match: state.top,
    items: cards.map((card) => ({
      kind: "group" as const,
      id: card.id,
      match: card.match,
      conditions: conditions(card),
    })),
  };
}

function labelOf(field: AudienceField): string {
  return field.label ?? fieldName(field.key);
}

function choicesFor(field: AudienceField | undefined, lookups: Lookups): Choice[] | undefined {
  if (field?.type === "set") {
    return lookups.tags;
  }
  if (field?.type === "saved_audience") {
    return lookups.audiences;
  }

  return field?.options?.map((o): Choice => [o, o.charAt(0).toUpperCase() + o.slice(1)]);
}

function MatchPick({
  label,
  value,
  onChange,
  disabled = false,
}: {
  label: string;
  value: Match;
  onChange: (m: Match) => void;
  disabled?: boolean;
}) {
  return (
    <SegmentedControl
      aria-label={label}
      isDisabled={disabled}
      selectedKeys={new Set([value])}
      onSelectionChange={(keys) => {
        const next = [...keys][0];
        if (next) {
          onChange(next as Match);
        }
      }}
    >
      {matchChoices.map((m) => (
        <SegmentedControlItem key={m.id} id={m.id}>
          {m.label}
        </SegmentedControlItem>
      ))}
    </SegmentedControl>
  );
}

function FieldPick({
  fields,
  value,
  onChange,
}: {
  fields: AudienceField[];
  value: string | undefined;
  onChange: (field: AudienceField) => void;
}) {
  return (
    <Select
      aria-label="Field"
      className="w-full"
      popoverClassName="w-72"
      placeholder="Choose a field"
      selectedKey={value ?? null}
      onSelectionChange={(key) => {
        const picked = fields.find((f) => f.key === key);
        if (picked) {
          onChange(picked);
        }
      }}
    >
      {fieldGroups.map((group) => {
        const items = fields
          .filter((f) => fieldGroup(f.key) === group)
          .sort((a, b) => labelOf(a).localeCompare(labelOf(b)));

        return items.length === 0 ? null : (
          <SelectSection key={group} title={group}>
            {items.map((f) => (
              <SelectItem key={f.key} id={f.key} textValue={labelOf(f)}>
                <span className="min-w-0 flex-1 truncate">{labelOf(f)}</span>
                {f.custom && typeWords[f.type] ? (
                  <span className="shrink-0 text-caption-1-regular text-text-tertiary">{typeWords[f.type]}</span>
                ) : null}
              </SelectItem>
            ))}
          </SelectSection>
        );
      })}
    </Select>
  );
}

function OpPick({
  field,
  value,
  onChange,
}: {
  field: AudienceField | undefined;
  value: string | undefined;
  onChange: (op: string) => void;
}) {
  return (
    <Select
      aria-label="Comparison"
      className="w-full"
      placeholder="Comparison"
      isDisabled={!field}
      selectedKey={value ?? null}
      onSelectionChange={(key) => key !== null && onChange(String(key))}
    >
      {(field?.ops ?? []).map((op) => (
        <SelectItem key={op} id={op}>
          {opNames[op] ?? op}
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * stored stands in for a field a saved rule names that the workspace no
 * longer lists, such as an attribute nobody holds yet, so the row still
 * says what it checks.
 */
function stored(c: Condition): AudienceField {
  return {
    key: c.field,
    type: "auto",
    where: "server",
    trust: "verified",
    ops: [c.op],
    custom: true,
    label: fieldName(c.field),
  };
}

/*
 * ConditionRow is one condition in its fixed columns, with the event
 * window under it when the field counts events.
 */
function ConditionRow({
  row,
  join,
  fields,
  lookups,
  onChange,
  onRemove,
}: {
  row: Row;
  join: string;
  fields: AudienceField[];
  lookups: Lookups;
  onChange: (condition: Condition) => void;
  onRemove: () => void;
}) {
  const c = row.condition;
  const known = c ? fields.find((f) => f.key === c.field) : undefined;
  const field = c && !known ? stored(c) : known;
  const listed = field && !known ? [...fields, field] : fields;
  const needsValue = c !== null && shapeOf(c.op, field?.type) !== "none";

  return (
    <li className="grid grid-cols-[2.75rem_minmax(0,1.15fr)_minmax(0,0.95fr)_minmax(0,1fr)_2rem] items-start gap-2">
      <span className="flex h-10 items-center text-body-2-medium text-text-tertiary">{join}</span>
      <FieldPick fields={listed} value={c?.field} onChange={(f) => onChange(withField(f))} />
      <OpPick field={field} value={c?.op} onChange={(op) => c && onChange(withOp(c, op, field?.type))} />
      <div className="flex min-w-0 flex-col gap-2">
        {c && needsValue ? (
          <ValueInput
            field={field}
            condition={c}
            choices={choicesFor(field, lookups)}
            onChange={(value) => onChange({ ...c, value })}
          />
        ) : (
          <Input aria-label="Value" isDisabled placeholder={c ? "No value needed" : "Value"} className="w-full" />
        )}
      </div>
      <span className="flex h-10 items-center">
        <IconButton icon={RiCloseLine} size="small" aria-label="Remove condition" onClick={onRemove} />
      </span>
      {c && field?.window ? (
        <div className="col-span-3 col-start-2">
          <WindowInput days={c.within_days} onChange={(days) => onChange(withWindow(c, days))} />
        </div>
      ) : null}
    </li>
  );
}

/*
 * GroupCard is one group: how its conditions join, the conditions, and a
 * way to add one.
 */
function GroupCard({
  card,
  index,
  only,
  fields,
  lookups,
  onChange,
  onRemove,
}: {
  card: Card;
  index: number;
  only: boolean;
  fields: AudienceField[];
  lookups: Lookups;
  onChange: (card: Card) => void;
  onRemove: () => void;
}) {
  const setRow = (id: string, condition: Condition) =>
    onChange({ ...card, rows: card.rows.map((r) => (r.id === id ? { ...r, condition } : r)) });

  return (
    <section
      aria-label={`Group ${index + 1}`}
      className="flex flex-col gap-3 rounded-2xl border border-border-button-default bg-background-secondary-default p-4"
    >
      <header className="flex flex-wrap items-center gap-3">
        <span className="text-body-medium text-text-primary">Group {index + 1}</span>
        <span className="text-body-2-regular text-text-secondary">matches</span>
        <MatchPick
          label={`How group ${index + 1} matches`}
          value={card.match}
          onChange={(match) => onChange({ ...card, match })}
        />
        <span className="text-body-2-regular text-text-secondary">of these</span>
        <span className="ms-auto">
          <IconButton
            icon={RiDeleteBinLine}
            size="small"
            aria-label={`Remove group ${index + 1}`}
            disabled={only}
            onClick={onRemove}
          />
        </span>
      </header>
      <ul className="flex flex-col gap-2">
        {card.rows.map((row, i) => (
          <ConditionRow
            key={row.id}
            row={row}
            join={i === 0 ? (card.match === "none" ? "Not" : "Where") : joins[card.match]}
            fields={fields}
            lookups={lookups}
            onChange={(condition) => setRow(row.id, condition)}
            onRemove={() => onChange({ ...card, rows: card.rows.filter((r) => r.id !== row.id) })}
          />
        ))}
      </ul>
      <div className="ps-14">
        <LinkButton
          variant="secondary"
          leadingIcon={RiAddLine}
          onClick={() => onChange({ ...card, rows: [...card.rows, { id: newId(), condition: null }] })}
        >
          Add condition
        </LinkButton>
      </div>
    </section>
  );
}

/*
 * RuleGroups edits a rule's form in place.
 */
export function RuleGroups({
  form,
  fields,
  lookups,
  onChange,
}: {
  form: Form;
  fields: AudienceField[];
  lookups: Lookups;
  onChange: (form: Form) => void;
}) {
  const [state, setState] = useState(() => fromForm(form));
  const update = (next: State) => {
    setState(next);
    onChange(toForm(next));
  };
  const setCard = (card: Card) => update({ ...state, cards: state.cards.map((c) => (c.id === card.id ? card : c)) });

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3">
        <div className="flex items-center gap-3">
          <span className="w-12 shrink-0 text-body-medium text-text-secondary">Who</span>
          <Select
            aria-label="Who"
            className="min-w-0 flex-1"
            selectedKey={state.visitors}
            onSelectionChange={(key) => key !== null && update({ ...state, visitors: key as Form["visitors"] })}
          >
            {visitorChoices.map(([id, name]) => (
              <SelectItem key={id} id={id}>
                {name}
              </SelectItem>
            ))}
          </Select>
        </div>
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <span className="text-body-medium text-text-secondary">People match</span>
          <MatchPick
            label="How groups match"
            value={state.top}
            disabled={state.cards.length < 2}
            onChange={(top) => update({ ...state, top })}
          />
          <span className="text-body-medium text-text-secondary">of the groups</span>
        </div>
      </div>
      <div className="flex flex-col">
        {state.cards.map((card, i) => (
          <div key={card.id} className="flex flex-col">
            {i > 0 ? (
              <span className="flex h-8 items-center justify-center">
                <span className="rounded-full bg-background-tertiary-default px-2.5 py-0.5 text-caption-1-medium text-text-secondary">
                  {state.top === "all" ? "and" : "or"}
                </span>
              </span>
            ) : null}
            <GroupCard
              card={card}
              index={i}
              only={state.cards.length === 1}
              fields={fields}
              lookups={lookups}
              onChange={setCard}
              onRemove={() => update({ ...state, cards: state.cards.filter((c) => c.id !== card.id) })}
            />
          </div>
        ))}
      </div>
      <div>
        <Button
          variant="secondary"
          leadingIcon={RiAddLine}
          onClick={() => update({ ...state, cards: [...state.cards, emptyCard()] })}
        >
          Add group
        </Button>
      </div>
    </div>
  );
}
