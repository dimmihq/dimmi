/*
 * The page a person lands on when an app, such as an AI agent, asks to
 * act for them. It says which app is asking and how far dimmi can vouch
 * for it, warns when the app runs on the person's own computer, lets
 * them pick the workspace and the access to give, and sends them back
 * to the app with the answer. A request the server refuses says why,
 * and offers the way back to the app when the app may hear of it.
 */
import { RiAlertLine, RiErrorWarningLine } from "@remixicon/react";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { Select, SelectItem } from "@/components/base/select/select";
import { abilityLabel } from "@/features/settings/token-format";
import { ApiError } from "@/lib/api/client";
import { type AppRequest, appRequestQuery, decideAppRequest } from "@/lib/api/connected-apps";

import { AuthCard, AuthHeader, AuthStatus } from "./auth-frame";

/*
 * Notice is a caution about the app, shown above the choices.
 */
function Notice({ children }: { children: string }) {
  return (
    <p className="flex items-start gap-2 rounded-xl bg-background-secondary-default p-3 text-body-2-regular text-text-secondary dark:bg-background-tertiary-default">
      <RiAlertLine aria-hidden className="mt-0.5 size-4 shrink-0 text-foreground-icon-secondary" />
      <span>{children}</span>
    </p>
  );
}

/*
 * Refused explains why a request cannot go ahead, with the way back to
 * the app when there is one.
 */
function Refused({ error }: { error: unknown }) {
  const message = error instanceof Error ? error.message : "This request could not be read.";
  const back = error instanceof ApiError ? error.details.redirect_to : undefined;

  return (
    <AuthStatus
      icon={RiErrorWarningLine}
      tone="error"
      title="This app can't connect"
      description={`${message} Start connecting again from the app.`}
    >
      {typeof back === "string" ? (
        <Button className="mt-6 w-full" variant="secondary" onClick={() => window.location.assign(back)}>
          Return to the app
        </Button>
      ) : null}
    </AuthStatus>
  );
}

function Choices({ query, app }: { query: string; app: AppRequest }) {
  const [workspace, setWorkspace] = useState(app.workspaces[0]?.id ?? "");
  const [scopes, setScopes] = useState(app.scopes);
  const [pending, setPending] = useState<"allow" | "deny" | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  const decide = async (approve: boolean) => {
    setPending(approve ? "allow" : "deny");
    setFailure(null);
    try {
      const { redirect_to } = await decideAppRequest(
        query,
        approve ? { approve: true, workspace, scopes } : { approve: false },
      );
      window.location.assign(redirect_to);
    } catch (error) {
      setFailure(error);
      setPending(null);
    }
  };
  const toggle = (scope: string, on: boolean) =>
    setScopes((current) => (on ? [...current, scope] : current.filter((s) => s !== scope)));

  if (app.workspaces.length === 0) {
    return (
      <p className="mt-6 text-body-regular text-text-secondary">
        You are not in a workspace yet, so there is nothing to connect this app to.
      </p>
    );
  }

  return (
    <div className="mt-6 flex flex-col gap-5">
      {app.workspaces.length > 1 ? (
        <Select
          label="Workspace"
          selectedKey={workspace}
          onSelectionChange={(key) => setWorkspace(String(key))}
          popoverClassName="w-72"
        >
          {app.workspaces.map((w) => (
            <SelectItem key={w.id} id={w.id} textValue={w.name}>
              {w.name}
            </SelectItem>
          ))}
        </Select>
      ) : (
        <p className="text-body-regular text-text-secondary">
          Workspace: <span className="text-text-primary">{app.workspaces[0]?.name}</span>
        </p>
      )}
      <fieldset className="flex flex-col gap-2.5">
        <legend className="mb-2.5 text-body-medium text-text-primary">It will be able to</legend>
        {app.scopes.map((scope) => (
          <Checkbox key={scope} size="sm" isSelected={scopes.includes(scope)} onChange={(on) => toggle(scope, on)}>
            {abilityLabel(scope)}
          </Checkbox>
        ))}
        <p className="text-caption-1-regular text-text-tertiary">
          It can never do more than your role in the workspace allows.
        </p>
      </fieldset>
      {failure ? (
        <p role="alert" className="text-body-2-regular text-text-error-primary">
          {failure instanceof Error ? failure.message : "That did not work. Try again."}
        </p>
      ) : null}
      <div className="flex flex-col gap-2">
        <Button
          className="w-full"
          loading={pending === "allow"}
          disabled={pending !== null || scopes.length === 0}
          onClick={() => void decide(true)}
        >
          Allow
        </Button>
        <Button
          className="w-full"
          variant="secondary"
          loading={pending === "deny"}
          disabled={pending !== null}
          onClick={() => void decide(false)}
        >
          Deny
        </Button>
      </div>
    </div>
  );
}

export function ConnectAppPage({ query }: { query: string }) {
  const { data: app, error, isPending } = useQuery(appRequestQuery(query));

  if (error) {
    return <Refused error={error} />;
  }

  if (isPending || !app) {
    return (
      <AuthCard>
        <AuthHeader title="Connecting an app" description="Reading the app's request…" />
      </AuthCard>
    );
  }

  return (
    <AuthCard>
      <AuthHeader
        title={`Connect ${app.client_name}`}
        description={`${app.client_name} wants to work in dimmi as you. When you allow it, you go back to ${app.redirect_host}.`}
      />
      <div className="mt-5 flex flex-col gap-2">
        {app.self_registered ? (
          <Notice>dimmi can't verify who made this app. Only continue if you trust where you started it.</Notice>
        ) : null}
        {app.loopback ? (
          <Notice>This app runs on your computer. Only continue if you started this connection yourself.</Notice>
        ) : null}
      </div>
      <Choices query={query} app={app} />
    </AuthCard>
  );
}
