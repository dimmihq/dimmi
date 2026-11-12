/*
 * The steps of an import after the upload. Mapping lists each column
 * with its header and a sample, and a picker of the field it fills: a
 * standard field, a custom attribute, a new attribute named from the
 * header, or nothing. Options chooses what happens to records already
 * there and shows the dry run of that choice. The run shows progress
 * while it works and what it came to, with the failed rows to download.
 */
import { RiDownloadLine, RiErrorWarningLine } from "@remixicon/react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { ProgressBar } from "@/components/base/progress/progress";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Select, SelectItem } from "@/components/base/select/select";
import type { Import, ImportMode, ImportPreview, ImportRowError } from "@/lib/api/imports";
import { formatNumber } from "@/lib/i18n/format";

import { attributeKey, type FieldChoice, newAttributeField, previewSummary, skipField } from "./import-plan";

const skipKey = "skip";

/*
 * sample returns the first values a column holds, for the mapping step.
 */
function sample(draft: Import, column: number): string {
  const values = draft.preview.map((row) => row[column] ?? "").filter(Boolean);

  return values.length > 0 ? values.slice(0, 2).join(", ") : "No values in the first rows";
}

export function MapStep({
  draft,
  choices,
  fields,
  onChange,
}: {
  draft: Import;
  choices: FieldChoice[];
  fields: string[];
  onChange: (fields: string[]) => void;
}) {
  const known = new Set(choices.map((c) => c.id));
  const set = (column: number, field: string) => onChange(fields.map((f, i) => (i === column ? field : f)));

  return (
    <ul aria-label="Columns" className="flex flex-col divide-y divide-separator-border">
      {draft.columns
        .map((header, at) => ({ header, column: at }))
        .map(({ header, column }) => {
          const field = fields[column] ?? skipField;
          const isNew = field.startsWith("attribute.") && !known.has(field);
          const taken = new Set(fields.filter((f, i) => i !== column && f !== skipField));

          return (
            <li key={`${column}-${header}`} className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate text-body-medium text-text-primary">{header || `Column ${column + 1}`}</p>
                  <p className="truncate text-body-2-regular text-text-tertiary">{sample(draft, column)}</p>
                </div>
                <Select
                  aria-label={`Field for ${header || `column ${column + 1}`}`}
                  size="sm"
                  className="w-44 shrink-0"
                  selectedKey={isNew ? newAttributeField : field || skipKey}
                  onSelectionChange={(next) => {
                    const picked = String(next);
                    if (picked === newAttributeField) {
                      set(column, `attribute.${attributeKey(header || `column ${column + 1}`)}`);
                    } else {
                      set(column, picked === skipKey ? skipField : picked);
                    }
                  }}
                >
                  <SelectItem id={skipKey} textValue="Don't import">
                    Don't import
                  </SelectItem>
                  {choices.map((c) => (
                    <SelectItem key={c.id} id={c.id} textValue={c.label} isDisabled={taken.has(c.id)}>
                      {c.label}
                    </SelectItem>
                  ))}
                  <SelectItem id={newAttributeField} textValue="New attribute">
                    New attribute…
                  </SelectItem>
                </Select>
              </div>
              {isNew ? (
                <Input
                  label="New attribute key"
                  inputDir="ltr"
                  value={field.slice("attribute.".length)}
                  onChange={(key) => set(column, `attribute.${key.trim()}`)}
                  hint="Defined as text or a number from the first value."
                />
              ) : null}
            </li>
          );
        })}
    </ul>
  );
}

function Problems({ errors }: { errors: ImportRowError[] }) {
  if (errors.length === 0) {
    return null;
  }

  return (
    <ul aria-label="Rows with problems" className="flex flex-col gap-1">
      {errors.slice(0, 5).map((e) => (
        <li key={e.row} className="flex gap-2 text-body-2-regular text-text-error-primary">
          <RiErrorWarningLine className="mt-0.5 size-4 shrink-0" aria-hidden />
          <span>
            Row {e.row}: {e.message}
          </span>
        </li>
      ))}
    </ul>
  );
}

export function OptionsStep({
  noun,
  mode,
  onMode,
  preview,
  checking,
}: {
  noun: string;
  mode: ImportMode;
  onMode: (mode: ImportMode) => void;
  preview: ImportPreview | undefined;
  checking: boolean;
}) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <Label>When someone is already here</Label>
        <RadioGroup aria-label="When a record exists" value={mode} onChange={(next) => onMode(next as ImportMode)}>
          <RadioCard
            value="update"
            title="Update them"
            description="Fill in what the file has. Empty cells change nothing; tags are only added."
          />
          <RadioCard value="skip" title="Leave them as they are" description="Only add the ones that are new." />
        </RadioGroup>
      </div>
      <div className="flex flex-col gap-2" aria-live="polite">
        <Label>Dry run</Label>
        {checking || preview === undefined ? (
          <ProgressBar aria-label="Checking the file" isIndeterminate size="sm" />
        ) : (
          <>
            <p role="status" className="text-body-2-regular text-text-primary">
              {previewSummary(preview, noun)}
            </p>
            <Problems errors={preview.errors} />
          </>
        )}
      </div>
    </div>
  );
}

export function RunStep({ run, noun, onDownload }: { run: Import; noun: string; onDownload: () => void }) {
  const done = run.status === "completed" || run.status === "failed";
  const counts = [
    ["New", run.created_count],
    ["Updated", run.updated_count],
    ["Skipped", run.skipped_count],
    ["Problems", run.failed_count],
  ] as const;

  return (
    <div className="flex flex-col gap-4">
      <ProgressBar
        label={done ? "Finished" : `Importing ${noun}`}
        value={run.processed_rows}
        maxValue={Math.max(run.row_count, 1)}
        valueLabel={`${formatNumber(run.processed_rows)} of ${formatNumber(run.row_count)}`}
      />
      {run.error ? <p className="text-body-2-regular text-text-error-primary">{run.error}</p> : null}
      <dl className="grid grid-cols-4 gap-3">
        {counts.map(([label, value]) => (
          <div key={label} className="flex flex-col gap-0.5">
            <dt className="text-body-2-regular text-text-tertiary">{label}</dt>
            <dd className="text-title-3-medium text-text-primary tabular-nums">{formatNumber(value)}</dd>
          </div>
        ))}
      </dl>
      <Problems errors={run.errors} />
      {done && run.failed_count > 0 ? (
        <Button
          variant="secondary"
          size="small"
          leadingIcon={RiDownloadLine}
          onClick={onDownload}
          className="self-start"
        >
          Download rows with problems
        </Button>
      ) : null}
    </div>
  );
}
