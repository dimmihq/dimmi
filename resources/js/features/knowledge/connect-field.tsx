/*
 * Choosing the connected account a source reads through. Each account
 * the workspace connected to the service is a card with the service's
 * logo, and the only one is picked at once. Services are connected on
 * the integrations page alone, so a service with no account links
 * there instead of connecting here.
 */
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";

import { LinkButton } from "@/components/base/buttons/link-button";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import { Spinner } from "@/components/base/spinner/spinner";
import { integrationsQuery } from "@/lib/api/integrations";
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
  const navigate = useNavigate();
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
      {accounts.length === 0 ? (
        <span>
          <LinkButton
            size="xs"
            onClick={() => navigate({ to: "/d/settings/$section", params: { section: "integrations" } })}
          >
            Connect {kind.name} in Integrations
          </LinkButton>
        </span>
      ) : null}
    </div>
  );
}
