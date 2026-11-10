/*
 * The model the agent writes with: the connection it goes through and
 * the model on it, each model with its price per million tokens and how
 * much it can read at once. Switching connections keeps the model when
 * the new one offers it, and otherwise picks the model suggested for
 * answers.
 */
import { Link } from "@tanstack/react-router";

import { Select, SelectItem } from "@/components/base/select/select";
import { Row, Section } from "@/features/settings/layout";
import type { AgentView, AiModelEntry, AiProviderKey } from "@/lib/api/ai";

import { type AgentChange, modelDetail, modelsFor, usableKeys } from "../format";
import { ModelLogo, ProviderLogo } from "../logos";
import { preferred } from "./options";

function ConnectionSelect({
  keys,
  value,
  disabled,
  onPick,
}: {
  keys: AiProviderKey[];
  value: string | null;
  disabled: boolean;
  onPick: (key: AiProviderKey) => void;
}) {
  return (
    <Select
      aria-label="Connection"
      size="sm"
      isDisabled={disabled}
      selectedKey={value}
      placeholder="Choose a connection"
      onSelectionChange={(id) => {
        const key = keys.find((k) => k.id === id);
        if (key) {
          onPick(key);
        }
      }}
      className="w-full sm:w-64"
      popoverClassName="w-72"
    >
      {keys.map((key) => (
        <SelectItem key={key.id} id={key.id} textValue={key.name}>
          <ProviderLogo provider={key.provider} size="xs" />
          <span className="truncate">{key.provider === "dimmi" ? "dimmi" : key.name}</span>
        </SelectItem>
      ))}
    </Select>
  );
}

/*
 * ModelSelect picks a model, each shown with its price per million
 * tokens and how much it reads at once.
 */
export function ModelSelect({
  label,
  models,
  value,
  disabled,
  placeholder,
  onPick,
}: {
  label: string;
  models: AiModelEntry[];
  value: string | null;
  disabled: boolean;
  placeholder: string;
  onPick: (model: string) => void;
}) {
  return (
    <Select
      aria-label={label}
      size="sm"
      isDisabled={disabled || models.length === 0}
      selectedKey={value}
      placeholder={placeholder}
      onSelectionChange={(id) => (id ? onPick(String(id)) : undefined)}
      className="w-full sm:w-64"
      popoverClassName="w-80"
      renderValue={({ selectedText, isPlaceholder }) =>
        isPlaceholder || !value ? (
          placeholder
        ) : (
          <>
            <ModelLogo model={value} size="xs" />
            <span className="truncate">{selectedText}</span>
          </>
        )
      }
    >
      {models.map((model) => (
        <SelectItem key={model.id} id={model.id} textValue={model.display_name}>
          <ModelLogo model={model.id} size="sm" />
          <span className="flex min-w-0 flex-col">
            <span className="truncate text-body-2-medium text-text-primary">{model.display_name}</span>
            {modelDetail(model) ? (
              <span className="truncate text-caption-1-regular text-text-tertiary">{modelDetail(model)}</span>
            ) : null}
          </span>
        </SelectItem>
      ))}
    </Select>
  );
}

export function ModelSection({
  view,
  canWrite,
  save,
}: {
  view: AgentView;
  canWrite: boolean;
  save: (change: AgentChange) => void;
}) {
  const { agent, catalog } = view;
  const keys = usableKeys(view.keys);
  const answerKey = keys.find((key) => key.id === agent.answer_key_id);
  const models = modelsFor(answerKey, catalog);
  const current = models.find((model) => model.id === agent.answer_model);

  if (keys.length === 0) {
    return (
      <Section title="Model" description="The model that writes the agent's answers.">
        <Row label="No connection yet" description="Connect a provider to choose the model the agent answers with.">
          <Link
            to="/d/settings/$section"
            params={{ section: "ai-providers" }}
            className="rounded-md text-body-2-medium text-text-primary outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            Add a provider
          </Link>
        </Row>
      </Section>
    );
  }

  const pickKey = (key: AiProviderKey) => {
    const offered = modelsFor(key, catalog);
    const keep = offered.some((model) => model.id === agent.answer_model);
    const model = keep ? undefined : preferred(offered, "answer")?.id;
    save({ answer_key_id: key.id, ...(model ? { answer_model: model } : {}) });
  };

  return (
    <Section title="Model" description="The model that writes the agent's answers.">
      <Row label="Connection" description="Which provider the agent goes through.">
        <ConnectionSelect keys={keys} value={answerKey?.id ?? null} disabled={!canWrite} onPick={pickKey} />
      </Row>
      <Row
        label="Answer model"
        description={
          current && modelDetail(current) ? modelDetail(current) : "Larger models answer better and cost more."
        }
      >
        <ModelSelect
          label="Answer model"
          models={models}
          value={current?.id ?? null}
          disabled={!canWrite || !answerKey}
          placeholder={answerKey ? "Choose a model" : "Choose a connection first"}
          onPick={(answer_model) => save({ answer_model })}
        />
      </Row>
    </Section>
  );
}
