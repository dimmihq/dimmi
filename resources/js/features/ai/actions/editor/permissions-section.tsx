/*
 * Who must agree before an action that changes data runs, and where it
 * may run. The customer confirming first is the default; a teammate or
 * an admin can approve on top, within a time limit after which the
 * request expires and the conversation goes to the team. Beside the
 * choices, the customer's side is told as it will happen, with the card
 * they will see, so the effect of each choice is plain before
 * publishing. Lookups need none of this and say so. Actions that change
 * data run only in the messenger, where the customer can press Confirm.
 */
import {
  RiCheckboxCircleLine,
  RiErrorWarningLine,
  RiFlashlightLine,
  RiShieldCheckLine,
  RiShieldStarLine,
  RiUserLine,
} from "@remixicon/react";

import { Checkbox } from "@/components/base/checkbox/checkbox";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import { Section } from "@/features/settings/layout";

import {
  ACTION_AGENTS,
  ACTION_CHANNELS,
  type ActionAgent,
  type ActionChannel,
  type Approval,
  type Confirmation,
} from "../api";
import type { Draft } from "../draft";
import { customerJourney, durationLabel } from "../format";
import { Block } from "../parts";
import type { Editor } from "./use-editor";

const timeouts = [15, 30, 60, 240, 1440];

const channelNames: Record<ActionChannel, string> = { widget: "Messenger", email: "Email", api: "API" };

const agentNames: Record<ActionAgent, { label: string; hint: string }> = {
  dimmi: { label: "dimmi", hint: "Answers customers." },
  operator: { label: "Operator", hint: "Works for your team." },
};

/*
 * CardPreview is the confirmation card as the customer will see it in
 * the messenger, drawn small and inert.
 */
function CardPreview({ title }: { title: string }) {
  return (
    <div
      aria-hidden
      className="flex max-w-72 flex-col gap-2.5 rounded-2xl bg-background-primary-default p-3 shadow-card"
    >
      <span className="text-body-2-medium text-text-primary">{title.trim() || "Do this for you"}?</span>
      <span className="text-caption-1-regular text-text-secondary">The details the agent filled in show here.</span>
      <span className="flex gap-1.5">
        <span className="rounded-lg bg-button-primary px-2.5 py-1 text-caption-1-medium text-text-white">Confirm</span>
        <span className="rounded-lg px-2.5 py-1 text-caption-1-medium text-text-secondary ring-1 ring-border-button-default">
          Cancel
        </span>
      </span>
    </div>
  );
}

function Journey({ draft }: { draft: Draft }) {
  return (
    <div className="flex flex-col gap-3 py-3 pe-3">
      <span className="text-body-medium text-text-primary">What the customer experiences</span>
      <ol className="flex flex-col gap-2.5">
        {customerJourney(draft).map((step, index) => (
          <li key={step} className="flex items-start gap-2.5 text-body-2-regular text-text-secondary">
            <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-background-tertiary-default text-caption-1-medium text-text-primary tabular-nums">
              {index + 1}
            </span>
            <span className="pt-px">{step}</span>
          </li>
        ))}
      </ol>
      {draft.side_effect === "write" && draft.confirmation === "customer" ? <CardPreview title={draft.title} /> : null}
    </div>
  );
}

function WritePermissions({ editor }: { editor: Editor }) {
  const { draft, update } = editor;
  const unguarded = draft.confirmation === "none" && draft.approval === "none";

  return (
    <>
      <Block label="Before it runs">
        <RadioGroup
          aria-label="Before it runs"
          value={draft.confirmation}
          onChange={(value) => update({ confirmation: value as Confirmation })}
          className="grid gap-2 sm:grid-cols-2"
        >
          <RadioCard
            value="customer"
            icon={RiUserLine}
            title="Customer confirms first"
            description="A card shows exactly what will happen. Recommended."
          />
          <RadioCard
            value="none"
            icon={RiFlashlightLine}
            title="Runs right away"
            description="For small changes that are easy to undo."
          />
        </RadioGroup>
      </Block>
      <Block label="Approval" hint="Approvals do not hold up the conversation: dimmi tells the customer it asked.">
        <RadioGroup
          aria-label="Approval"
          value={draft.approval}
          onChange={(value) => update({ approval: value as Approval })}
          className="grid gap-2 sm:grid-cols-3"
        >
          <RadioCard value="none" icon={RiCheckboxCircleLine} title="No approval" description="Runs once allowed." />
          <RadioCard
            value="teammate"
            icon={RiShieldCheckLine}
            title="A teammate approves"
            description="Anyone who can approve calls."
          />
          <RadioCard
            value="admin"
            icon={RiShieldStarLine}
            title="An admin approves"
            description="Only people who manage actions."
          />
        </RadioGroup>
        {draft.approval === "none" ? null : (
          <div className="flex flex-col gap-1.5 pt-1 sm:flex-row sm:items-center sm:justify-between">
            <span className="text-body-2-regular text-text-secondary">If nobody decides within</span>
            <Select
              aria-label="Approval time limit"
              size="sm"
              selectedKey={String(draft.approval_timeout_minutes)}
              onSelectionChange={(key) => update({ approval_timeout_minutes: Number(key) })}
              className="sm:w-40"
            >
              {timeouts.map((minutes) => (
                <SelectItem key={minutes} id={String(minutes)} textValue={durationLabel(minutes)}>
                  {durationLabel(minutes)}
                </SelectItem>
              ))}
            </Select>
          </div>
        )}
        {unguarded ? (
          <p className="flex items-start gap-2 rounded-xl bg-status-yellow-background p-3 text-body-2-regular text-status-yellow-text">
            <RiErrorWarningLine aria-hidden className="mt-0.5 size-4 shrink-0" />
            Nothing stops this action once dimmi decides to use it. Keep it to changes that are easy to undo.
          </p>
        ) : null}
      </Block>
    </>
  );
}

export function PermissionsSection({ editor }: { editor: Editor }) {
  const { draft } = editor;

  return (
    <Section
      title="Permissions"
      description={
        draft.side_effect === "read"
          ? "Lookups run right away and never change data."
          : "Who must agree before this changes anything."
      }
    >
      {draft.side_effect === "write" ? <WritePermissions editor={editor} /> : null}
      {editor.problems.permissions ? (
        <p role="alert" className="py-3 pe-3 text-body-2-regular text-text-error-primary">
          {editor.problems.permissions}
        </p>
      ) : null}
      <Journey draft={draft} />
    </Section>
  );
}

/*
 * ChannelsSection says which agents may use the action, and where dimmi
 * may use it. Operator works for the team: it looks things up at once
 * and proposes changes a teammate applies, so the customer's channel
 * does not apply to it.
 */
export function ChannelsSection({ editor }: { editor: Editor }) {
  const { draft, update, problems } = editor;
  const write = draft.side_effect === "write";
  const toggle = (channel: ActionChannel, on: boolean) =>
    update({
      channels: on
        ? ACTION_CHANNELS.filter((c) => c === channel || draft.channels.includes(c))
        : draft.channels.filter((c) => c !== channel),
    });
  const toggleAgent = (agent: ActionAgent, on: boolean) =>
    update({
      agents: on
        ? ACTION_AGENTS.filter((a) => a === agent || draft.agents.includes(a))
        : draft.agents.filter((a) => a !== agent),
    });

  return (
    <Section title="Who can use it" description="Connect an action once and give it to dimmi, Operator or both.">
      <Block
        label="Used by"
        hint={
          draft.agents.includes("operator")
            ? "Operator looks things up at once and proposes changes for a teammate to apply."
            : undefined
        }
      >
        <div className="flex flex-wrap gap-x-5 gap-y-2">
          {ACTION_AGENTS.map((agent) => (
            <Checkbox
              key={agent}
              size="sm"
              isSelected={draft.agents.includes(agent)}
              onChange={(on) => toggleAgent(agent, on)}
            >
              {agentNames[agent].label}
              <span className="text-text-tertiary"> · {agentNames[agent].hint}</span>
            </Checkbox>
          ))}
        </div>
      </Block>
      {draft.agents.includes("dimmi") ? (
        <Block
          label="dimmi uses it in"
          hint={
            write ? "Actions that change data run only in the messenger, where the customer can confirm." : undefined
          }
          problem={problems.channels}
        >
          <div className="flex flex-wrap gap-x-5 gap-y-2">
            {ACTION_CHANNELS.map((channel) => (
              <Checkbox
                key={channel}
                size="sm"
                isSelected={draft.channels.includes(channel)}
                isDisabled={write && channel !== "widget"}
                onChange={(on) => toggle(channel, on)}
              >
                {channelNames[channel]}
              </Checkbox>
            ))}
          </div>
        </Block>
      ) : null}
    </Section>
  );
}
