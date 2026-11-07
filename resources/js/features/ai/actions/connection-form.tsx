/*
 * The details of a connection, for adding one or changing it: a name,
 * where it lives and how dimmi signs in. A REST API also takes default
 * headers, variables its actions read as {connection.key} and a timeout,
 * tucked under More settings. Saving tests the connection straight away
 * and says how it went; an MCP server then turns its tools into draft
 * actions. Secrets are write-only: when changing a connection, an empty
 * secret keeps the stored one, shown only by its last four characters.
 */
import { RiArrowDownSLine, RiErrorWarningLine } from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { ModalBody, ModalFooter } from "@/components/base/modal/modal";
import { Select, SelectItem } from "@/components/base/select/select";
import { toast } from "@/components/base/toast/toast";
import { cx } from "@/utils/cx";

import {
  type ActionConnection,
  type AuthType,
  actionsKey,
  type ConnectionKind,
  createConnection,
  syncTools,
  testConnection,
  updateConnection,
} from "./api";
import {
  authChoices,
  checkConnection,
  connectionBody,
  connectionRefusal,
  initialValues,
  kindInfo,
  type ConnectionProblems as Problems,
  type ConnectionValues as Values,
} from "./connection-values";
import { authLabels, syncLine } from "./format";
import { PairRows } from "./parts";

const timeouts = [2000, 5000, 8000, 10000, 15000];

/*
 * useSave stores the connection, tests it, and for an MCP server turns
 * its tools into draft actions, saying in a toast how each step went.
 */
function useSave(workspace: string, kind: ConnectionKind, connection: ActionConnection | undefined) {
  const client = useQueryClient();

  return async (values: Values) => {
    const body = connectionBody(kind, values);
    const saved = connection
      ? await updateConnection(workspace, connection.id, body)
      : await createConnection(workspace, body);
    const test = await testConnection(workspace, saved.id).catch(() => null);
    if (test?.status === "ok" && kind === "mcp") {
      const sync = await syncTools(workspace, saved.id).catch(() => null);
      toast.success(`${saved.name} connected`, {
        description: sync ? syncLine(sync) : "Sync its tools from the connection's menu.",
      });
    } else if (test?.status === "ok") {
      toast.success(`${saved.name} connected`, { description: "Add the actions dimmi may call through it." });
    } else {
      toast.info(`${saved.name} saved`, {
        description: "The connection test did not pass. Check the address and secret, then test again.",
      });
    }
    await client.invalidateQueries({ queryKey: actionsKey(workspace) });
  };
}

/*
 * ConnectionForm asks for a connection's details and saves them.
 */
export function ConnectionForm({
  workspace,
  kind,
  connection,
  onCancel,
  onDone,
}: {
  workspace: string;
  kind: ConnectionKind;
  connection?: ActionConnection;
  onCancel: () => void;
  onDone: () => void;
}) {
  const [values, setValues] = useState(() => initialValues(kind, connection));
  const [problems, setProblems] = useState<Problems>({});
  const [busy, setBusy] = useState(false);
  const save = useSave(workspace, kind, connection);
  const set = (change: Partial<Values>) => setValues((before) => ({ ...before, ...change }));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const missing = checkConnection(values, connection !== undefined);
    setProblems(missing);
    if (Object.keys(missing).length > 0) {
      return;
    }
    setBusy(true);
    try {
      await save(values);
      onDone();
    } catch (error) {
      setProblems(connectionRefusal(error));
    } finally {
      setBusy(false);
    }
  };

  return (
    <form className="contents" onSubmit={submit} noValidate>
      <ModalBody className="flex flex-col gap-5 pb-1">
        <Input
          label="Name"
          placeholder={kind === "mcp" ? "Billing tools" : "Billing API"}
          hint="How teammates find it. The agent never sees it."
          value={values.name}
          onChange={(name) => set({ name })}
          isInvalid={Boolean(problems.name)}
          errorMessage={problems.name}
        />
        <Input
          label={kind === "mcp" ? "Server URL" : "Base URL"}
          placeholder={kind === "mcp" ? "https://mcp.example.com/mcp" : "https://api.example.com/v1"}
          type="url"
          inputDir="ltr"
          hint={
            kind === "mcp"
              ? "The Streamable HTTP endpoint of the server."
              : "Every action's path is added to this address."
          }
          value={values.url}
          onChange={(url) => set({ url })}
          isInvalid={Boolean(problems.url)}
          errorMessage={problems.url}
        />
        <AuthFields kind={kind} values={values} problems={problems} hint={connection?.secret_hint} onChange={set} />
        {kind === "http" ? <MoreSettings values={values} onChange={set} /> : null}
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
          {busy ? "Testing…" : connection ? "Save & test" : `Connect ${kindInfo[kind].name}`}
        </Button>
      </ModalFooter>
    </form>
  );
}

/*
 * AuthFields picks how dimmi signs in and asks for what that way needs.
 */
function AuthFields({
  kind,
  values,
  problems,
  hint,
  onChange,
}: {
  kind: ConnectionKind;
  values: Values;
  problems: Problems;
  hint?: string | null;
  onChange: (change: Partial<Values>) => void;
}) {
  const keep = hint ? `Leave empty to keep the stored one (${hint}).` : "Stored encrypted. Never shown again.";

  return (
    <div className="flex flex-col gap-4 rounded-2xl bg-background-secondary-default p-3">
      <Select
        label="Authentication"
        selectedKey={values.auth}
        onSelectionChange={(key) => onChange({ auth: String(key) as AuthType })}
      >
        {authChoices[kind].map((auth) => (
          <SelectItem key={auth} id={auth} textValue={authLabels[auth]}>
            {authLabels[auth]}
          </SelectItem>
        ))}
      </Select>
      {values.auth === "api_key_header" ? (
        <Input
          label="Header name"
          inputDir="ltr"
          value={values.header}
          onChange={(header) => onChange({ header })}
          isInvalid={Boolean(problems.header)}
          errorMessage={problems.header}
        />
      ) : null}
      {values.auth === "api_key_query" ? (
        <Input
          label="Query parameter"
          inputDir="ltr"
          value={values.param}
          onChange={(param) => onChange({ param })}
          hint="Keys in addresses can end up in logs. Prefer a header when the API allows it."
        />
      ) : null}
      {values.auth === "basic" ? (
        <Input
          label="Username"
          inputDir="ltr"
          autoComplete="off"
          value={values.username}
          onChange={(username) => onChange({ username })}
          isInvalid={Boolean(problems.username)}
          errorMessage={problems.username}
        />
      ) : null}
      {values.auth === "none" ? null : (
        <Input
          label={values.auth === "basic" ? "Password" : values.auth === "bearer" ? "Token" : "API key"}
          type="password"
          autoComplete="off"
          inputDir="ltr"
          value={values.auth === "basic" ? values.password : values.token}
          onChange={(secret) => onChange(values.auth === "basic" ? { password: secret } : { token: secret })}
          isInvalid={Boolean(problems.token ?? problems.password)}
          errorMessage={problems.token ?? problems.password}
          hint={keep}
        />
      )}
    </div>
  );
}

/*
 * MoreSettings holds what most REST APIs never need: headers sent with
 * every request, variables, and how long to wait.
 */
function MoreSettings({ values, onChange }: { values: Values; onChange: (change: Partial<Values>) => void }) {
  const [open, setOpen] = useState(values.headers.length > 0 || values.variables.length > 0);

  return (
    <div className="flex flex-col gap-4">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen(!open)}
        className="flex items-center gap-1 self-start rounded-md text-body-2-medium text-text-secondary outline-none hover:text-text-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        More settings
        <RiArrowDownSLine
          aria-hidden
          className={cx("size-4 transition-transform duration-150", open && "rotate-180")}
        />
      </button>
      {open ? (
        <>
          <div className="flex flex-col gap-1.5">
            <span className="text-body-2-medium text-text-primary">Default headers</span>
            <PairRows
              label="Default header"
              rows={values.headers}
              onChange={(headers) => onChange({ headers })}
              namePlaceholder="Accept-Version"
              valuePlaceholder="2024-06-01"
              addLabel="Add header"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <span className="text-body-2-medium text-text-primary">Variables</span>
            <span className="text-body-2-regular text-text-secondary">
              Values that are not secret, used in actions as {"{connection.name}"}.
            </span>
            <PairRows
              label="Variable"
              rows={values.variables}
              onChange={(variables) => onChange({ variables })}
              namePlaceholder="region"
              valuePlaceholder="eu"
              addLabel="Add variable"
            />
          </div>
          <Select
            label="Timeout"
            selectedKey={String(values.timeout)}
            onSelectionChange={(key) => onChange({ timeout: Number(key) })}
          >
            {timeouts.map((ms) => (
              <SelectItem key={ms} id={String(ms)} textValue={`${ms / 1000} seconds`}>
                {`${ms / 1000} seconds`}
              </SelectItem>
            ))}
          </Select>
        </>
      ) : null}
    </div>
  );
}
