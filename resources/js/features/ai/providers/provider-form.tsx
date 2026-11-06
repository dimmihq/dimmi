/*
 * The second step of adding a provider: a name, the key and, for a
 * server that speaks OpenAI, its address. Test & save stores the key and
 * checks it with the provider in one go. When the provider refuses it,
 * the reason shows under the form and the saved draft is fixed in place
 * on the next try, so a typo never leaves a second connection behind;
 * closing the modal throws the failed draft away.
 */
import { RiErrorWarningLine, RiExternalLinkLine } from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Input } from "@/components/base/input/input";
import { ModalBody, ModalFooter } from "@/components/base/modal/modal";
import { toast } from "@/components/base/toast/toast";
import {
  type AiProviderDescriptor,
  type AiProviderKey,
  aiKey,
  createAiKey,
  testAiKey,
  updateAiKey,
} from "@/lib/api/ai";

import { keyStatus, modelCount } from "../format";
import { providerInfo, providerName } from "../provider-info";
import { ProviderMark } from "../provider-mark";
import { checkFields, type Problems, problemsOf } from "./problems";

/*
 * ProviderForm asks for one provider's details and saves them.
 */
export function ProviderForm({
  workspace,
  descriptor,
  draft,
  onDraft,
  onCancel,
  onDone,
}: {
  workspace: string;
  descriptor: AiProviderDescriptor;
  draft: string | null;
  onDraft: (id: string | null) => void;
  onCancel: () => void;
  onDone: () => void;
}) {
  const client = useQueryClient();
  const provider = descriptor.id;
  const info = providerInfo[provider];
  const compatible = provider === "openai_compatible";
  const [name, setName] = useState(compatible ? "My server" : providerName(provider));
  const [apiKey, setApiKey] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [problems, setProblems] = useState<Problems>({});
  const [busy, setBusy] = useState(false);

  const store = async (): Promise<AiProviderKey> => {
    const body = {
      name: name.trim(),
      ...(apiKey.trim() ? { api_key: apiKey.trim() } : {}),
      ...(compatible ? { base_url: baseUrl.trim() } : {}),
    };
    if (draft) {
      await updateAiKey(workspace, draft, body);
      return testAiKey(workspace, draft);
    }
    const created = await createAiKey(workspace, { provider, ...body });
    onDraft(created.id);

    return created;
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const missing = checkFields(provider, { name, apiKey, baseUrl });
    setProblems(missing);
    if (Object.keys(missing).length > 0) {
      return;
    }
    setBusy(true);
    try {
      const saved = await store();
      await client.invalidateQueries({ queryKey: aiKey(workspace) });
      if (keyStatus(saved) !== "ok") {
        setProblems({
          form: `${providerName(provider)} refused the key${saved.status_reason ? `: ${saved.status_reason}` : "."}`,
        });
        return;
      }
      onDraft(null);
      toast.success(`${saved.name} connected`, { description: modelCount(saved) });
      onDone();
    } catch (error) {
      setProblems(problemsOf(error));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="contents" onSubmit={submit} noValidate>
      <ModalBody className="flex flex-col gap-5 pb-1">
        <div className="flex items-center gap-3 rounded-2xl bg-background-secondary-default p-3">
          <ProviderMark provider={provider} size="lg" />
          <div className="flex min-w-0 flex-col">
            <span className="text-body-medium text-text-primary">{providerName(provider)}</span>
            <span className="text-body-2-regular text-text-secondary">{info?.blurb}</span>
          </div>
        </div>
        <Input
          label="Name"
          hint="How this connection shows in the model picker."
          value={name}
          onChange={setName}
          isInvalid={Boolean(problems.name)}
          errorMessage={problems.name}
        />
        {compatible ? (
          <Input
            label="Base URL"
            placeholder="https://llm.example.com/v1"
            type="url"
            value={baseUrl}
            onChange={setBaseUrl}
            isInvalid={Boolean(problems.base_url)}
            errorMessage={problems.base_url}
            hint="The address the OpenAI-style /v1 endpoints live under."
          />
        ) : null}
        <div className="flex flex-col gap-1.5">
          <Input
            label={info?.keyLabel || "API key"}
            placeholder={info?.keyPlaceholder}
            type="password"
            autoComplete="off"
            value={apiKey}
            onChange={setApiKey}
            isInvalid={Boolean(problems.api_key)}
            errorMessage={problems.api_key}
            hint="Stored encrypted. Only its last four characters show again."
          />
          {descriptor.docs_url ? (
            <LinkButton
              href={descriptor.docs_url}
              target="_blank"
              rel="noopener noreferrer"
              size="small"
              trailingIcon={RiExternalLinkLine}
              className="self-start"
            >
              Get a key
            </LinkButton>
          ) : null}
        </div>
        {problems.form ? (
          <p
            role="alert"
            className="flex items-start gap-2 rounded-xl bg-notification-error-background p-3 text-body-2-regular text-notification-error-foreground"
          >
            <RiErrorWarningLine aria-hidden className="mt-0.5 size-4 shrink-0" />
            <span>{problems.form}</span>
          </p>
        ) : null}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onCancel} disabled={busy}>
          Cancel
        </Button>
        <Button type="submit" loading={busy}>
          {busy ? "Testing…" : "Test & save"}
        </Button>
      </ModalFooter>
    </form>
  );
}
