/*
 * The cards of an SLA policy's editor, each editing the draft the page
 * saves in one go: its name, its targets with a switch each, the clock
 * (warning, business hours, pauses), the ticket types and priorities
 * new tickets follow it for, and what a breach changes. Schedules, ticket
 * types and teams are picked from what the workspace holds.
 */
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Input } from "@/components/base/input/input";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { Row, Section } from "@/features/settings/layout";
import type { Schedule } from "@/lib/api/schedules";
import { type TicketPriority, type TicketType, ticketPriorities } from "@/lib/api/tickets";

import { priorityLabel } from "../labels";
import { DurationInput } from "./duration-input";
import { type BreachPriority, breachPriorities, type DurationUnit, type PolicyDraft, targetKeys } from "./form";

export interface DraftProps {
  draft: PolicyDraft;
  set: (change: Partial<PolicyDraft>) => void;
  locked: boolean;
}

const none = "__none";
const maxTarget = 365 * 86400;
const maxWarning = 7 * 86400;

/*
 * Block is a card row whose control needs the card's whole width, under
 * its label.
 */
function Block({ label, description, children }: { label: string; description?: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2.5 border-b border-separator-border py-2.5 pe-2.5 last:border-b-0">
      <div className="flex flex-col">
        <span className="text-body-regular text-text-primary">{label}</span>
        {description ? <span className="text-body-2-regular text-text-secondary">{description}</span> : null}
      </div>
      {children}
    </div>
  );
}

function useUnits(): Record<DurationUnit, string> {
  const { t } = useTranslation("sla");

  return { minutes: t("units.minutes"), hours: t("units.hours"), days: t("units.days") };
}

export function DetailsSection({ draft, set, locked }: DraftProps) {
  const { t } = useTranslation("sla");

  return (
    <Section title={t("details.title")}>
      <Row label={t("details.name")}>
        <Input
          aria-label={t("details.name")}
          size="small"
          value={draft.name}
          onChange={(name) => set({ name })}
          isDisabled={locked}
          maxLength={80}
          placeholder={t("details.namePlaceholder")}
          className="w-64"
        />
      </Row>
      <Block label={t("details.description")} description={t("details.descriptionHint")}>
        <Textarea
          aria-label={t("details.description")}
          rows={2}
          value={draft.description}
          onChange={(description) => set({ description })}
          isDisabled={locked}
          maxLength={500}
        />
      </Block>
    </Section>
  );
}

export function TargetsSection({ draft, set, locked }: DraftProps) {
  const { t } = useTranslation("sla");
  const units = useUnits();
  const setTarget = (key: (typeof targetKeys)[number], seconds: number | null) =>
    set({ targets: { ...draft.targets, [key]: seconds } });

  return (
    <Section title={t("targetsSection.title")} description={t("targetsSection.description")}>
      {targetKeys.map((key) => {
        const seconds = draft.targets[key];
        const name = t(`targets.${key}`);

        return (
          <Row key={key} label={name} description={t(`targetHints.${key}`)}>
            {seconds === null ? (
              <span className="text-body-2-regular text-text-tertiary">{t("targetsSection.off")}</span>
            ) : (
              <DurationInput
                label={t("targetsSection.amount", { target: name })}
                unitLabel={t("targetsSection.unit", { target: name })}
                unitNames={units}
                seconds={seconds}
                max={maxTarget}
                disabled={locked}
                onChange={(next) => setTarget(key, next)}
              />
            )}
            <Switch
              aria-label={t("targetsSection.toggle", { target: name })}
              size="sm"
              isSelected={seconds !== null}
              isDisabled={locked}
              onChange={(on) => setTarget(key, on ? 3600 : null)}
            />
          </Row>
        );
      })}
    </Section>
  );
}

export function ClockSection({ draft, set, locked, schedules }: DraftProps & { schedules: Schedule[] }) {
  const { t } = useTranslation("sla");
  const units = useUnits();
  const warns = draft.warn_before > 0;

  return (
    <Section title={t("clock.title")}>
      <Row label={t("clock.schedule")} description={t("clock.scheduleHint")}>
        <Select
          aria-label={t("clock.schedule")}
          size="sm"
          isDisabled={locked}
          selectedKey={draft.schedule_id ?? none}
          onSelectionChange={(key) => set({ schedule_id: key === none || key === null ? null : String(key) })}
          popoverClassName="w-64"
        >
          {[{ id: none, name: t("clock.allHours") }, ...schedules.map(scheduleOption(t))].map((s) => (
            <SelectItem key={s.id} id={s.id} textValue={s.name}>
              {s.name}
            </SelectItem>
          ))}
        </Select>
      </Row>
      <Row label={t("clock.warn")} description={t("clock.warnHint")}>
        {warns ? (
          <DurationInput
            label={t("clock.warnLabel")}
            unitLabel={t("targetsSection.unit", { target: t("clock.warnLabel") })}
            unitNames={units}
            seconds={draft.warn_before}
            max={maxWarning}
            disabled={locked}
            onChange={(warn_before) => set({ warn_before })}
          />
        ) : null}
        <Switch
          aria-label={t("clock.warn")}
          size="sm"
          isSelected={warns}
          isDisabled={locked}
          onChange={(on) => set({ warn_before: on ? 900 : 0 })}
        />
      </Row>
      <Row label={t("clock.pauseCustomer")} description={t("clock.pauseCustomerHint")}>
        <Switch
          aria-label={t("clock.pauseCustomer")}
          size="sm"
          isSelected={draft.pause_on_waiting_customer}
          isDisabled={locked}
          onChange={(pause_on_waiting_customer) => set({ pause_on_waiting_customer })}
        />
      </Row>
      <Row label={t("clock.pauseTeam")} description={t("clock.pauseTeamHint")}>
        <Switch
          aria-label={t("clock.pauseTeam")}
          size="sm"
          isSelected={draft.pause_on_waiting_team}
          isDisabled={locked}
          onChange={(pause_on_waiting_team) => set({ pause_on_waiting_team })}
        />
      </Row>
    </Section>
  );
}

function scheduleOption(t: (key: string, options?: Record<string, string>) => string) {
  return (s: Schedule) => ({ id: s.id, name: s.is_default ? t("clock.defaultSchedule", { name: s.name }) : s.name });
}

function toggled(list: string[], id: string, on: boolean): string[] {
  return on ? [...list.filter((x) => x !== id), id] : list.filter((x) => x !== id);
}

export function AppliesSection({ draft, set, locked, types }: DraftProps & { types: TicketType[] }) {
  const { t } = useTranslation("sla");
  const shown = types.filter((type) => !type.archived_at || draft.ticket_type_ids.includes(type.id));

  return (
    <Section title={t("applies.title")} description={t("applies.description")}>
      <Block label={t("applies.types")} description={t("applies.typesHint")}>
        {shown.length === 0 ? (
          <span className="text-body-2-regular text-text-tertiary">{t("applies.noTypes")}</span>
        ) : (
          <fieldset aria-label={t("applies.types")} className="flex flex-wrap gap-x-5 gap-y-2">
            {shown.map((type) => (
              <Checkbox
                key={type.id}
                isDisabled={locked}
                isSelected={draft.ticket_type_ids.includes(type.id)}
                onChange={(on) => set({ ticket_type_ids: toggled(draft.ticket_type_ids, type.id, on) })}
              >
                {type.name}
              </Checkbox>
            ))}
          </fieldset>
        )}
      </Block>
      <Block label={t("applies.priorities")} description={t("applies.prioritiesHint")}>
        <fieldset aria-label={t("applies.priorities")} className="flex flex-wrap gap-x-5 gap-y-2">
          {ticketPriorities.map((priority: TicketPriority) => (
            <Checkbox
              key={priority}
              isDisabled={locked}
              isSelected={draft.priorities.includes(priority)}
              onChange={(on) => set({ priorities: toggled(draft.priorities, priority, on) })}
            >
              {priorityLabel(priority)}
            </Checkbox>
          ))}
        </fieldset>
      </Block>
    </Section>
  );
}

export function BreachSection({ draft, set, locked, teams }: DraftProps & { teams: { id: string; name: string }[] }) {
  const { t } = useTranslation("sla");

  return (
    <Section title={t("breach.title")} description={t("breach.description")}>
      <Block label={t("breach.priority")} description={t("breach.priorityHint")}>
        <RadioGroup
          aria-label={t("breach.priority")}
          value={draft.breach_priority}
          isDisabled={locked}
          onChange={(next) => set({ breach_priority: next as BreachPriority })}
          className="grid grid-cols-2 gap-2 sm:grid-cols-3"
        >
          {breachPriorities.map((p) => (
            <RadioCard key={p} value={p} title={p === "none" ? t("breach.none") : priorityLabel(p)} />
          ))}
        </RadioGroup>
      </Block>
      <Row label={t("breach.team")} description={t("breach.teamHint")}>
        <Select
          aria-label={t("breach.team")}
          size="sm"
          isDisabled={locked}
          selectedKey={draft.breach_team_id ?? none}
          onSelectionChange={(key) => set({ breach_team_id: key === none || key === null ? null : String(key) })}
          popoverClassName="w-56"
        >
          {[{ id: none, name: t("breach.noTeam") }, ...teams].map((team) => (
            <SelectItem key={team.id} id={team.id} textValue={team.name}>
              {team.name}
            </SelectItem>
          ))}
        </Select>
      </Row>
    </Section>
  );
}
