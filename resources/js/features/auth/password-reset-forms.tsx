/*
 * Getting back into an account: asking for a reset link, which always
 * answers the same so it never tells whether an address has an account,
 * and choosing a new password from the link.
 */
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Notification } from "@/components/base/notification/notification";
import { LinkButton } from "@/features/navigation/links";
import { type Me, requestPasswordReset, resetPassword } from "@/lib/api/auth";

import { AuthCard, AuthFooter, AuthHeader } from "./auth-frame";
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

  return (
    <AuthCard>
      <AuthHeader title="Reset your password" description="We will mail you a link to choose a new one." />
      {send.isSuccess ? (
        <div className="mt-6">
          <Notification
            status="success"
            title="Check your inbox"
            description={send.data.message}
            dismissible={false}
            className="w-full"
          />
        </div>
      ) : (
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
      )}
      <AuthFooter>
        Remembered it? <LinkButton to="/login">Sign in</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}

export interface ResetPasswordFormProps {
  id: string;
  token: string;
  minLength: number;
  onSignedIn: (me: Me) => void;
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
    <AuthCard>
      <AuthHeader
        title="This link is incomplete"
        description="Open the link from the mail again, or copy all of it into the address bar."
      />
      <AuthFooter>
        <LinkButton to="/login">Back to sign in</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}
