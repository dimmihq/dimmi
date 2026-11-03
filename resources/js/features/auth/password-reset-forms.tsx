/*
 * Getting back into an account: asking for a reset link, which always
 * answers the same so it never tells whether an address has an account,
 * and choosing a new password from the link.
 */
import { RiLinkUnlinkM, RiMailSendLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { HintText } from "@/components/base/input/hint-text";
import { Input } from "@/components/base/input/input";
import { ButtonLink, LinkButton } from "@/features/navigation/links";
import { requestPasswordReset, resetPassword, type SignInResult } from "@/lib/api/auth";

import { AuthCard, AuthFooter, AuthHeader, AuthStatus } from "./auth-frame";
import { field } from "./form-data";
import { fieldError } from "./form-errors";

export function ForgotPasswordForm() {
  const [email, setEmail] = useState("");
  const send = useMutation({ mutationFn: requestPasswordReset });
  const emailError = fieldError(send.error, "email", ["email"], "email");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    send.mutate(email);
  };

  if (send.isSuccess) {
    return <ResetLinkSent email={email} onResend={() => send.mutateAsync(email)} />;
  }

  return (
    <AuthCard>
      <AuthHeader title="Reset your password" description="We will mail you a link to choose a new one." />
      <form
        onSubmit={submit}
        onChange={() => send.reset()}
        className="mt-6 flex flex-col gap-4"
        aria-busy={send.isPending}
      >
        <Input
          name="email"
          type="email"
          label="Email"
          placeholder="you@company.com"
          autoComplete="email"
          isRequired
          validationBehavior="aria"
          value={email}
          onChange={setEmail}
          isInvalid={Boolean(emailError)}
          errorMessage={emailError}
        />
        <Button type="submit" className="w-full" disabled={send.isPending}>
          {send.isPending ? "Sending…" : "Send reset link"}
        </Button>
      </form>
      <AuthFooter>
        Remembered it? <LinkButton to="/login">Sign in</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}

/*
 * ResetLinkSent confirms the request. It never says whether the address
 * has an account, so it reads the same for every address.
 */
function ResetLinkSent({ email, onResend }: { email: string; onResend: () => Promise<unknown> }) {
  const resend = useMutation({ mutationFn: onResend });
  const resendError = fieldError(resend.error, "email", ["email"], "email");

  return (
    <AuthStatus
      icon={RiMailSendLine}
      title="Check your inbox"
      description={
        <>
          If <span className="text-body-medium text-text-primary">{email}</span> has an account, a link to choose a new
          password is on its way. It works for 60 minutes.
        </>
      }
    >
      <ButtonLink to="/login" variant="secondary" className="mt-6 w-full">
        Back to sign in
      </ButtonLink>
      {resendError ? (
        <HintText isInvalid role="alert" className="mt-3">
          {resendError}
        </HintText>
      ) : null}
      <AuthFooter>
        {resend.isSuccess ? (
          "We sent it again."
        ) : (
          <>
            Didn't get it?{" "}
            <LinkButton onClick={() => resend.mutate()} disabled={resend.isPending}>
              Send again
            </LinkButton>
          </>
        )}
      </AuthFooter>
    </AuthStatus>
  );
}

export interface ResetPasswordFormProps {
  id: string;
  token: string;
  minLength: number;
  onSignedIn: (result: SignInResult) => void;
}

export function ResetPasswordForm({ id, token, minLength, onSignedIn }: ResetPasswordFormProps) {
  const reset = useMutation({
    mutationFn: (form: FormData) => resetPassword(id, token, field(form, "password")),
    onSuccess: onSignedIn,
  });
  const passwordError = fieldError(reset.error, "password", ["password"], "password");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    reset.mutate(new FormData(event.currentTarget));
  };

  if (!(id && token)) {
    return <BrokenLink />;
  }

  return (
    <AuthCard>
      <AuthHeader title="Choose a new password" description="Every other session is signed out once you do." />
      <form
        onSubmit={submit}
        onChange={() => reset.reset()}
        className="mt-6 flex flex-col gap-4"
        aria-busy={reset.isPending}
      >
        <Input
          name="password"
          type="password"
          label="New password"
          placeholder="Choose a password"
          autoComplete="new-password"
          hint={`At least ${minLength} characters.`}
          isRequired
          validationBehavior="aria"
          isInvalid={Boolean(passwordError)}
          errorMessage={passwordError}
        />
        <Button type="submit" className="w-full" disabled={reset.isPending}>
          {reset.isPending ? "Saving…" : "Save password and sign in"}
        </Button>
      </form>
      <AuthFooter>
        Link expired? <LinkButton to="/forgot-password">Send a new one</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}

export function BrokenLink() {
  return (
    <AuthStatus
      icon={RiLinkUnlinkM}
      tone="error"
      title="This link is incomplete"
      description="Open the link from the mail again, or copy all of it into the address bar."
    >
      <AuthFooter>
        <LinkButton to="/login">Back to sign in</LinkButton>
      </AuthFooter>
    </AuthStatus>
  );
}
