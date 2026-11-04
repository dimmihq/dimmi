/*
 * The page an invitation mail opens. It shows which workspace asked, who
 * asked and as what, then takes the shortest way in: accepting at once
 * for someone signed in, signing in first for an address that has an
 * account, or creating the account right here, already proven and
 * already a member. An invitation that can no longer be used says why
 * and who can send a new one.
 */
import { RiCheckboxCircleLine, RiErrorWarningLine, RiLinkUnlinkM, RiUserForbidLine } from "@remixicon/react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { FormEvent } from "react";

import { Button } from "@/components/base/buttons/button";
import { HintText } from "@/components/base/input/hint-text";
import { Input } from "@/components/base/input/input";
import { Notification } from "@/components/base/notification/notification";
import { ButtonLink } from "@/features/navigation/links";
import { roleName } from "@/features/settings/invitation-text";
import { rememberWorkspace } from "@/features/shell/workspace";
import { type AuthConfig, type Me, meQuery, signOut } from "@/lib/api/auth";
import {
  acceptInvitation,
  type InvitationPreview,
  invitationPreviewQuery,
  signUpWithInvitation,
} from "@/lib/api/invitations";

import { AuthCard, AuthHeader, AuthStatus } from "./auth-frame";
import { field } from "./form-data";
import { fieldError } from "./form-errors";
import { alreadyJoined, type Ended, emailMismatch, endedBy, endedText, forgetInvitationToken } from "./invitation-link";
import { Working } from "./link-pages";
import { oauthError } from "./oauth-errors";
import { ProviderButtons } from "./provider-buttons";

/*
 * enter opens the app in the workspace just joined. The page loads
 * afresh, so every part of the app reads the new membership.
 */
function enter(workspace: string | null) {
  forgetInvitationToken();
  if (workspace) {
    rememberWorkspace(workspace);
  }
  window.location.assign("/d");
}

function mismatchText(preview: InvitationPreview): string {
  return `Use the address the invitation was sent to (${preview.email_masked}).`;
}

/*
 * returnError says why a provider sign-in came back without letting the
 * person in, or null.
 */
function returnError(code: string | undefined, preview: InvitationPreview): string | null {
  if (code === "email_mismatch") {
    return mismatchText(preview);
  }

  return oauthError(code);
}

function Invited({ preview }: { preview: InvitationPreview }) {
  const inviter = preview.inviter?.name ?? "Someone";

  return (
    <AuthHeader
      title={`Join ${preview.workspace.name}`}
      description={`${inviter} invited you to join as ${roleName(preview.role)}.`}
    />
  );
}

function Problem({ text }: { text: string | null }) {
  if (!text) {
    return null;
  }

  return <Notification status="error" title={text} dismissible={false} role="alert" className="mt-6 w-full" />;
}

function EndedCard({ ended, preview, signedIn }: { ended: Ended; preview?: InvitationPreview; signedIn: boolean }) {
  const text = endedText(ended, preview?.inviter?.name ?? null);
  const accepted = ended === "accepted";

  return (
    <AuthStatus
      icon={accepted ? RiCheckboxCircleLine : RiLinkUnlinkM}
      tone={accepted ? "neutral" : "error"}
      title={text.title}
      description={text.description}
    >
      {signedIn ? (
        <ButtonLink to="/d" className="mt-6 w-full">
          Open dimmi
        </ButtonLink>
      ) : (
        <ButtonLink to="/login" className="mt-6 w-full">
          Go to sign in
        </ButtonLink>
      )}
    </AuthStatus>
  );
}

/*
 * Unreadable is an invitation the server would not show: one it does not
 * know, or a failure on the way, in its own words.
 */
function Unreadable({ error, signedIn }: { error: Error; signedIn: boolean }) {
  const ended = endedBy(error);
  if (ended) {
    return <EndedCard ended={ended} signedIn={signedIn} />;
  }

  return (
    <AuthStatus
      icon={RiErrorWarningLine}
      tone="error"
      title="This invitation could not be opened"
      description={fieldError(error, "token", [], "token")}
    >
      <Button className="mt-6 w-full" onClick={() => window.location.reload()}>
        Try again
      </Button>
    </AuthStatus>
  );
}

/*
 * OtherAccount tells someone signed in at another address that the
 * invitation is not theirs to accept from this account.
 */
function OtherAccount({ me, preview }: { me: Me; preview: InvitationPreview }) {
  const out = useMutation({ mutationFn: signOut, onSuccess: () => window.location.reload() });

  return (
    <AuthStatus
      icon={RiUserForbidLine}
      tone="error"
      title="This invitation is for another address"
      description={`You are signed in as ${me.user.email}, but the invitation was sent to ${preview.email_masked}. Sign out, then sign in with that address.`}
    >
      <Button className="mt-6 w-full" disabled={out.isPending} onClick={() => out.mutate()}>
        {out.isPending ? "Signing out…" : "Sign out"}
      </Button>
    </AuthStatus>
  );
}

function AcceptStep({ token, me, preview }: { token: string; me: Me; preview: InvitationPreview }) {
  const accept = useMutation({
    mutationFn: () => acceptInvitation(token),
    onSuccess: (membership) => enter(membership.workspace.id),
    onError: (error) => (alreadyJoined(error) ? enter(null) : undefined),
  });
  const ended = endedBy(accept.error);

  if (ended) {
    return <EndedCard ended={ended} preview={preview} signedIn />;
  }

  if (emailMismatch(accept.error)) {
    return <OtherAccount me={me} preview={preview} />;
  }

  const failure = alreadyJoined(accept.error) ? undefined : fieldError(accept.error, "token", [], "token");

  return (
    <AuthCard>
      <Invited preview={preview} />
      <p className="mt-6 text-body-regular text-text-secondary">
        Signed in as <span className="text-body-medium text-text-primary">{me.user.email}</span>
      </p>
      <Button className="mt-4 w-full" disabled={accept.isPending || accept.isSuccess} onClick={() => accept.mutate()}>
        {accept.isPending ? "Joining…" : "Accept invitation"}
      </Button>
      {failure ? (
        <HintText isInvalid role="alert" className="mt-2">
          {failure}
        </HintText>
      ) : null}
    </AuthCard>
  );
}

function SignInStep({
  token,
  preview,
  config,
  error,
}: {
  token: string;
  preview: InvitationPreview;
  config: AuthConfig;
  error?: string;
}) {
  const navigate = useNavigate();

  return (
    <AuthCard>
      <Invited preview={preview} />
      <Problem text={returnError(error, preview)} />
      <ProviderButtons
        providers={config.providers}
        redirect="/invitation"
        invitation={token}
        action="Continue with"
        emailLabel="or with your password"
      />
      <Button className="mt-6 w-full" onClick={() => navigate({ to: "/login", search: { redirect: "/invitation" } })}>
        Sign in to accept
      </Button>
    </AuthCard>
  );
}

/*
 * SignupStep creates the invitee's account. The server proves the
 * address by the invitation itself and makes the account a member of the
 * workspace, so the person lands in it straight away.
 */
function SignupStep({
  token,
  preview,
  config,
  error,
}: {
  token: string;
  preview: InvitationPreview;
  config: AuthConfig;
  error?: string;
}) {
  const register = useMutation({
    mutationFn: (form: FormData) =>
      signUpWithInvitation({
        name: field(form, "name"),
        email: field(form, "email"),
        password: field(form, "password"),
        invitation_token: token,
      }),
    onSuccess: (me) => {
      const joined = me.workspaces.find((workspace) => workspace.id === preview.workspace.id);
      enter(joined?.id ?? me.workspaces[0]?.id ?? null);
    },
  });
  const fields = ["name", "email", "password"];
  const errorFor = (name: string) => fieldError(register.error, name, fields, "email");
  const emailError = emailMismatch(register.error) ? mismatchText(preview) : errorFor("email");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    register.mutate(new FormData(event.currentTarget));
  };

  return (
    <AuthCard>
      <Invited preview={preview} />
      <Problem text={returnError(error, preview)} />
      <ProviderButtons
        providers={config.providers}
        redirect="/invitation"
        invitation={token}
        action="Sign up with"
        emailLabel="or sign up with email"
      />
      <form
        onSubmit={submit}
        onChange={() => register.reset()}
        className="mt-5 flex flex-col gap-4"
        aria-busy={register.isPending}
      >
        <Input
          name="name"
          label="Full name"
          placeholder="Ada Lovelace"
          autoComplete="name"
          isRequired
          validationBehavior="aria"
          isInvalid={Boolean(errorFor("name"))}
          errorMessage={errorFor("name")}
        />
        <Input
          name="email"
          type="email"
          label="Email"
          placeholder={preview.email_masked}
          autoComplete="email"
          hint={`The address the invitation was sent to, ${preview.email_masked}.`}
          isRequired
          validationBehavior="aria"
          isInvalid={Boolean(emailError)}
          errorMessage={emailError}
        />
        <Input
          name="password"
          type="password"
          label="Password"
          placeholder="Choose a password"
          autoComplete="new-password"
          hint={`At least ${config.password_min_length} characters.`}
          isRequired
          validationBehavior="aria"
          isInvalid={Boolean(errorFor("password"))}
          errorMessage={errorFor("password")}
        />
        <Button type="submit" className="w-full" disabled={register.isPending || register.isSuccess}>
          {register.isPending ? "Creating account…" : "Create account"}
        </Button>
        <p className="text-caption-1-regular text-text-tertiary">
          By creating an account you agree to our Terms of Service and Privacy Policy.
        </p>
      </form>
    </AuthCard>
  );
}

export interface InvitationPageProps {
  /** The invitation's token, from the link or kept from an earlier visit; "" when there is none. */
  token: string;
  config: AuthConfig;
  /** The code a provider sign-in came back with, when it failed. */
  error?: string;
}

/*
 * InvitationPage reads the invitation and offers the way in that fits
 * the person at the keyboard.
 */
export function InvitationPage({ token, config, error }: InvitationPageProps) {
  const preview = useQuery({ ...invitationPreviewQuery(token), enabled: token !== "" });
  const me = useQuery(meQuery);
  const signedIn = Boolean(me.data);

  if (token === "") {
    return <EndedCard ended="invalid" signedIn={signedIn} />;
  }

  if (preview.isPending || me.isPending) {
    return <Working title="Opening your invitation…" />;
  }

  if (preview.isError) {
    return <Unreadable error={preview.error} signedIn={signedIn} />;
  }

  const invitation = preview.data;
  if (invitation.status !== "pending") {
    return <EndedCard ended={invitation.status} preview={invitation} signedIn={signedIn} />;
  }

  if (me.data) {
    return <AcceptStep token={token} me={me.data} preview={invitation} />;
  }

  const Step = invitation.has_account ? SignInStep : SignupStep;

  return <Step token={token} preview={invitation} config={config} error={error} />;
}
