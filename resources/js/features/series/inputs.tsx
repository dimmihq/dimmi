/*
 * The small inputs the series inspector is built from, on BoardUI's
 * form components: a choice between named values, a number, a line of
 * text, a switch, and a button that opens the rule builder and says
 * how many conditions the rule holds. Each reports every change, so the
 * builder saves as the teammate types.
 */
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Select, SelectItem } from "@/components/base/select/select";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { RulesModal } from "@/features/broadcasts/rules-modal";
import type { AudienceRules } from "@/lib/api/audiences";

export function Pick<T extends string>({
  label,
  value,
  options,
  onChange,
  disabled = false,
}: {
  label: string;
  value: T | undefined;
  options: [T, string][];
  onChange: (value: T) => void;
  disabled?: boolean;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      isDisabled={disabled}
      selectedKey={value ?? null}
      onSelectionChange={(key) => (key === null ? undefined : onChange(String(key) as T))}
      popoverClassName="w-60"
    >
      {options.map(([id, name]) => (
        <SelectItem key={id} id={id} textValue={name}>
          {name}
        </SelectItem>
      ))}
    </Select>
  );
}

export function Text({
  label,
  value,
  placeholder,
  onChange,
  width = "w-56",
  disabled = false,
}: {
  label: string;
  value: string | undefined;
  placeholder?: string;
  onChange: (value: string) => void;
  width?: string;
  disabled?: boolean;
}) {
  return (
    <Input
      aria-label={label}
      size="small"
      value={value ?? ""}
      placeholder={placeholder}
      onChange={onChange}
      isDisabled={disabled}
      className={width}
    />
  );
}

export function Amount({
  label,
  value,
  onChange,
  disabled = false,
}: {
  label: string;
  value: number | undefined;
  onChange: (value: number) => void;
  disabled?: boolean;
}) {
  return (
    <Input
      aria-label={label}
      size="small"
      type="number"
      value={String(value ?? 0)}
      onChange={(text) => onChange(Math.trunc(globalThis.Number(text) || 0))}
      isDisabled={disabled}
      className="w-20"
    />
  );
}

export function Body({
  label,
  value,
  onChange,
  rows = 6,
  disabled = false,
}: {
  label: string;
  value: string | undefined;
  onChange: (value: string) => void;
  rows?: number;
  disabled?: boolean;
}) {
  return (
    <Textarea
      aria-label={label}
      value={value ?? ""}
      onChange={onChange}
      rows={rows}
      autoResize
      isDisabled={disabled}
      className="w-full"
    />
  );
}

export function Flag({
  label,
  on,
  onChange,
  disabled = false,
}: {
  label: string;
  on: boolean | undefined;
  onChange: (on: boolean) => void;
  disabled?: boolean;
}) {
  return <Switch aria-label={label} size="sm" isSelected={on ?? false} onChange={onChange} isDisabled={disabled} />;
}

/*
 * countRules counts the conditions a rule holds, for its button.
 */
function countRules(rules: AudienceRules | undefined): number {
  const walk = (node: unknown): number => {
    if (!node || typeof node !== "object") {
      return 0;
    }
    const group = node as { all?: unknown[]; any?: unknown[]; not?: unknown; field?: string };
    if (group.field) {
      return 1;
    }

    return [...(group.all ?? []), ...(group.any ?? [])].reduce<number>((n, child) => n + walk(child), walk(group.not));
  };

  return walk(rules?.rules);
}

export function Rules({
  workspace,
  title,
  rules,
  onChange,
  disabled = false,
}: {
  workspace: string;
  title: string;
  rules: AudienceRules | undefined;
  onChange: (rules: AudienceRules | undefined) => void;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const n = countRules(rules);

  return (
    <span className="flex items-center gap-2">
      <Button variant="secondary" size="small" disabled={disabled} onClick={() => setOpen(true)}>
        {n === 0 ? "Add rules" : `${n} ${n === 1 ? "condition" : "conditions"}`}
      </Button>
      {n > 0 && !disabled ? (
        <Button variant="ghost" size="small" onClick={() => onChange(undefined)}>
          Clear
        </Button>
      ) : null}
      {open ? (
        <RulesModal
          workspace={workspace}
          title={title}
          rules={rules ?? null}
          onSave={(next) => onChange(next)}
          onClose={() => setOpen(false)}
        />
      ) : null}
    </span>
  );
}
