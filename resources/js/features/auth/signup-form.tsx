/*
 * Creating an account with a name, an email and a password, or through
 * a provider. On a server that takes accounts by invitation only, the
 * page says so instead of offering a form that cannot work.
 */
import { useMutation } from "@tanstack/react-query";
import type { FormEvent } from "react";

import { Button } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Input } from "@/components/base/input/input";
import { Notification } from "@/components/base/notification/notification";
import { type AuthConfig, type Me, signUp } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";

import { AuthCard, AuthFooter, AuthHeader, FormError } from "./auth-frame";
import { field } from "./form-data";
import { ProviderButtons } from "./provider-buttons";

export interface SignupFormProps {
  config: AuthConfig;
  redirect: string;
  onSignedIn: (me: Me) => void;
}

export function SignupForm({ config, redirect, onSignedIn }: SignupFormProps) {
  const register = useMutation({
    mutationFn: (form: FormData) =>
      signUp({ name: field(form, "name"), email: field(form, "email"), password: field(form, "password") }),
    onSuccess: onSignedIn,
  });
  const error = register.error instanceof ApiError ? register.error : null;
  const fieldError = error && Object.keys(error.fields).length > 0;

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    register.mutate(new FormData(event.currentTarget));
  };

  if (config.signup_mode !== "open") {
    return (
      <AuthCard>
        <AuthHeader title="Create your account" />
        <div className="mt-6">
          <Notification
            status="information"
            title="Signing up is by invitation"
            description="Ask someone on your team to invite you. The invitation mail has a link to join."
            dismissible={false}
            className="w-full"
          />
        </div>
        <AuthFooter>
          Already have an account? <LinkButton href="/login">Sign in</LinkButton>
        </AuthFooter>
      </AuthCard>
    );
  }

  return (
    <AuthCard>
      <AuthHeader title="Create your account" description="Set up Dimmi for your team in a couple of minutes." />
      <ProviderButtons
        providers={config.providers}
        redirect={redirect}
        action="Sign up with"
        emailLabel="or sign up with email"
      />
      <form
        onSubmit={submit}
        onChange={() => register.reset()}
        className="mt-5 flex flex-col gap-4"
        aria-busy={register.isPending}
      >
        <FormError message={fieldError ? null : error?.message} />
        <Input
          name="name"
          label="Full name"
          placeholder="Ada Lovelace"
          autoComplete="name"
          isRequired
          validationBehavior="aria"
          isInvalid={Boolean(error?.fields.name)}
          errorMessage={error?.fields.name}
        />
        <Input
          name="email"
          type="email"
          label="Email"
          placeholder="you@company.com"
          autoComplete="email"
          hint="We use this to contact you, and never share it."
          isRequired
          validationBehavior="aria"
          isInvalid={Boolean(error?.fields.email)}
          errorMessage={error?.fields.email}
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
          isInvalid={Boolean(error?.fields.password)}
          errorMessage={error?.fields.password}
        />
        <Button type="submit" className="w-full" disabled={register.isPending}>
          {register.isPending ? "Creating account…" : "Create account"}
        </Button>
        <p className="text-caption-1-regular text-text-tertiary">
          By creating an account you agree to our Terms of Service and Privacy Policy.
        </p>
      </form>
      <AuthFooter>
        Already have an account? <LinkButton href="/login">Sign in</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}
