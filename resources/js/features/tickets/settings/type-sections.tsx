/*
 * The settings cards of a ticket type's editor, each editing the draft
 * the page saves in one go: what the type is called and how it looks,
 * where its tickets land, the states it uses with its default and
 * default resolved state, and, for customer types, what the customer
 * sees and how the ticket closes with them.
 */
import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { NumberField } from "@/components/base/number-field/number-field";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { Row, Section } from "@/features/settings/layout";
import { stateGroups, type TicketKind, type TicketState } from "@/lib/api/tickets";
import { cx } from "@/utils/cx";

import { groupLabel, kindHint, kindLabel, typeIcons } from "../labels";
import { byGroup, type TypeDraft } from "./form";

export interface DraftProps {
  draft: TypeDraft;
  set: (change: Partial<TypeDraft>) => void;
  locked: boolean;
}

const none = "__none";

/*
 * Block is a card row whose control needs the card's whole width, such
 * as a message, under its label.
 */
function Block({ label, description, children }: { label: string; description?: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2 border-b border-separator-border py-2.5 pe-2.5 last:border-b-0">
      <div className="flex flex-col">
        <span className="text-body-regular text-text-primary">{label}</span>
        {description ? <span className="text-body-2-regular text-text-secondary">{description}</span> : null}
      </div>
      {children}
    </div>
  );
}

export function DetailsSection({ draft, set, locked, kind }: DraftProps & { kind: TicketKind }) {
  return (
    <Section title="Details">
      <Row label="Kind" description={kindHint(kind)}>
        <span className="text-body-2-medium text-text-secondary">{kindLabel(kind)}</span>
      </Row>
      <Row label="Name">
        <Input
          aria-label="Name"
          size="small"
          value={draft.name}
          onChange={(name) => set({ name })}
          isDisabled={locked}
          maxLength={60}
          className="w-64"
        />
      </Row>
      <Block label="Description" description="Customers read it on the form; teammates in the type picker.">
        <Textarea
          aria-label="Description"
          rows={2}
          value={draft.description}
          onChange={(description) => set({ description })}
          isDisabled={locked}
          maxLength={500}
        />
      </Block>
      <Block label="Icon">
        <fieldset aria-label="Icon" className="flex flex-wrap gap-1.5">
          {Object.entries(typeIcons).map(([name, Icon]) => (
            <button
              key={name}
              type="button"
              aria-pressed={draft.icon === name}
              aria-label={name}
              disabled={locked}
              onClick={() => set({ icon: name })}
              className={cx(
                "flex size-9 items-center justify-center rounded-lg border outline-none transition-colors duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
                draft.icon === name
                  ? "border-border-button-active bg-background-primary-active text-foreground-icon-primary"
                  : "border-transparent text-foreground-icon-secondary hover:bg-background-secondary-hover",
              )}
            >
              <Icon aria-hidden className="size-[18px]" />
            </button>
          ))}
        </fieldset>
      </Block>
      <Row label="AI fills fields" description="Suggest field values from the conversation, using each field's hint.">
        <Switch
          aria-label="AI fills fields"
          size="sm"
          isSelected={draft.ai_auto_fill}
          isDisabled={locked}
          onChange={(ai_auto_fill) => set({ ai_auto_fill })}
        />
      </Row>
    </Section>
  );
}

export function RoutingSection({
  draft,
  set,
  locked,
  inboxes,
  teams,
}: DraftProps & { inboxes: { id: string; name: string }[]; teams: { id: string; name: string }[] }) {
  return (
    <Section title="Routing" description="Where new tickets of this type land.">
      <Row label="Inbox">
        <Select
          aria-label="Inbox"
          size="sm"
          isDisabled={locked}
          selectedKey={draft.inbox_id ?? none}
          onSelectionChange={(key) => set({ inbox_id: key === none ? null : String(key) })}
          popoverClassName="w-56"
        >
          {[{ id: none, name: "The conversation's inbox" }, ...inboxes].map((i) => (
            <SelectItem key={i.id} id={i.id} textValue={i.name}>
              {i.name}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label="Team">
        <Select
          aria-label="Team"
          size="sm"
          isDisabled={locked}
          selectedKey={draft.team_id ?? none}
          onSelectionChange={(key) => set({ team_id: key === none ? null : String(key) })}
          popoverClassName="w-56"
        >
          {[{ id: none, name: "No team" }, ...teams].map((t) => (
            <SelectItem key={t.id} id={t.id} textValue={t.name}>
              {t.name}
            </SelectItem>
          ))}
        </Select>
      </Row>
    </Section>
  );
}

function StateSelect({
  label,
  value,
  options,
  locked,
  onChange,
}: {
  label: string;
  value: string | null;
  options: TicketState[];
  locked: boolean;
  onChange: (id: string) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      isDisabled={locked || options.length === 0}
      selectedKey={value && options.some((o) => o.id === value) ? value : null}
      placeholder="Pick a state"
      onSelectionChange={(key) => key !== null && onChange(String(key))}
      popoverClassName="w-56"
    >
      {options.map((o) => (
        <SelectItem key={o.id} id={o.id} textValue={o.name}>
          {o.name}
        </SelectItem>
      ))}
    </Select>
  );
}

export function StatesSection({ draft, set, locked, states }: DraftProps & { states: TicketState[] }) {
  const groups = byGroup(states);
  const chosen = states.filter((s) => draft.state_ids.includes(s.id));
  const toggle = (id: string, on: boolean) =>
    set({ state_ids: on ? [...draft.state_ids, id] : draft.state_ids.filter((s) => s !== id) });

  return (
    <Section title="States" description="The states tickets of this type can move through.">
      {stateGroups.map((group) => (
        <Row key={group} label={groupLabel(group)}>
          {groups[group].map((state) => (
            <Checkbox
              key={state.id}
              isDisabled={locked}
              isSelected={draft.state_ids.includes(state.id)}
              onChange={(on) => toggle(state.id, on)}
            >
              {state.name}
            </Checkbox>
          ))}
        </Row>
      ))}
      <Row label="New tickets start in">
        <StateSelect
          label="Default state"
          value={draft.default_state_id}
          options={chosen.filter((s) => s.group !== "resolved")}
          locked={locked}
          onChange={(default_state_id) => set({ default_state_id })}
        />
      </Row>
      <Row label="Closing the conversation moves it to">
        <StateSelect
          label="Default resolved state"
          value={draft.default_resolved_state_id}
          options={chosen.filter((s) => s.group === "resolved")}
          locked={locked}
          onChange={(default_resolved_state_id) => set({ default_resolved_state_id })}
        />
      </Row>
    </Section>
  );
}

export function CustomerSection({ draft, set, locked }: DraftProps) {
  const autoResolve = draft.auto_resolve_days !== null;

  return (
    <Section title="Customer" description="What customers see of this type and how it ends with them.">
      <Row label="Shown in the messenger and portal" description="Customers can open it from Home and the help center.">
        <Switch
          aria-label="Shown in the messenger"
          size="sm"
          isSelected={draft.customer_visible}
          isDisabled={locked}
          onChange={(customer_visible) => set({ customer_visible })}
        />
      </Row>
      <Row label="Shared with the company" description="Everyone at the customer's company sees these requests.">
        <Switch
          aria-label="Shared with the company"
          size="sm"
          isSelected={draft.company_sharing}
          isDisabled={locked}
          onChange={(company_sharing) => set({ company_sharing })}
        />
      </Row>
      <Row
        label="Reopen window"
        description="A reply this many days after resolving reopens it; later, a follow-up opens."
      >
        <NumberField
          aria-label="Reopen window in days"
          size="small"
          minValue={0}
          maxValue={90}
          value={draft.reopen_days}
          isDisabled={locked}
          onChange={(days) => set({ reopen_days: Number.isNaN(days) ? 0 : days })}
          className="w-32"
        />
      </Row>
      <Row label="Auto-resolve" description="Resolve tickets left waiting on the customer, after telling them.">
        <Switch
          aria-label="Auto-resolve"
          size="sm"
          isSelected={autoResolve}
          isDisabled={locked}
          onChange={(on) => set({ auto_resolve_days: on ? 7 : null })}
        />
      </Row>
      {autoResolve ? (
        <Block label="After waiting this many days, with this message">
          <NumberField
            aria-label="Auto-resolve after days"
            size="small"
            minValue={1}
            maxValue={90}
            value={draft.auto_resolve_days ?? 7}
            isDisabled={locked}
            onChange={(days) => set({ auto_resolve_days: Number.isNaN(days) ? 1 : days })}
            className="w-32"
          />
          <Textarea
            aria-label="Auto-resolve message"
            rows={2}
            maxLength={2000}
            isDisabled={locked}
            value={draft.auto_resolve_message ?? ""}
            placeholder="We haven't heard back, so we've closed this. Reply any time to reopen it."
            onChange={(message) => set({ auto_resolve_message: message || null })}
          />
        </Block>
      ) : null}
      <Block label="Close the loop">
        <p className="text-body-2-regular text-text-secondary">
          When back-office work or a linked issue resolves, the ticket moves to Close the loop. Use a workflow on
          “Ticket enters close the loop” to message the customer.{" "}
          <Link to="/d/workflows" className="text-body-2-medium text-text-primary underline">
            Open workflows
          </Link>
        </p>
      </Block>
    </Section>
  );
}
