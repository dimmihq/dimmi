/*
 * Pieces the actions pages share: the mark of a connection by its kind
 * (a REST API or an MCP server), the chips that say what an action does
 * to data and whether it is live, and rows of name and value pairs for
 * headers, query parameters, variables and response fields, each row
 * removable and a button to add the next.
 */
import { RiAddLine, RiCloseLine } from "@remixicon/react";
import type { ReactNode } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { KindMark } from "@/features/automation/parts";

import type { AiAction, ConnectionKind, SideEffect } from "./api";
import { kindInfo } from "./connection-values";
import { type Pair, rowKey } from "./draft";
import { actionLook, sideEffectLabels } from "./format";

/*
 * Segments picks one of a few views or modes with BoardUI's segmented
 * control.
 */
export function Segments<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: { id: T; label: string }[];
  onChange: (value: T) => void;
}) {
  return (
    <SegmentedControl
      aria-label={label}
      selectedKeys={[value]}
      onSelectionChange={(keys) => {
        const [key] = [...keys];
        if (key !== undefined) {
          onChange(key as T);
        }
      }}
      className="self-start"
    >
      {options.map((option) => (
        <SegmentedControlItem key={option.id} id={option.id}>
          {option.label}
        </SegmentedControlItem>
      ))}
    </SegmentedControl>
  );
}

/*
 * ConnectionMark is a connection's tinted mark: a globe for a REST API,
 * a plug for an MCP server.
 */
export function ConnectionMark({ kind, size = "md" }: { kind: ConnectionKind; size?: "sm" | "md" }) {
  const info = kindInfo[kind];

  return <KindMark icon={info.icon} tint={info.tint} size={size} />;
}

export function SideEffectChip({ side }: { side: SideEffect }) {
  return (
    <Chip variant="caption" color={side === "write" ? "orange" : "neutral"}>
      {sideEffectLabels[side]}
    </Chip>
  );
}

export function ActionStateChip({
  action,
}: {
  action: Pick<AiAction, "status" | "has_unpublished_changes" | "published_version">;
}) {
  const look = actionLook(action);

  return (
    <Chip variant="caption" color={look.tone}>
      {look.label}
    </Chip>
  );
}

/*
 * PairRows edits a list of name and value pairs. The value column is the
 * wider one, as it often holds a template.
 */
export function PairRows({
  label,
  rows,
  onChange,
  namePlaceholder = "Name",
  valuePlaceholder = "Value",
  addLabel = "Add",
  disabled = false,
}: {
  label: string;
  rows: Pair[];
  onChange: (rows: Pair[]) => void;
  namePlaceholder?: string;
  valuePlaceholder?: string;
  addLabel?: string;
  disabled?: boolean;
}) {
  const set = (key: string, change: Partial<Pair>) =>
    onChange(rows.map((row) => (row.key === key ? { ...row, ...change } : row)));

  return (
    <div className="flex flex-col gap-2">
      {rows.map((row, index) => (
        <div key={row.key} className="flex items-start gap-2">
          <Input
            aria-label={`${label} ${index + 1} name`}
            size="small"
            value={row.name}
            placeholder={namePlaceholder}
            onChange={(name) => set(row.key, { name })}
            isDisabled={disabled}
            inputDir="ltr"
            className="w-2/5 min-w-0"
          />
          <Input
            aria-label={`${label} ${index + 1} value`}
            size="small"
            value={row.value}
            placeholder={valuePlaceholder}
            onChange={(value) => set(row.key, { value })}
            isDisabled={disabled}
            inputDir="ltr"
            className="min-w-0 flex-1"
          />
          <Button
            variant="ghost"
            size="small"
            leadingIcon={RiCloseLine}
            aria-label={`Remove ${label.toLowerCase()} ${index + 1}`}
            onClick={() => onChange(rows.filter((other) => other.key !== row.key))}
            disabled={disabled}
          />
        </div>
      ))}
      {disabled ? null : (
        <Button
          variant="secondary"
          size="xs"
          leadingIcon={RiAddLine}
          onClick={() => onChange([...rows, { key: rowKey(), name: "", value: "" }])}
          className="self-start"
        >
          {addLabel}
        </Button>
      )}
    </div>
  );
}

/*
 * Block is one labelled area inside an editor card, with a line on what
 * it is for and, below, a problem to fix.
 */
export function Block({
  label,
  hint,
  problem,
  children,
}: {
  label: string;
  hint?: ReactNode;
  problem?: string;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-2 border-b border-separator-border py-3 pe-3 last:border-b-0">
      <div className="flex flex-col gap-0.5">
        <span className="text-body-medium text-text-primary">{label}</span>
        {hint ? <span className="text-body-2-regular text-text-secondary">{hint}</span> : null}
      </div>
      {children}
      {problem ? (
        <p role="alert" className="text-body-2-regular text-text-error-primary">
          {problem}
        </p>
      ) : null}
    </div>
  );
}
