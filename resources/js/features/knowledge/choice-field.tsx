/*
 * A setting picked from what the connected service holds, such as a
 * repository or one of its branches, never typed in free. Its values
 * are read with the settings it depends on and read again when one of
 * them changes; until they are all filled it waits, disabled, with a
 * spinner while the service answers. Opening it shows every choice,
 * and typing narrows the list, since an account can reach hundreds of
 * repositories. The value the service marks, such as the default
 * branch, is picked at once.
 */
import { type RemixiconComponentType, RiGitBranchLine, RiGitRepositoryLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";

import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { SpinnerGlyph } from "@/components/base/spinner/spinner";
import { type SchemaProperty, type SettingOption, settingOptions } from "@/lib/api/knowledge-sources";

import type { FormValues } from "./model";

const unreadable = "These could not be read. Try again in a moment.";

/*
 * icons mark what a setting picks, such as a repository or a branch.
 */
const icons: Record<string, RemixiconComponentType> = {
  repository: RiGitRepositoryLine,
  branch: RiGitBranchLine,
};

function Reading({ what }: { what: string }) {
  return (
    <span className="flex items-center gap-1.5">
      <SpinnerGlyph size="sm" />
      Reading {what.toLowerCase()} choices…
    </span>
  );
}

function OptionLine({ option }: { option: SettingOption }) {
  return (
    <span className="flex w-full min-w-0 items-center justify-between gap-3">
      <span className="truncate">{option.label}</span>
      {option.detail ? <span className="shrink-0 text-body-2-regular text-text-tertiary">{option.detail}</span> : null}
    </span>
  );
}

export function ChoiceField({
  workspace,
  type,
  name,
  prop,
  values,
  error,
  onChange,
}: {
  workspace: string;
  type: string;
  name: string;
  prop: SchemaProperty;
  values: FormValues;
  error?: string;
  onChange: (value: string) => void;
}) {
  const depends = prop["x-depends"] ?? [];
  const ready = depends.every((d) => typeof values[d] === "string" && values[d] !== "");
  const settings = Object.fromEntries(depends.map((d) => [d, values[d]]));
  const options = useQuery({
    queryKey: ["workspace", workspace, "knowledge", "options", type, name, settings],
    queryFn: () => settingOptions(workspace, { type, settings, field: name }),
    enabled: ready,
  });
  const value = typeof values[name] === "string" ? String(values[name]) : "";
  const preselect = options.data?.find((o) => o.default)?.value ?? null;

  useEffect(() => {
    if (value === "" && preselect) {
      onChange(preselect);
    }
  }, [value, preselect, onChange]);

  const waiting = ready && options.isFetching;
  const Icon = icons[name];

  return (
    <ComboBox
      key={options.dataUpdatedAt}
      label={prop.title ?? name}
      placeholder={prop["x-placeholder"] ?? "Choose one"}
      leadingIcon={Icon}
      menuTrigger="focus"
      hint={
        waiting ? <Reading what={prop.title ?? name} /> : (error ?? (options.isError ? unreadable : prop.description))
      }
      isInvalid={error !== undefined || options.isError}
      isDisabled={!ready || options.isPending}
      defaultItems={options.data ?? []}
      selectedKey={value || null}
      onSelectionChange={(key) => onChange(key === null ? "" : String(key))}
      emptyMessage="Nothing matches"
      className="w-full"
    >
      {(o: SettingOption) => (
        <ComboBoxItem id={o.value} textValue={o.label}>
          {Icon ? <Icon aria-hidden className="size-4 shrink-0 text-foreground-icon-tertiary" /> : null}
          <OptionLine option={o} />
        </ComboBoxItem>
      )}
    </ComboBox>
  );
}
