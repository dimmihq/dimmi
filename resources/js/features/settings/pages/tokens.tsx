/*
 * The person's API tokens in the workspace on screen: what each may do,
 * when it was last used and when it expires, minting a new one, and
 * revoking one. A new token's secret shows once, in the dialog that
 * minted it, with a button to copy it, and is gone when it closes.
 * Minting asks for the password again when the last sign-in is a while
 * ago, since a token is a standing key to the workspace.
 */
import { RiCheckLine, RiFileCopyLine, RiKey2Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { IconButton } from "@/components/base/buttons/icon-button";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { HintText } from "@/components/base/input/hint-text";
import { Input } from "@/components/base/input/input";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { ApiError } from "@/lib/api/client";
import {
  allAbilities,
  createToken,
  type MintedToken,
  type PersonalAccessToken,
  revokeToken,
  tokenAbilitiesQuery,
  tokensQuery,
} from "@/lib/api/tokens";
import { entitlementsQuery } from "@/lib/api/workspace";
import { Action, Choice } from "../controls";
import { Empty, Glyph, Item, Section, SettingsPage } from "../layout";
import { useRecentAuth } from "../recent-auth";
import { abilityLabel, expiryChoices, expiryFor, isLive, rateLimitLine, tokenDetail } from "../token-format";
import { useAccount } from "../use-account";

type Dialog =
  | { kind: "create" }
  | { kind: "secret"; token: MintedToken }
  | { kind: "revoke"; token: PersonalAccessToken }
  | null;

export function TokensSettings() {
  const me = useAccount();
  const workspace = me.workspace.id;
  const queryClient = useQueryClient();
  const inWorkspace = workspace !== "";
  const { data: tokens = [], isPending } = useQuery({ ...tokensQuery(workspace), enabled: inWorkspace });
  const { data: entitlements } = useQuery({ ...entitlementsQuery(workspace), enabled: inWorkspace });
  const [dialog, setDialog] = useState<Dialog>(null);
  const refresh = () => queryClient.invalidateQueries({ queryKey: tokensQuery(workspace).queryKey });
  const revoke = useMutation({
    mutationFn: (id: string) => revokeToken(workspace, id),
    onSettled: refresh,
  });
  const now = new Date();

  return (
    <SettingsPage title="API tokens" description="Tokens act as you, with only the abilities you give them.">
      <Section
        title="Your tokens"
        description={[
          "The secret shows once. Up to 50 working tokens.",
          entitlements ? rateLimitLine(entitlements.api) : "",
        ]
          .filter(Boolean)
          .join(" ")}
        action={inWorkspace ? <Action onPress={() => setDialog({ kind: "create" })}>New token</Action> : null}
      >
        {revoke.error ? (
          <div className="py-2.5 pe-2.5">
            <Notification
              status="error"
              title={revoke.error.message}
              dismissible={false}
              role="alert"
              className="w-full"
            />
          </div>
        ) : null}
        {inWorkspace ? null : <Empty>Tokens work inside a workspace. Join or create one first.</Empty>}
        {inWorkspace && isPending ? <Empty>Loading…</Empty> : null}
        {inWorkspace && !isPending && tokens.length === 0 ? <Empty>No tokens yet</Empty> : null}
        {tokens.map((token) => (
          <Item key={token.id} mark={<Glyph icon={RiKey2Line} />} title={token.name} detail={tokenDetail(token, now)}>
            {isLive(token, now) ? (
              <Action onPress={() => setDialog({ kind: "revoke", token })}>Revoke</Action>
            ) : (
              <Chip variant="caption" color="gray">
                {token.revoked_at ? "Revoked" : "Expired"}
              </Chip>
            )}
          </Item>
        ))}
      </Section>

      {dialog?.kind === "create" ? (
        <CreateTokenDialog
          workspace={workspace}
          hasPassword={me.hasPassword}
          onClose={() => setDialog(null)}
          onCreated={(token) => {
            void refresh();
            setDialog({ kind: "secret", token });
          }}
        />
      ) : null}
      {dialog?.kind === "secret" ? <SecretDialog token={dialog.token} onClose={() => setDialog(null)} /> : null}
      <AlertDialog
        isOpen={dialog?.kind === "revoke"}
        onOpenChange={(open) => !open && setDialog(null)}
        title={dialog?.kind === "revoke" ? `Revoke ${dialog.token.name}?` : "Revoke this token?"}
        description="Anything that uses it stops working at once. This cannot be undone."
        confirmLabel="Revoke"
        onConfirm={() => {
          if (dialog?.kind === "revoke") {
            revoke.mutate(dialog.token.id);
          }
        }}
      />
    </SettingsPage>
  );
}

function CreateTokenDialog({
  workspace,
  hasPassword,
  onClose,
  onCreated,
}: {
  workspace: string;
  hasPassword: boolean;
  onClose: () => void;
  onCreated: (token: MintedToken) => void;
}) {
  const { data: abilities = [] } = useQuery(tokenAbilitiesQuery(workspace));
  const [name, setName] = useState("");
  const [expiry, setExpiry] = useState("90 days");
  const [chosen, setChosen] = useState<string[]>([]);
  const [failure, setFailure] = useState<ApiError | Error | null>(null);
  const [pending, setPending] = useState(false);
  const { run, prompt } = useRecentAuth(hasPassword);
  const everything = chosen.includes(allAbilities);

  const toggle = (ability: string, on: boolean) =>
    setChosen((current) => (on ? [...current, ability] : current.filter((held) => held !== ability)));

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFailure(null);
    setPending(true);

    try {
      const made: { token?: MintedToken } = {};
      const done = await run(async () => {
        made.token = await createToken(workspace, {
          name,
          abilities: chosen,
          expires_at: expiryFor(expiry, new Date()),
        });
      });
      if (done && made.token) {
        onCreated(made.token);
      }
    } catch (error) {
      setFailure(error instanceof Error ? error : new Error("Something went wrong. Try again."));
    } finally {
      setPending(false);
    }
  };

  const fields = failure instanceof ApiError ? failure.fields : {};
  const general = failure && !fields.name && !fields.abilities ? failure.message : null;

  return (
    <>
      <Modal isOpen size="lg" onOpenChange={(open) => !open && onClose()}>
        <form onSubmit={submit} className="flex min-h-0 flex-col gap-4">
          <ModalHeader
            icon={RiKey2Line}
            title="New API token"
            description="The token acts as you in this workspace, with only the abilities you pick."
          />
          <ModalBody className="flex flex-col gap-5">
            {general ? (
              <Notification status="error" title={general} dismissible={false} role="alert" className="w-full" />
            ) : null}
            <Input
              label="Name"
              placeholder="CI deploy"
              value={name}
              onChange={setName}
              isRequired
              maxLength={80}
              isInvalid={Boolean(fields.name)}
              errorMessage={fields.name}
            />
            <div className="flex flex-col gap-1.5">
              <span className="text-body-medium text-text-secondary">Expires</span>
              <Choice
                label="Expires"
                options={expiryChoices.map((choice) => choice.label)}
                value={expiry}
                onChange={setExpiry}
              />
            </div>
            <fieldset className="flex flex-col gap-3">
              <legend className="mb-1.5 text-body-medium text-text-secondary">Abilities</legend>
              <Checkbox isSelected={everything} onChange={(on) => setChosen(on ? [allAbilities] : [])}>
                {abilityLabel(allAbilities)}
              </Checkbox>
              <div className="grid grid-cols-1 gap-x-6 gap-y-2.5 sm:grid-cols-2">
                {abilities.map((group) => (
                  <Checkbox
                    key={group.ability}
                    size="sm"
                    isDisabled={everything || !group.allowed}
                    isSelected={everything ? group.allowed : chosen.includes(group.ability)}
                    onChange={(on) => toggle(group.ability, on)}
                  >
                    {abilityLabel(group.ability)}
                  </Checkbox>
                ))}
              </div>
              {fields.abilities ? (
                <HintText isInvalid role="alert">
                  {fields.abilities}
                </HintText>
              ) : null}
            </fieldset>
          </ModalBody>
          <ModalFooter>
            <Button variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={pending || name.trim() === "" || chosen.length === 0}>
              {pending ? "Creating…" : "Create token"}
            </Button>
          </ModalFooter>
        </form>
      </Modal>
      {prompt}
    </>
  );
}

function SecretDialog({ token, onClose }: { token: MintedToken; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    await navigator.clipboard.writeText(token.plain_text_token);
    setCopied(true);
  };

  return (
    <Modal isOpen isDismissable={false} onOpenChange={(open) => !open && onClose()}>
      <ModalHeader
        icon={RiKey2Line}
        title="Copy your new token"
        description="It shows only now. Store it somewhere safe, such as your CI's secrets."
        showCloseButton={false}
      />
      <ModalBody className="flex flex-col gap-1.5">
        <div className="flex items-end gap-2">
          <Input
            label={token.name}
            value={token.plain_text_token}
            isReadOnly
            inputDir="ltr"
            className="min-w-0 flex-1"
            fieldClassName="font-mono"
          />
          <IconButton
            icon={copied ? RiCheckLine : RiFileCopyLine}
            aria-label={copied ? "Copied" : "Copy token"}
            onClick={() => void copy()}
          />
        </div>
        <span aria-live="polite" className="text-body-2-regular text-text-tertiary">
          {copied ? "Copied to the clipboard." : "Send it as a bearer token in the Authorization header."}
        </span>
      </ModalBody>
      <ModalFooter>
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
    </Modal>
  );
}
