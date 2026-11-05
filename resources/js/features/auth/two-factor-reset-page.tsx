/*
 * The page a teammate opens from the link an admin mailed them, when they
 * lost their second factor. Nothing happens until they ask: then the
 * server turns two-factor authentication off, after asking for their
 * password when the account has one, and signs them out everywhere. They
 * sign in again and set up a new authenticator.
 */
import { RiCheckboxCircleLine, RiErrorWarningLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { ButtonLink } from "@/features/navigation/links";
import { completeTwoFactorReset } from "@/lib/api/account";
import { ApiError } from "@/lib/api/client";

import { AuthCard, AuthHeader, AuthStatus } from "./auth-frame";
import { fieldError } from "./form-errors";
import { BrokenLink } from "./password-reset-forms";

/*
 * passwordRequired is the server's answer to an account that has a
 * password and was sent none.
 */
const passwordRequired = "password_required";

/*
 * brokenLink reports whether the server refused the link itself, rather
 * than the password.
 */
function brokenLink(error: unknown): error is ApiError {
  return error instanceof ApiError && Boolean(error.fields.token);
}

export function TwoFactorResetPage({ id, token }: { id: string; token: string }) {
  const [password, setPassword] = useState("");
  const [asksPassword, setAsksPassword] = useState(false);
  const reset = useMutation({
    mutationFn: () => completeTwoFactorReset(id, token, asksPassword ? password : undefined),
    onError: (error) => {
      if (error instanceof ApiError && error.code === passwordRequired) {
        setAsksPassword(true);
      }
    },
  });
  const asked = reset.error instanceof ApiError && reset.error.code === passwordRequired;
  const passwordError = asked ? undefined : fieldError(reset.error, "password", ["password"], "password");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    reset.mutate();
  };

  if (!(id && token)) {
    return <BrokenLink />;
  }

  if (reset.isSuccess) {
    return (
      <AuthStatus
        icon={RiCheckboxCircleLine}
        title="Two-factor authentication is off"
        description="You were signed out everywhere. Sign in again and set up a new authenticator."
      >
        <ButtonLink to="/login" className="mt-6 w-full">
          Sign in
        </ButtonLink>
      </AuthStatus>
    );
  }

  if (brokenLink(reset.error)) {
    return (
      <AuthStatus
        icon={RiErrorWarningLine}
        tone="error"
        title="This link does not work"
        description={reset.error.message}
      >
        <ButtonLink to="/login" className="mt-6 w-full">
          Back to sign in
        </ButtonLink>
      </AuthStatus>
    );
  }

  return (
    <AuthCard>
      <AuthHeader
        title="Reset two-factor authentication"
        description="Your admin sent this link because you cannot use your second factor. Turning it off signs you out everywhere."
      />
      <form onSubmit={submit} className="mt-6 flex flex-col gap-4" aria-busy={reset.isPending}>
        {asksPassword ? (
          <Input
            name="password"
            type="password"
            label="Password"
            hint="Enter your password to prove it is you."
            autoComplete="current-password"
            autoFocus
            isRequired
            validationBehavior="aria"
            value={password}
            onChange={(typed) => {
              setPassword(typed);
              reset.reset();
            }}
            isInvalid={Boolean(passwordError)}
            errorMessage={passwordError}
          />
        ) : null}
        {!asksPassword && passwordError ? (
          <p role="alert" className="text-body-2-regular text-text-error-primary">
            {passwordError}
          </p>
        ) : null}
        <Button type="submit" className="w-full" disabled={reset.isPending || (asksPassword && password === "")}>
          {reset.isPending ? "Turning off…" : "Turn off two-factor authentication"}
        </Button>
      </form>
    </AuthCard>
  );
}
