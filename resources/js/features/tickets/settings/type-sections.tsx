/*
 * The settings cards of a ticket type's editor, each editing the draft
 * the page saves in one go: what the type is called and how it looks,
 * where its tickets land, and, for customer types, what the customer
 * sees and how the ticket closes with them. Its states have their own
 * section, in type-states.
 */
import { Link } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { Input } from "@/components/base/input/input";
import { NumberField } from "@/components/base/number-field/number-field";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { Row, Section } from "@/features/settings/layout";
import type { TicketKind } from "@/lib/api/tickets";
import { cx } from "@/utils/cx";

import { kindHint, kindLabel, typeIcons } from "../labels";
import type { TypeDraft } from "./form";

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
