/*
 * Signing in with a password, or by mail: the person asks for a link and
 * then either follows it or types the six-digit code it carries here.
 */
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { HintText } from "@/components/base/input/hint-text";
import { Input } from "@/components/base/input/input";
import { InputOtp } from "@/components/base/input-otp/input-otp";
import { Notification } from "@/components/base/notification/notification";
import { LinkButton } from "@/features/navigation/links";
import { type AuthConfig, type SignInResult, sendMagicLink, signIn, signInWithCode } from "@/lib/api/auth";

import { AuthCard, AuthFooter, AuthHeader } from "./auth-frame";
import { field } from "./form-data";
import { fieldError } from "./form-errors";
import { ProviderButtons } from "./provider-buttons";

export interface LoginFormProps {
  config: AuthConfig;
  redirect: string;
  initialError?: string | null;
  onSignedIn: (result: SignInResult) => void;
}

export function LoginForm({ config, redirect, initialError, onSignedIn }: LoginFormProps) {
  const [mailedTo, setMailedTo] = useState<string | null>(null);

  if (mailedTo) {
    return <CodeStep email={mailedTo} onSignedIn={onSignedIn} onBack={() => setMailedTo(null)} />;
  }

  return (
    <PasswordStep
      config={config}
      redirect={redirect}
      initialError={initialError}
      onSignedIn={onSignedIn}
      onMailed={setMailedTo}
    />
  );
}

function PasswordStep({
  config,
  redirect,
  initialError,
  onSignedIn,
  onMailed,
}: LoginFormProps & { onMailed: (email: string) => void }) {
  const [email, setEmail] = useState("");
  const login = useMutation({
    mutationFn: (form: FormData) => signIn(field(form, "email"), field(form, "password")),
    onSuccess: onSignedIn,
  });
  const link = useMutation({ mutationFn: sendMagicLink, onSuccess: () => onMailed(email) });
  const error = login.error ?? link.error;
  const fields = ["email", "password"];
  const emailError = fieldError(error, "email", fields, "email");
  const passwordError = fieldError(error, "password", fields, "email");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    login.mutate(new FormData(event.currentTarget));
  };

  return (
    <AuthCard>
      <AuthHeader title="Log in to dimmi" />
      {initialError ? (
        <Notification status="error" title={initialError} dismissible={false} role="alert" className="mt-6 w-full" />
      ) : null}
      <ProviderButtons
        providers={config.providers}
        redirect={redirect}
        action="Continue with"
        emailLabel="or sign in with email"
      />
      <form
        onSubmit={submit}
        onChange={() => {
          login.reset();
          link.reset();
        }}
        className="mt-5 flex flex-col gap-4"
        aria-busy={login.isPending}
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
        <Input
          name="password"
          type="password"
          label="Password"
          placeholder="Enter your password"
          autoComplete="current-password"
          isRequired
          validationBehavior="aria"
          isInvalid={Boolean(passwordError)}
          errorMessage={passwordError}
        />
        <div className="flex justify-end">
          <LinkButton to="/forgot-password">Forgot password?</LinkButton>
        </div>
        <Button type="submit" className="w-full" disabled={login.isPending}>
          {login.isPending ? "Signing in…" : "Sign in"}
        </Button>
        {config.magic_link ? (
          <Button
            variant="secondary"
            className="w-full"
            disabled={link.isPending || !email}
            onClick={() => link.mutate(email)}
          >
            {link.isPending ? "Sending…" : "Email me a sign-in link"}
          </Button>
        ) : null}
      </form>
      {config.signup_mode === "open" ? (
        <AuthFooter>
          New here? <LinkButton to="/signup">Create an account</LinkButton>
        </AuthFooter>
      ) : null}
    </AuthCard>
  );
}

export function CodeStep({
  email,
  onSignedIn,
  onBack,
}: {
  email: string;
  onSignedIn: (result: SignInResult) => void;
  onBack: () => void;
}) {
  const [code, setCode] = useState("");
  const verify = useMutation({
    mutationFn: (typed: string) => signInWithCode(email, typed),
    onSuccess: onSignedIn,
    onError: () => setCode(""),
  });
  const check = (typed: string) => {
    if (!verify.isPending) {
      verify.mutate(typed);
    }
  };
  const resend = useMutation({ mutationFn: () => sendMagicLink(email) });
  const codeError = fieldError(verify.error ?? resend.error, "code", ["code"], "code");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    check(code);
  };

  return (
    <AuthCard>
      <AuthHeader
        title="Check your inbox"
        description={
          <>
            We sent a sign-in link to <span className="text-body-medium text-text-primary">{email}</span>. Open it, or
            type the code from the mail.
          </>
        }
      />
      <form onSubmit={submit} className="mt-6 flex flex-col gap-4" aria-busy={verify.isPending}>
        <div className="flex flex-col gap-1.5">
          <span className="text-body-medium text-text-secondary">Sign-in code</span>
          <InputOtp
            value={code}
            onChange={(typed) => {
              setCode(typed);
              verify.reset();
              resend.reset();
            }}
            onComplete={check}
            length={6}
            groupEvery={3}
            isInvalid={Boolean(codeError)}
            aria-label="Sign-in code"
            className="justify-start"
          />
          {codeError ? (
            <HintText isInvalid role="alert">
              {codeError}
            </HintText>
          ) : null}
        </div>
        <Button type="submit" className="w-full" disabled={verify.isPending || code.length < 6}>
          {verify.isPending ? "Signing in…" : "Sign in"}
        </Button>
      </form>
      <AuthFooter>
        Code not arriving? <LinkButton onClick={() => resend.mutate()}>Send a new one</LinkButton> or{" "}
        <LinkButton onClick={onBack}>use a password</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}
