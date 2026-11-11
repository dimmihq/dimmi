/*
 * Choosing the connected account a source reads through. Each account
 * the workspace already connected to the service is a card with the
 * service's logo, and the only one is picked at once. Under them, one
 * row connects another account through the service's own consent
 * screen; the browser comes back to the add flow where it left, with
 * the new account picked. A service hosted per customer, such as
 * Zendesk, asks for its subdomain on that same row first. Connecting an
 * account that is already connected renews it rather than adding it
 * twice.
 */
import { RiAddLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Spinner } from "@/components/base/spinner/spinner";
import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";
import { integrationsQuery, startInstallation } from "@/lib/api/integrations";
import type { SchemaProperty, SourceType } from "@/lib/api/knowledge-sources";

import { SourceLogo } from "./logos";
import type { FormValues } from "./model";

/*
 * AccountStep is the first step of adding a connected service's source.
 */
export function AccountStep({
  workspace,
  kind,
  values,
  onChange,
}: {
  workspace: string;
  kind: SourceType;
  values: FormValues;
  onChange: (values: FormValues) => void;
}) {
  const [name, prop] = Object.entries(kind.schema.properties).find(([, p]) => p.format === "x-integration") ?? [];
  if (!name || !prop || !kind.integration) {
    return null;
  }

  return (
    <AccountField
      workspace={workspace}
      kind={kind}
      provider={kind.integration}
      prop={prop}
      value={values[name]}
      onChange={(value) => onChange({ ...values, [name]: value, selected: [] })}
    />
  );
}

export function AccountField({
  workspace,
  kind,
  provider,
  prop,
  value,
  onChange,
}: {
  workspace: string;
  kind: SourceType;
  provider: string;
  prop: SchemaProperty;
  value: unknown;
  onChange: (value: string) => void;
}) {
  const installations = useQuery(integrationsQuery(workspace));
  const accounts = (installations.data ?? []).filter((i) => i.provider === provider && i.status === "active");
  const picked = typeof value === "string" && value ? value : null;
  const only = accounts.length === 1 ? accounts[0].id : null;

  useEffect(() => {
    if (!picked && only) {
      onChange(only);
    }
  }, [picked, only, onChange]);

  if (installations.isPending) {
    return (
      <div className="flex items-center gap-2 text-body-regular text-text-secondary">
        <Spinner size="sm" aria-hidden />
        Looking for connected {kind.name} accounts…
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {accounts.length > 0 ? (
        <RadioGroup aria-label={prop.title ?? `${kind.name} account`} value={picked} onChange={onChange}>
          {accounts.map((a) => (
            <RadioCard
              key={a.id}
              value={a.id}
              title={
                <span className="flex items-center gap-2">
                  <SourceLogo logo={kind.logo} size="sm" />
                  {a.name}
                </span>
              }
            />
          ))}
        </RadioGroup>
      ) : (
        <p className="text-body-regular text-text-secondary">{prop.description}</p>
      )}
      <ConnectRow workspace={workspace} kind={kind} provider={provider} prop={prop} more={accounts.length > 0} />
    </div>
  );
}

/*
 * ConnectRow sends the browser to the service's consent screen, after
 * the subdomain for a service hosted per customer.
 */
function ConnectRow({
  workspace,
  kind,
  provider,
  prop,
  more,
}: {
  workspace: string;
  kind: SourceType;
  provider: string;
  prop: SchemaProperty;
  more: boolean;
}) {
  const [account, setAccount] = useState("");
  const hosted = Boolean(prop["x-account"]);
  const label = more ? `Connect another ${kind.name} account` : `Connect ${kind.name}`;

  const connect = useMutation({
    mutationFn: () =>
      startInstallation(workspace, provider, {
        features: ["knowledge"],
        return_to: `${window.location.pathname}?add=${encodeURIComponent(kind.type)}`,
        account: account.trim() || undefined,
      }),
    onSuccess: (started) => window.location.assign(started.authorize_url),
    onError: (err) => toast.error(err instanceof ApiError ? err.message : `${kind.name} could not be connected.`),
  });

  if (!hosted) {
    return (
      <div>
        <Button
          variant={more ? "secondary" : "primary"}
          leadingIcon={RiAddLine}
          loading={connect.isPending}
          onClick={() => connect.mutate()}
        >
          {label}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <span className="text-body-2-medium text-text-primary">{label}</span>
      <div className="flex items-center gap-2">
        <Input
          aria-label={`${kind.name} subdomain`}
          placeholder="your-subdomain"
          value={account}
          onChange={setAccount}
          className="min-w-0 flex-1"
        />
        <Button
          variant={more ? "secondary" : "primary"}
          loading={connect.isPending}
          disabled={account.trim() === ""}
          onClick={() => connect.mutate()}
          className="shrink-0"
        >
          Connect
        </Button>
      </div>
      <span className="text-body-2-regular text-text-tertiary">The part before .{provider}.com</span>
    </div>
  );
}
