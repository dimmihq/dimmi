/*
 * A chip as the step writer draws it: a quiet pill with an icon for what
 * it names. Pressing an action chip opens how strictly dimmi must call
 * it: required, which is checked before a reply goes out and retried up
 * to three times before a teammate takes over, or only when it applies;
 * which earlier actions must run first; and what to do when dimmi cannot
 * call it in a conversation.
 */
import {
  RiArticleLine,
  RiFlashlightLine,
  RiListOrdered2,
  RiPriceTag3Line,
  RiTeamLine,
  RiTicketLine,
  RiUserLine,
} from "@remixicon/react";
import { type NodeViewProps, NodeViewWrapper } from "@tiptap/react";
import { useContext } from "react";

import { Popover, PopoverHeader, PopoverTrigger } from "@/components/base/popover/popover";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import type { BindingKind } from "@/lib/api/procedures";
import { cx } from "@/utils/cx";

import { Picks } from "../pick";
import { labelOf } from "./procedure-suggestions";
import { ChipContext, type ChipOptions } from "./procedure-token";

const icons: Record<BindingKind, typeof RiFlashlightLine> = {
  action: RiFlashlightLine,
  attribute: RiUserLine,
  article: RiArticleLine,
  handoff: RiTeamLine,
  tag: RiPriceTag3Line,
  ticket_type: RiTicketLine,
  sub_procedure: RiListOrdered2,
};

const tints: Record<BindingKind, string> = {
  action: "bg-status-lime-background text-status-lime-text",
  attribute: "bg-status-blue-background text-status-blue-text",
  article: "bg-status-blue-background text-status-blue-text",
  handoff: "bg-status-orange-background text-status-orange-text",
  tag: "bg-badge-neutral-background text-text-secondary",
  ticket_type: "bg-status-purple-background text-status-purple-text",
  sub_procedure: "bg-status-purple-background text-status-purple-text",
};

function Choice({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: readonly (readonly [string, string])[];
  onChange: (value: string) => void;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-body-2-medium text-text-secondary">{label}</span>
      <SegmentedControl
        aria-label={label}
        selectedKeys={new Set([value])}
        onSelectionChange={(keys) => {
          const next = [...keys][0];
          if (next) {
            onChange(String(next));
          }
        }}
      >
        {options.map(([id, name]) => (
          <SegmentedControlItem key={id} id={id}>
            {name}
          </SegmentedControlItem>
        ))}
      </SegmentedControl>
    </div>
  );
}

export function ProcedureChip({ node, editor, extension, selected }: NodeViewProps) {
  const live = useContext(ChipContext);
  const options = extension.options as ChipOptions;
  const chip = { kind: node.attrs.kind as BindingKind, ref: String(node.attrs.ref) };
  const binding = live
    ? live.step.bindings?.find((b) => b.kind === chip.kind && b.ref === chip.ref)
    : options.binding(chip);
  const resolved = live ? labelOf(live.sources, chip) : options.label(chip);
  const label = resolved || binding?.label || String(node.attrs.label) || chip.ref;
  const Icon = icons[chip.kind] ?? RiFlashlightLine;
  const pill = cx(
    "mx-px inline-flex items-center gap-0.5 rounded-md px-1 align-baseline outline-none",
    tints[chip.kind],
    selected && "ring-2 ring-border-focus-ring",
  );
  const content = (
    <>
      <Icon aria-hidden className="size-3 shrink-0" />
      {label}
    </>
  );

  if (chip.kind !== "action" || !editor.isEditable) {
    return (
      <NodeViewWrapper as="span" data-chip={chip.kind} className="inline">
        <span className={pill}>{content}</span>
      </NodeViewWrapper>
    );
  }

  const earlier = (live ? live.earlier : options.earlier()).filter(([id]) => id !== chip.ref);
  const requires = live ? (live.step.enforcement?.requires ?? []) : options.requires();

  return (
    <NodeViewWrapper as="span" data-chip={chip.kind} className="inline">
      <PopoverTrigger>
        <button type="button" contentEditable={false} className={cx(pill, "cursor-pointer")}>
          {content}
          {binding?.execution === "conditional" ? <span className="opacity-70">· if needed</span> : null}
        </button>
        <Popover placement="bottom start" aria-label={`${label} action`} className="flex w-80 flex-col gap-4">
          <PopoverHeader title={label} description="How dimmi treats this action in the procedure." />
          <Choice
            label="Execution"
            value={binding?.execution ?? "required"}
            options={[
              ["required", "Required"],
              ["conditional", "Only when it applies"],
            ]}
            onChange={(execution) => options.onBinding(chip, { execution: execution as "required" | "conditional" })}
          />
          <p className="-mt-2 text-body-2-regular text-text-tertiary">
            {binding?.execution === "conditional"
              ? "dimmi calls it when it fits. Skipping it never holds a reply back."
              : "dimmi must call it before answering. A reply that skips it is sent back up to three times, then a teammate takes over."}
          </p>
          <div className="flex flex-col gap-1.5">
            <span className="text-body-2-medium text-text-secondary">Run only after</span>
            <Picks
              choices={earlier}
              value={requires}
              empty="No earlier actions in this procedure."
              onChange={options.onRequires}
            />
          </div>
          <Choice
            label="If unavailable"
            value={binding?.on_missing ?? "handoff"}
            options={[
              ["handoff", "Hand over to the team"],
              ["continue", "Continue without it"],
            ]}
            onChange={(onMissing) => options.onBinding(chip, { on_missing: onMissing as "handoff" | "continue" })}
          />
        </Popover>
      </PopoverTrigger>
    </NodeViewWrapper>
  );
}
