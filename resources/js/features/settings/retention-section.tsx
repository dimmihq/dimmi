/*
 * How long the workspace keeps conversations and the files around them.
 * Closed conversations are kept for ever until a period is set; turning
 * one on first says how many conversations would go to the trash, since
 * that cannot be undone once the trash empties. A shorter period never
 * takes effect the day it is saved.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { previewRetention, type Retention, updateWorkspace, workspaceQuery } from "@/lib/api/workspace";

import { Section } from "./layout";
import { failureText } from "./teams";
import { useAccount } from "./use-account";

type Field = keyof Retention;

const fields: { key: Field; label: string; hint: string }[] = [
  {
    key: "closed_conversation_days",
    label: "Closed conversations",
    hint: "Days after closing, with nothing new, before they move to the trash. Leave empty to keep them.",
  },
  { key: "trash_days", label: "Trash", hint: "Days before the trash is emptied, 7 to 365." },
  { key: "spam_days", label: "Spam", hint: "Days before spam is deleted, 7 to 365." },
  { key: "raw_email_days", label: "Original emails", hint: "Days the raw email is kept after reading, 0 to 365." },
  { key: "export_days", label: "Data exports", hint: "Days an export can be downloaded, 1 to 30." },
];

function toText(value: number | null): string {
  return value === null ? "" : String(value);
}

function RetentionForm({ current, canManage }: { current: Retention; canManage: boolean }) {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const [values, setValues] = useState<Record<Field, string>>(() => ({
    closed_conversation_days: toText(current.closed_conversation_days),
    spam_days: toText(current.spam_days),
    trash_days: toText(current.trash_days),
    raw_email_days: toText(current.raw_email_days),
    export_days: toText(current.export_days),
  }));
  const [confirming, setConfirming] = useState<number | null>(null);
  const change = (): Partial<Retention> => ({
    closed_conversation_days: values.closed_conversation_days === "" ? null : Number(values.closed_conversation_days),
    spam_days: Number(values.spam_days),
    trash_days: Number(values.trash_days),
    raw_email_days: Number(values.raw_email_days),
    export_days: Number(values.export_days),
  });
  const save = useMutation({
    mutationFn: () => updateWorkspace(workspace, { retention: change() }),
    onSuccess: (saved) => client.setQueryData(workspaceQuery(workspace).queryKey, saved),
  });
  const submit = async () => {
    const next = change().closed_conversation_days;
    if (next && current.closed_conversation_days === null) {
      const preview = await previewRetention(workspace, next);
      setConfirming(preview.count);

      return;
    }
    save.mutate();
  };

  return (
    <div className="flex flex-col gap-3 py-3 pe-3">
      {fields.map((field) => (
        <Input
          key={field.key}
          label={field.label}
          hint={field.hint}
          inputMode="numeric"
          size="small"
          className="max-w-72"
          isDisabled={!canManage}
          value={values[field.key]}
          onChange={(value) => setValues((before) => ({ ...before, [field.key]: value }))}
        />
      ))}
      {save.error ? <p className="text-body-2-regular text-text-error-primary">{failureText(save.error)}</p> : null}
      {canManage ? (
        <Button size="small" variant="secondary" className="self-start" loading={save.isPending} onClick={submit}>
          Save retention
        </Button>
      ) : null}
      <AlertDialog
        isOpen={confirming !== null}
        onOpenChange={(open) => {
          if (!open) {
            setConfirming(null);
          }
        }}
        title="Start removing old conversations?"
        description={`${confirming ?? 0} closed conversations are past this period and will move to the trash tomorrow. Once the trash empties they are gone for good.`}
        confirmLabel="Save retention"
        onConfirm={() => save.mutateAsync()}
      />
    </div>
  );
}

export function RetentionSection({ canManage }: { canManage: boolean }) {
  const workspace = useAccount().workspace.id;
  const { data } = useQuery(workspaceQuery(workspace));
  const current = data?.retention;
  if (!current) {
    return <Section title="Retention" loading rows={3} />;
  }

  return (
    <Section title="Retention" description="How long conversations and the files around them are kept.">
      <RetentionForm key={JSON.stringify(current)} current={current} canManage={canManage} />
    </Section>
  );
}
