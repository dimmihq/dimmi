/*
 * Creating an account with a name, an email and a password, or through
 * a provider. On a server that takes accounts by invitation only, the
 * page says so instead of offering a form that cannot work.
 */
import { RiMailLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";
import type { FormEvent } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { ButtonLink, LinkButton } from "@/features/navigation/links";
import { type AuthConfig, type Me, signUp } from "@/lib/api/auth";

import { AuthCard, AuthFooter, AuthHeader, AuthStatus } from "./auth-frame";
import { field } from "./form-data";
import { fieldError } from "./form-errors";
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
  const fields = ["name", "email", "password"];
  const errorFor = (name: string) => fieldError(register.error, name, fields, "email");

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    register.mutate(new FormData(event.currentTarget));
  };

  if (config.signup_mode !== "open") {
    return (
      <AuthStatus
        icon={RiMailLine}
        title="Signing up is by invitation"
        description="Ask someone on your team to invite you. The invitation mail has a link to join."
      >
        <ButtonLink to="/login" variant="secondary" className="mt-6 w-full">
          Back to sign in
        </ButtonLink>
      </AuthStatus>
    );
  }

  return (
    <AuthCard>
      <AuthHeader title="Create your account" />
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
          placeholder="you@company.com"
          autoComplete="email"
          hint="We use this to contact you, and never share it."
          isRequired
          validationBehavior="aria"
          isInvalid={Boolean(errorFor("email"))}
          errorMessage={errorFor("email")}
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
        <Button type="submit" className="w-full" loading={register.isPending}>
          Create account
        </Button>
        <p className="text-caption-1-regular text-text-tertiary">
          By creating an account you agree to our Terms of Service and Privacy Policy.
        </p>
      </form>
      <AuthFooter>
        Already have an account? <LinkButton to="/login">Sign in</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}
