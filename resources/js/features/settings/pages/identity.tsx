/*
 * The secrets a workspace's server signs messenger visitors' identity
 * tokens with, and the code that signs them. A new shared secret shows
 * once, in the dialog that made it, with its key id; rotating makes a
 * new one and keeps the old one working for 7 days, so a deploy can
 * switch without failed sign-ins. Revoking stops a secret at once and
 * signs out every visitor it verified. A secret can also point at an
 * auth provider's key set, so the provider's own tokens sign customers
 * in. Above the secrets sits how strict sign-in is, picked in a draft
 * and saved from the bar at the foot; leaving with it unsaved asks
 * first. Below, a checker that explains a pasted token.
 */
import { RiCheckLine, RiFileCopyLine, RiFingerprintLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { Textarea } from "@/components/base/textarea/textarea";
import { ApiError } from "@/lib/api/client";
import {
  type CreatedIdentitySecret,
  createIdentitySecret,
  type IdentityAlgorithm,
  type IdentitySecret,
  identitySecretsQuery,
  revokeIdentitySecret,
  rotateIdentitySecret,
} from "@/lib/api/identity";

import { Action, Choice } from "../controls";
import { IdentityChecker } from "../identity-checker";
import { type SnippetLanguage, secretDetail, secretState, signingSnippet, snippetLanguages } from "../identity-format";
import { IdentityModeSection } from "../identity-mode";
import { useIdentityModeDraft } from "../identity-mode-draft";
import { Empty, Glyph, Item, RowsSkeleton, Section, SettingsPage } from "../layout";
import { SaveBar } from "../save-bar";
import { Snippet } from "../snippet";
import { useAccount } from "../use-account";

type Dialog =
  | { kind: "create" }
  | { kind: "created"; secret: CreatedIdentitySecret }
  | { kind: "revoke"; secret: IdentitySecret }
  | null;

const algorithms: Record<string, IdentityAlgorithm> = {
  "Shared secret (HS256)": "HS256",
  "RSA public key (RS256)": "RS256",
  "P-256 public key (ES256)": "ES256",
  "Ed25519 public key (EdDSA)": "EdDSA",
  "Auth provider key set (JWKS)": "RS256",
};

const providerAlgorithms: IdentityAlgorithm[] = ["RS256", "ES256", "EdDSA"];

export function IdentitySettings() {
  const me = useAccount();
  const workspace = me.workspace.id;
  const queryClient = useQueryClient();
  const inWorkspace = workspace !== "";
  const { data: secrets = [], isPending } = useQuery({ ...identitySecretsQuery(workspace), enabled: inWorkspace });
  const [dialog, setDialog] = useState<Dialog>(null);
  const [language, setLanguage] = useState<SnippetLanguage>("Next.js");
  const refresh = () => queryClient.invalidateQueries({ queryKey: identitySecretsQuery(workspace).queryKey });
  const rotate = useMutation({
    mutationFn: (id: string) => rotateIdentitySecret(workspace, id),
    onSuccess: (secret) => setDialog({ kind: "created", secret }),
    onSettled: refresh,
  });
  const revoke = useMutation({
    mutationFn: (id: string) => revokeIdentitySecret(workspace, id),
    onSettled: refresh,
  });
  const live = secrets.filter((secret) => secret.status !== "revoked");
  const kid = live.find((secret) => secret.status === "active")?.kid ?? "isk_…";
  const failure = rotate.error ?? revoke.error;
  const mode = useIdentityModeDraft(workspace, me.hasPassword);
  useBlocker({
    shouldBlockFn: () => mode.dirty && !window.confirm("Leave without saving your changes?"),
    enableBeforeUnload: () => mode.dirty,
  });

  return (
    <SettingsPage
      title="Identity verification"
      description="Your server signs a token for each signed-in user, so nobody can pose as one of your customers."
      footer={
        <SaveBar
          dirty={mode.dirty}
          saving={mode.save.isPending}
          error={mode.error}
          onSave={() => mode.save.mutate(false)}
          onDiscard={mode.discard}
        />
      }
    >
      {inWorkspace ? <IdentityModeSection workspace={workspace} draft={mode} /> : null}
      <Section
        title="Secrets"
        description="Up to 5 at once. A rotated secret keeps working for 7 days."
        action={inWorkspace ? <Action onPress={() => setDialog({ kind: "create" })}>New secret</Action> : null}
      >
        {failure ? (
          <div className="py-2.5 pe-2.5">
            <Notification status="error" title={failure.message} dismissible={false} role="alert" className="w-full" />
          </div>
        ) : null}
        {inWorkspace && isPending ? <RowsSkeleton rows={1} /> : null}
        {inWorkspace && !isPending && secrets.length === 0 ? (
          <Empty>No secrets yet. Create one to sign in your customers.</Empty>
        ) : null}
        {secrets.map((secret) => (
          <SecretRow
            key={secret.id}
            secret={secret}
            rotating={rotate.isPending && rotate.variables === secret.id}
            onRotate={() => rotate.mutate(secret.id)}
            onRevoke={() => setDialog({ kind: "revoke", secret })}
          />
        ))}
      </Section>
      <Section
        title="Sign tokens on your server"
        description="Answer the signed-in user's token from an endpoint on your site, then point the messenger at it."
      >
        <div className="flex flex-col gap-3 py-2.5 pe-2.5">
          <Choice
            label="Language"
            options={snippetLanguages}
            value={language}
            onChange={(v) => setLanguage(v as SnippetLanguage)}
          />
          <Snippet
            code={signingSnippet(language, kid)}
            hint={
              language === "Script tag"
                ? "Add it to your messenger snippet."
                : "Keep the secret in DIMMI_IDENTITY_SECRET."
            }
            className="bg-background-secondary-default"
          />
        </div>
      </Section>
      {inWorkspace ? <IdentityChecker workspace={workspace} /> : null}

      {dialog?.kind === "create" ? (
        <CreateSecretDialog
          workspace={workspace}
          onClose={() => setDialog(null)}
          onCreated={(secret) => {
            void refresh();
            setDialog({ kind: "created", secret });
          }}
        />
      ) : null}
      {dialog?.kind === "created" ? <CreatedDialog secret={dialog.secret} onClose={() => setDialog(null)} /> : null}
      <AlertDialog
        isOpen={dialog?.kind === "revoke"}
        onOpenChange={(open) => !open && setDialog(null)}
        title={dialog?.kind === "revoke" ? `Revoke ${dialog.secret.name}?` : "Revoke this secret?"}
        description="Tokens signed with it stop working at once and every customer it signed in is signed out. This cannot be undone."
        confirmLabel="Revoke"
        onConfirm={() => {
          if (dialog?.kind === "revoke") {
            revoke.mutate(dialog.secret.id);
          }
        }}
      />
    </SettingsPage>
  );
}

function SecretRow({
  secret,
  rotating,
  onRotate,
  onRevoke,
}: {
  secret: IdentitySecret;
  rotating: boolean;
  onRotate: () => void;
  onRevoke: () => void;
}) {
  const state = secretState(secret);

  return (
    <Item mark={<Glyph icon={RiFingerprintLine} />} title={secret.name} detail={secretDetail(secret)}>
      {state ? (
        <Chip variant="caption" color={secret.status === "revoked" ? "gray" : "orange"}>
          {state}
        </Chip>
      ) : null}
      {secret.jwks_error && secret.status !== "revoked" ? (
        <Chip variant="caption" color="orange">
          Key set unreachable
        </Chip>
      ) : null}
      {secret.status === "active" && secret.algorithm === "HS256" ? (
        <Action onPress={onRotate} loading={rotating}>
          Rotate
        </Action>
      ) : null}
      {secret.status !== "revoked" ? <Action onPress={onRevoke}>Revoke</Action> : null}
    </Item>
  );
}

function CreateSecretDialog({
  workspace,
  onClose,
  onCreated,
}: {
  workspace: string;
  onClose: () => void;
  onCreated: (secret: CreatedIdentitySecret) => void;
}) {
  const [name, setName] = useState("Production");
  const [kind, setKind] = useState("Shared secret (HS256)");
  const [publicKey, setPublicKey] = useState("");
  const [jwksUrl, setJwksUrl] = useState("");
  const [issuer, setIssuer] = useState("");
  const [providerAlgorithm, setProviderAlgorithm] = useState<IdentityAlgorithm>("RS256");
  const [failure, setFailure] = useState<Error | null>(null);
  const [pending, setPending] = useState(false);
  const provider = kind === "Auth provider key set (JWKS)";
  const algorithm = provider ? providerAlgorithm : (algorithms[kind] ?? "HS256");
  const shared = algorithm === "HS256";
  const material = () => {
    if (provider) {
      return { jwks_url: jwksUrl.trim(), ...(issuer.trim() ? { issuer: issuer.trim() } : {}) };
    }
    return shared ? {} : { public_key: publicKey };
  };
  const missing = provider ? jwksUrl.trim() === "" : !shared && publicKey.trim() === "";

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFailure(null);
    setPending(true);
    try {
      onCreated(await createIdentitySecret(workspace, { name, algorithm, ...material() }));
    } catch (error) {
      setFailure(error instanceof Error ? error : new Error("Something went wrong. Try again."));
    } finally {
      setPending(false);
    }
  };

  const fields = failure instanceof ApiError ? failure.fields : {};
  const general =
    failure && !fields.name && !fields.public_key && !fields.jwks_url && !fields.issuer ? failure.message : null;

  return (
    <Modal isOpen size="lg" onOpenChange={(open) => !open && onClose()}>
      <form onSubmit={submit} className="flex min-h-0 flex-col gap-4">
        <ModalHeader
          icon={RiFingerprintLine}
          title="New identity secret"
          description="Your server signs each signed-in user's token with it."
        />
        <ModalBody className="flex flex-col gap-5">
          {general ? (
            <Notification status="error" title={general} dismissible={false} role="alert" className="w-full" />
          ) : null}
          <Input
            label="Name"
            value={name}
            onChange={setName}
            isRequired
            maxLength={60}
            isInvalid={Boolean(fields.name)}
            errorMessage={fields.name}
          />
          <div className="flex flex-col gap-1.5">
            <span className="text-body-medium text-text-secondary">Kind</span>
            <Choice label="Kind" options={Object.keys(algorithms)} value={kind} onChange={setKind} />
          </div>
          {provider ? (
            <ProviderFields
              algorithm={providerAlgorithm}
              onAlgorithm={setProviderAlgorithm}
              jwksUrl={jwksUrl}
              onJwksUrl={setJwksUrl}
              issuer={issuer}
              onIssuer={setIssuer}
              fields={fields}
            />
          ) : null}
          {shared || provider ? null : (
            <Textarea
              label="Public key"
              placeholder="-----BEGIN PUBLIC KEY-----"
              value={publicKey}
              onChange={setPublicKey}
              rows={6}
              isRequired
              fieldClassName="font-mono"
              isInvalid={Boolean(fields.public_key)}
              hint={fields.public_key ?? "PEM text. Your private key never leaves your server."}
            />
          )}
        </ModalBody>
        <ModalFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={pending || name.trim() === "" || missing}>
            {pending ? "Creating…" : "Create secret"}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}

function ProviderFields({
  algorithm,
  onAlgorithm,
  jwksUrl,
  onJwksUrl,
  issuer,
  onIssuer,
  fields,
}: {
  algorithm: IdentityAlgorithm;
  onAlgorithm: (algorithm: IdentityAlgorithm) => void;
  jwksUrl: string;
  onJwksUrl: (value: string) => void;
  issuer: string;
  onIssuer: (value: string) => void;
  fields: Record<string, string>;
}) {
  return (
    <>
      <Input
        label="Key set address"
        placeholder="https://your-app.clerk.accounts.dev/.well-known/jwks.json"
        value={jwksUrl}
        onChange={onJwksUrl}
        isRequired
        inputDir="ltr"
        isInvalid={Boolean(fields.jwks_url)}
        errorMessage={fields.jwks_url}
        hint="Clerk, Auth0, Supabase and most providers publish one."
      />
      <Input
        label="Issuer"
        placeholder="Read from the address when left empty"
        value={issuer}
        onChange={onIssuer}
        inputDir="ltr"
        isInvalid={Boolean(fields.issuer)}
        errorMessage={fields.issuer}
        hint="The iss claim of your provider's tokens."
      />
      <div className="flex flex-col gap-1.5">
        <span className="text-body-medium text-text-secondary">Algorithm</span>
        <Choice
          label="Algorithm"
          options={providerAlgorithms}
          value={algorithm}
          onChange={(v) => onAlgorithm(v as IdentityAlgorithm)}
        />
      </div>
    </>
  );
}

function CreatedDialog({ secret, onClose }: { secret: CreatedIdentitySecret; onClose: () => void }) {
  return (
    <Modal isOpen isDismissable={false} onOpenChange={(open) => !open && onClose()}>
      <ModalHeader
        icon={RiFingerprintLine}
        title={createdTitle(secret)}
        description={createdDescription(secret)}
        showCloseButton={false}
      />
      <ModalBody className="flex flex-col gap-4">
        {secret.secret ? <CopyField label="Secret" value={secret.secret} /> : null}
        {secret.kid ? <CopyField label="Key id (kid header)" value={secret.kid} /> : null}
      </ModalBody>
      <ModalFooter>
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
    </Modal>
  );
}

function CopyField({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
  };

  return (
    <div className="flex items-end gap-2">
      <Input
        label={label}
        value={value}
        isReadOnly
        inputDir="ltr"
        className="min-w-0 flex-1"
        fieldClassName="font-mono"
      />
      <IconButton
        icon={copied ? RiCheckLine : RiFileCopyLine}
        aria-label={copied ? "Copied" : `Copy ${label.toLowerCase()}`}
        onClick={() => void copy()}
      />
    </div>
  );
}

function createdTitle(secret: CreatedIdentitySecret): string {
  if (secret.secret) {
    return "Copy your new secret";
  }
  return secret.source === "jwks" ? "Key set connected" : "Public key added";
}

function createdDescription(secret: CreatedIdentitySecret): string {
  if (secret.secret) {
    return "It shows only now. Put it in your server's environment as DIMMI_IDENTITY_SECRET.";
  }
  if (secret.source === "jwks") {
    return "Pass your provider's session token to the messenger as is; dimmi checks it against the key set.";
  }
  return "Sign tokens with your private key and this key id.";
}
