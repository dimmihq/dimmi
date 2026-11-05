/*
 * The ways the person signs in besides their password: one row per
 * identity provider the server offers, linked or not. Linking goes
 * through the provider in this browser and needs a recent proof of who is
 * at the keyboard, asked for first when it is too old; the provider sends
 * the browser back here, which says how it went. Unlinking the last way
 * in is refused, with the way out: set a password first.
 */
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { Notification, NotificationViewport } from "@/components/base/notification/notification";
import { authConfigQuery, type Provider } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";
import { type Identity, identitiesQuery, linkAddress, recentlyConfirmed, unlinkIdentity } from "@/lib/api/identities";

import { ConfirmPasswordDialog } from "./confirm-password-dialog";
import { Action } from "./controls";
import { Empty, Row, RowsSkeleton, Section } from "./layout";
import { useRecentAuth } from "./recent-auth";

/*
 * linkFailures says in words why a provider could not be linked, by the
 * code the server sent the browser back with.
 */
const linkFailures: Record<string, string> = {
  identity_taken: "That account is already linked to another dimmi user.",
  provider_already_linked: "You already linked an account of this provider. Unlink it first.",
  oauth_denied: "The provider did not let dimmi link the account.",
};

/*
 * Outcome is what the provider's round trip ended with, read once from
 * the address the browser came back to.
 */
interface Outcome {
  status: "success" | "error";
  title: string;
}

/*
 * readOutcome reads the outcome of a link from the address.
 */
function readOutcome(providers: Provider[]): Outcome | null {
  const query = new URLSearchParams(window.location.search);
  const linked = query.get("linked");
  const failed = query.get("error");
  if (!(linked || failed)) {
    return null;
  }

  if (linked) {
    const name = providers.find((provider) => provider.key === linked)?.name ?? linked;
    return { status: "success", title: `${name} linked. You can sign in with it now.` };
  }

  return { status: "error", title: linkFailures[failed ?? ""] ?? "The account could not be linked. Try again." };
}

/*
 * detailOf is how a linked account is named: its address, or its user
 * name when the provider shares no address.
 */
function detailOf(identity: Identity | undefined): string {
  if (!identity) {
    return "Not linked";
  }

  return identity.email ?? identity.username ?? "Linked";
}

function ProviderRow({
  provider,
  identity,
  onLink,
  onUnlink,
}: {
  provider: Provider;
  identity: Identity | undefined;
  onLink: (provider: Provider) => void;
  onUnlink: (identity: Identity) => void;
}) {
  return (
    <Row label={provider.name} description={detailOf(identity)}>
      {identity ? (
        <Action onPress={() => onUnlink(identity)}>Unlink</Action>
      ) : (
        <Action onPress={() => onLink(provider)}>Link</Action>
      )}
    </Row>
  );
}

/*
 * useLink sends the browser to link the provider, asking for the password
 * first when the last proof of who is at the keyboard is too old.
 */
function useLink() {
  const [waiting, setWaiting] = useState<Provider | null>(null);

  const go = (provider: Provider) => window.location.assign(linkAddress(provider));
  const link = async (provider: Provider) => {
    if (await recentlyConfirmed()) {
      go(provider);
      return;
    }

    setWaiting(provider);
  };
  const confirmed = () => {
    if (waiting) {
      go(waiting);
    }
    setWaiting(null);
  };

  return { waiting, link, confirmed, cancel: () => setWaiting(null) };
}

/*
 * unlinkFailure says why an unlink was refused, with the way out when it
 * was the last way in.
 */
function unlinkFailure(error: unknown): string | null {
  if (!error) {
    return null;
  }

  if (error instanceof ApiError && error.code === "last_login_method") {
    return "This is your only way to sign in. Set a password first.";
  }

  return error instanceof Error ? error.message : "Something went wrong.";
}

export function SignInMethodsSection({ hasPassword }: { hasPassword: boolean }) {
  const queryClient = useQueryClient();
  const { data: config } = useSuspenseQuery(authConfigQuery);
  const { data: identities = [], isPending } = useQuery(identitiesQuery);
  const providers = config.providers;
  const [outcome, setOutcome] = useState<Outcome | null>(() => readOutcome(providers));
  useEffect(() => {
    if (window.location.search !== "") {
      window.history.replaceState(window.history.state, "", window.location.pathname);
    }
  }, []);
  const { waiting, link, confirmed, cancel } = useLink();
  const { run, prompt } = useRecentAuth(hasPassword);
  const unlink = useMutation({
    mutationFn: (identity: Identity) => run(() => unlinkIdentity(identity.id)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["identities"] }),
  });
  const failure = unlinkFailure(unlink.error);

  return (
    <Section title="Sign-in methods" description="Accounts at other services you can sign in with.">
      {failure ? (
        <div className="py-2.5 pe-2.5">
          <Notification status="error" title={failure} dismissible={false} role="alert" className="w-full" />
        </div>
      ) : null}
      <Row
        label="Password"
        description={hasPassword ? "You can sign in with your password." : "Not set. Set one under Password."}
      />
      {providers.length === 0 ? <Empty>This server offers no other way to sign in.</Empty> : null}
      {isPending && providers.length > 0 ? <RowsSkeleton rows={providers.length} /> : null}
      {(isPending ? [] : providers).map((provider) => (
        <ProviderRow
          key={provider.key}
          provider={provider}
          identity={identities.find((identity) => identity.provider === provider.key)}
          onLink={(chosen) => void link(chosen)}
          onUnlink={(identity) => unlink.mutate(identity)}
        />
      ))}
      <ConfirmPasswordDialog
        isOpen={waiting !== null}
        hasPassword={hasPassword}
        onCancel={cancel}
        onConfirmed={confirmed}
      />
      {prompt}
      <NotificationViewport>
        {outcome ? (
          <Notification
            key="link-outcome"
            status={outcome.status}
            title={outcome.title}
            role="status"
            autoDismissDuration={6000}
            onDismiss={() => setOutcome(null)}
          />
        ) : null}
      </NotificationViewport>
    </Section>
  );
}
