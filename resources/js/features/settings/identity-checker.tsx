/*
 * A developer pastes a token their server minted and reads what the
 * messenger would make of it: valid or not, why, the slip that explains
 * a failure, and the claims. The check is dry, so the token still works
 * afterwards, and it is never kept. The signature is never shown back.
 */
import { RiCheckboxCircleLine, RiErrorWarningLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Textarea } from "@/components/base/textarea/textarea";
import { checkIdentityToken, type IdentityValidation } from "@/lib/api/identity";

import { Section } from "./layout";

export function IdentityChecker({ workspace }: { workspace: string }) {
  const [token, setToken] = useState("");
  const check = useMutation({ mutationFn: (value: string) => checkIdentityToken(workspace, value) });

  return (
    <Section title="Check a token" description="Paste a token your server signed. It is checked without using it up.">
      <div className="flex flex-col gap-3 py-2.5 pe-2.5">
        <Textarea
          label="Identity token"
          placeholder="eyJhbGciOi…"
          value={token}
          onChange={setToken}
          rows={3}
          fieldClassName="font-mono"
        />
        <div>
          <Button
            size="small"
            disabled={token.trim() === "" || check.isPending}
            onClick={() => check.mutate(token.trim())}
          >
            {check.isPending ? "Checking…" : "Check token"}
          </Button>
        </div>
        {check.error ? <p className="text-body-2-regular text-text-error-primary">{check.error.message}</p> : null}
        {check.data ? <Diagnosis result={check.data} /> : null}
      </div>
    </Section>
  );
}

function Diagnosis({ result }: { result: IdentityValidation }) {
  const Icon = result.valid ? RiCheckboxCircleLine : RiErrorWarningLine;
  const title = result.valid ? "Valid token" : (result.message ?? "Invalid token");
  const preview = result.contact_preview;

  return (
    <div className="flex flex-col gap-3 rounded-xl border border-border-button-default bg-background-primary-default p-4">
      <div className="flex items-start gap-2">
        <Icon
          aria-hidden
          className={
            result.valid ? "size-5 shrink-0 text-foreground-icon-primary" : "size-5 shrink-0 text-text-error-primary"
          }
        />
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="text-body-medium text-text-primary">{title}</span>
          {result.code ? <span className="font-mono text-body-2-regular text-text-tertiary">{result.code}</span> : null}
        </div>
      </div>
      {result.mistake ? <p className="text-body-2-medium text-text-primary">{result.mistake.hint}</p> : null}
      {!result.mistake && result.hint ? <p className="text-body-2-regular text-text-secondary">{result.hint}</p> : null}
      {result.warnings.map((w) => (
        <p key={w.code} className="text-body-2-regular text-text-secondary">
          {w.hint}
        </p>
      ))}
      {preview ? (
        <p className="text-body-2-regular text-text-secondary">
          {preview.action === "reuse"
            ? "Signs in a customer you already have."
            : "Would create a new customer on first sign-in."}
        </p>
      ) : null}
      {result.claims ? (
        <pre
          dir="ltr"
          className="overflow-x-auto rounded-lg bg-background-secondary-default p-3 font-mono text-caption-1-regular text-text-secondary"
        >
          {JSON.stringify({ header: result.header, claims: result.claims }, null, 2)}
        </pre>
      ) : null}
    </div>
  );
}
