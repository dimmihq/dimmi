/*
 * What the workspace keeps out of its conversations. The built-in rules
 * mask card numbers and US social security numbers; the workspace adds
 * its own patterns, applied after them in their order. Every rule acts
 * before a message, note or subject is stored, and what it replaces is
 * gone for good. The test box shows what the rules would make of a text
 * without storing it. Below, the link checks run on what customers send.
 * Only people the server lets manage these see the controls.
 */
import { RiArrowUpLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { AlertDialog } from "@/components/base/modal/modal";
import { Switch } from "@/components/base/switch/switch";
import { Textarea } from "@/components/base/textarea/textarea";
import { ApiError } from "@/lib/api/client";
import {
  type BuiltIns,
  createRedactionRule,
  deleteRedactionRule,
  type RedactionRule,
  type RedactionTest,
  redactionKey,
  redactionQuery,
  tryRedaction,
  updateRedaction,
  updateRedactionRule,
} from "@/lib/api/redaction";
import { callerQuery } from "@/lib/api/workspace";

import { Action, Danger } from "../controls";
import { Empty, Row, Section, SettingsPage } from "../layout";
import { LinkSafetySection } from "../link-safety-section";
import { RetentionSection } from "../retention-section";
import { failureText } from "../teams";
import { useAccount } from "../use-account";

const maxRules = 10;

function useRefresh(workspace: string) {
  const client = useQueryClient();

  return () => client.invalidateQueries({ queryKey: redactionKey(workspace) });
}

function RuleDialog({ rule, onClose }: { rule: RedactionRule | null; onClose: () => void }) {
  const workspace = useAccount().workspace.id;
  const refresh = useRefresh(workspace);
  const [name, setName] = useState(rule?.name ?? "");
  const [pattern, setPattern] = useState(rule?.pattern ?? "");
  const [replacement, setReplacement] = useState(rule?.replacement ?? "[redacted]");
  const save = useMutation({
    mutationFn: () => {
      const fields = { name, pattern, replacement };

      return rule ? updateRedactionRule(workspace, rule.id, fields) : createRedactionRule(workspace, fields);
    },
    onSuccess: refresh,
  });
  const fields = save.error instanceof ApiError ? save.error.fields : {};
  const refused = save.error && Object.keys(fields).length === 0 ? failureText(save.error) : null;

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      variant="default"
      icon={false}
      title={rule ? `Edit ${rule.name}` : "New redaction rule"}
      description="A pattern in Go's regular expression syntax. Each match is replaced before the text is stored."
      confirmLabel={rule ? "Save" : "Add rule"}
      isConfirmDisabled={name.trim() === "" || pattern === ""}
      onConfirm={() => save.mutateAsync()}
    >
      <div className="flex flex-col gap-3">
        <Input label="Name" value={name} onChange={setName} isInvalid={"name" in fields} errorMessage={fields.name} />
        <Input
          label="Pattern"
          inputDir="ltr"
          placeholder="ACCT-\d+"
          value={pattern}
          onChange={setPattern}
          isInvalid={"pattern" in fields}
          errorMessage={fields.pattern}
        />
        <Input
          label="Replace with"
          value={replacement}
          onChange={setReplacement}
          isInvalid={"replacement" in fields}
          errorMessage={fields.replacement}
        />
        {refused ? <p className="text-body-2-regular text-text-error-primary">{refused}</p> : null}
      </div>
    </AlertDialog>
  );
}

function DeleteRuleDialog({ rule, onClose }: { rule: RedactionRule; onClose: () => void }) {
  const workspace = useAccount().workspace.id;
  const refresh = useRefresh(workspace);
  const remove = useMutation({ mutationFn: () => deleteRedactionRule(workspace, rule.id), onSuccess: refresh });

  return (
    <AlertDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      title={`Delete ${rule.name}?`}
      description="New messages stop being checked against it. What it already replaced stays replaced."
      confirmLabel="Delete rule"
      onConfirm={() => remove.mutateAsync()}
    />
  );
}

function BuiltInRules({ builtIns, canManage }: { builtIns: BuiltIns; canManage: boolean }) {
  const workspace = useAccount().workspace.id;
  const refresh = useRefresh(workspace);
  const change = useMutation({
    mutationFn: (next: Partial<BuiltIns>) => updateRedaction(workspace, next),
    onSuccess: refresh,
  });

  return (
    <Section
      title="Built-in rules"
      description="They apply to messages, notes and subjects as they are stored. Attachments and images are not scanned."
    >
      <Row
        label="Card numbers"
        description="Masks payment card numbers and keeps the last four digits. Order numbers stay as they are."
      >
        <Switch
          aria-label="Card numbers"
          size="sm"
          isSelected={builtIns.card_numbers}
          isDisabled={!canManage}
          onChange={(on) => change.mutate({ card_numbers: on })}
        />
      </Row>
      <Row
        label="US social security numbers"
        description="Off by default, because order and ticket numbers often share their shape."
      >
        <Switch
          aria-label="US social security numbers"
          size="sm"
          isSelected={builtIns.us_ssn}
          isDisabled={!canManage}
          onChange={(on) => change.mutate({ us_ssn: on })}
        />
      </Row>
    </Section>
  );
}

function RuleRow({
  rule,
  first,
  canManage,
  onEdit,
  onDelete,
  onMoveUp,
}: {
  rule: RedactionRule;
  first: boolean;
  canManage: boolean;
  onEdit: () => void;
  onDelete: () => void;
  onMoveUp: () => void;
}) {
  const workspace = useAccount().workspace.id;
  const refresh = useRefresh(workspace);
  const toggle = useMutation({
    mutationFn: (enabled: boolean) => updateRedactionRule(workspace, rule.id, { enabled }),
    onSuccess: refresh,
  });

  return (
    <Row
      label={rule.name}
      description={
        <span dir="ltr" className="font-mono">
          {rule.pattern} → {rule.replacement}
        </span>
      }
    >
      <Switch
        aria-label={`Use ${rule.name}`}
        size="sm"
        isSelected={rule.enabled}
        isDisabled={!canManage}
        onChange={(on) => toggle.mutate(on)}
      />
      {canManage ? (
        <>
          <IconButton
            size="small"
            icon={RiArrowUpLine}
            aria-label={`Move ${rule.name} up`}
            disabled={first}
            onClick={onMoveUp}
          />
          <Action size="xs" onPress={onEdit}>
            Edit
          </Action>
          <Danger onPress={onDelete}>Delete</Danger>
        </>
      ) : null}
    </Row>
  );
}

function OwnRules({ rules, canManage }: { rules: RedactionRule[]; canManage: boolean }) {
  const workspace = useAccount().workspace.id;
  const refresh = useRefresh(workspace);
  const [editing, setEditing] = useState<RedactionRule | "new" | null>(null);
  const [deleting, setDeleting] = useState<RedactionRule | null>(null);
  const reorder = useMutation({
    mutationFn: (order: string[]) => updateRedaction(workspace, { order }),
    onSuccess: refresh,
  });
  const moveUp = (index: number) => {
    const order = rules.map((rule) => rule.id);
    [order[index - 1], order[index]] = [order[index] ?? "", order[index - 1] ?? ""];
    reorder.mutate(order);
  };

  return (
    <Section
      title="Your rules"
      description={`Applied after the built-in rules, top to bottom. A workspace keeps up to ${maxRules}.`}
      action={canManage && rules.length < maxRules ? <Action onPress={() => setEditing("new")}>New rule</Action> : null}
    >
      {rules.length === 0 ? <Empty>No rules of your own yet.</Empty> : null}
      {rules.map((rule, index) => (
        <RuleRow
          key={rule.id}
          rule={rule}
          first={index === 0}
          canManage={canManage}
          onEdit={() => setEditing(rule)}
          onDelete={() => setDeleting(rule)}
          onMoveUp={() => moveUp(index)}
        />
      ))}
      {editing ? (
        <RuleDialog
          key={editing === "new" ? "new" : editing.id}
          rule={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
        />
      ) : null}
      {deleting ? <DeleteRuleDialog rule={deleting} onClose={() => setDeleting(null)} /> : null}
    </Section>
  );
}

function hitsText(result: RedactionTest): string {
  const count = result.redactions.reduce((total, hit) => total + hit.count, 0);

  return count === 1 ? "1 item redacted" : `${count} items redacted`;
}

function TryRules() {
  const workspace = useAccount().workspace.id;
  const [text, setText] = useState("");
  const attempt = useMutation({ mutationFn: () => tryRedaction(workspace, text) });

  return (
    <Section title="Try your rules" description="Nothing you type here is stored or logged.">
      <div className="flex flex-col gap-3 py-3 pe-3">
        <Textarea
          aria-label="Text to try"
          placeholder="Paste a message to see what would be stored"
          rows={3}
          autoResize
          maxRows={10}
          value={text}
          onChange={setText}
        />
        <Button
          size="small"
          variant="secondary"
          className="self-start"
          loading={attempt.isPending}
          disabled={text.trim() === ""}
          onClick={() => attempt.mutate()}
        >
          Try
        </Button>
        {attempt.data ? (
          <output aria-label="Stored as" className="flex flex-col gap-1">
            <span className="text-body-2-medium text-text-secondary">{hitsText(attempt.data)}</span>
            <span className="whitespace-pre-wrap text-body-regular text-text-primary">{attempt.data.text}</span>
          </output>
        ) : null}
        {attempt.error ? (
          <p className="text-body-2-regular text-text-error-primary">{failureText(attempt.error)}</p>
        ) : null}
      </div>
    </Section>
  );
}

export function ConversationSafetySettings() {
  const workspace = useAccount().workspace.id;
  const { data: redaction } = useQuery(redactionQuery(workspace));
  const { data: caller } = useQuery(callerQuery(workspace));
  const permissions = caller?.member?.permissions ?? [];
  const canManage = permissions.includes("redaction:manage") && permissions.includes("settings:write");

  return (
    <SettingsPage
      title="Conversation safety"
      description="Keep card numbers and other sensitive text out of what your workspace stores."
    >
      {redaction ? (
        <>
          <BuiltInRules builtIns={redaction.built_ins} canManage={canManage} />
          <OwnRules rules={redaction.rules} canManage={canManage} />
          {canManage ? <TryRules /> : null}
        </>
      ) : null}
      <LinkSafetySection canManage={permissions.includes("settings:write")} />
      <RetentionSection canManage={permissions.includes("settings:write")} />
    </SettingsPage>
  );
}
