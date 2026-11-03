/*
 * Getting back into an account: asking for a reset link, which always
 * answers the same so it never tells whether an address has an account,
 * and choosing a new password from the link.
 */
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Input } from "@/components/base/input/input";
import { Notification } from "@/components/base/notification/notification";
import { type Me, requestPasswordReset, resetPassword } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";

import { AuthCard, AuthFooter, AuthHeader, FormError } from "./auth-frame";
import { field } from "./form-data";

function failure(error: unknown): ApiError | null {
  return error instanceof ApiError ? error : null;
}

export function ForgotPasswordForm() {
  const [email, setEmail] = useState("");
  const send = useMutation({ mutationFn: requestPasswordReset });
  const error = failure(send.error);

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
          <FormError message={error?.fields.email ? null : error?.message} />
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
            isInvalid={Boolean(error?.fields.email)}
            errorMessage={error?.fields.email}
          />
          <Button type="submit" className="w-full" disabled={send.isPending}>
            {send.isPending ? "Sending…" : "Send reset link"}
          </Button>
        </form>
      )}
      <AuthFooter>
        Remembered it? <LinkButton href="/login">Sign in</LinkButton>
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
  const error = failure(reset.error);

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
        <FormError message={error?.fields.password ? null : error?.message} />
        <Input
          name="password"
          type="password"
          label="New password"
          placeholder="Choose a password"
          autoComplete="new-password"
          hint={`At least ${minLength} characters.`}
          isRequired
          validationBehavior="aria"
          isInvalid={Boolean(error?.fields.password)}
          errorMessage={error?.fields.password}
        />
        <Button type="submit" className="w-full" disabled={reset.isPending}>
          {reset.isPending ? "Saving…" : "Save password and sign in"}
        </Button>
      </form>
      <AuthFooter>
        Link expired? <LinkButton href="/forgot-password">Send a new one</LinkButton>
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
        <LinkButton href="/login">Back to sign in</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}
