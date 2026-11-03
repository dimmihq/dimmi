/*
 * The second step of signing in to an account protected by a second
 * factor: the six-digit code from the authenticator app, or, when the
 * phone is not at hand, one of the recovery codes saved when two-factor
 * was turned on. A challenge lasts a few minutes and takes a few wrong
 * codes; once it is spent the person starts again from the first step.
 */
import { RiErrorWarningLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { HintText } from "@/components/base/input/hint-text";
import { Input } from "@/components/base/input/input";
import { InputOtp } from "@/components/base/input-otp/input-otp";
import { ButtonLink, LinkButton } from "@/features/navigation/links";
import { type ChallengeMethod, type Me, signInWithSecondFactor } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";

import { AuthCard, AuthFooter, AuthHeader, AuthStatus } from "./auth-frame";
import { fieldError } from "./form-errors";
import { firstMethod, offers, type SecondFactor } from "./second-factor";

export interface TwoFactorFormProps {
  methods: ChallengeMethod[] | null;
  onSignedIn: (me: Me) => void;
  onExpired: () => void;
}

export function TwoFactorForm({ methods, onSignedIn, onExpired }: TwoFactorFormProps) {
  const [method, setMethod] = useState<SecondFactor>(() => firstMethod(methods));
  const [code, setCode] = useState("");
  const verify = useMutation({
    mutationFn: (typed: string) => signInWithSecondFactor(method, typed),
    onSuccess: onSignedIn,
    onError: (error) => {
      if (error instanceof ApiError && error.code === "two_factor_challenge_expired") {
        onExpired();

        return;
      }

      setCode("");
    },
  });
  const check = (typed: string) => {
    if (!verify.isPending) {
      verify.mutate(typed);
    }
  };
  const codeError = fieldError(verify.error, "code", ["code", "method"], "code") ?? verify.error?.message;
  const other: SecondFactor = method === "totp" ? "recovery_code" : "totp";

  if (methods !== null && methods.length === 0) {
    return (
      <AuthStatus
        icon={RiErrorWarningLine}
        tone="error"
        title="No way to finish signing in"
        description="Two-factor authentication is on for this account, but none of its factors can be used on this server. Ask your administrator, who can turn it off with dimmi user:disable-2fa."
      >
        <ButtonLink to="/login" className="mt-6 w-full">
          Back to sign in
        </ButtonLink>
      </AuthStatus>
    );
  }

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    check(code);
  };
  const switchTo = (next: SecondFactor) => {
    setMethod(next);
    setCode("");
    verify.reset();
  };

  return (
    <AuthCard>
      {method === "totp" ? (
        <AuthHeader
          title="Two-factor authentication"
          description="Enter the six-digit code from your authenticator app."
        />
      ) : (
        <AuthHeader
          title="Use a recovery code"
          description="Enter one of the recovery codes you saved when you turned on two-factor. Each works once."
        />
      )}
      <form onSubmit={submit} className="mt-6 flex flex-col gap-4" aria-busy={verify.isPending}>
        {method === "totp" ? (
          <div className="flex flex-col gap-1.5">
            <span className="text-body-medium text-text-secondary">Authentication code</span>
            <InputOtp
              value={code}
              onChange={(typed) => {
                setCode(typed);
                verify.reset();
              }}
              onComplete={check}
              length={6}
              groupEvery={3}
              isInvalid={Boolean(codeError)}
              aria-label="Authentication code"
              className="justify-start"
            />
            {codeError ? (
              <HintText isInvalid role="alert">
                {codeError}
              </HintText>
            ) : null}
          </div>
        ) : (
          <Input
            name="code"
            label="Recovery code"
            placeholder="xxxxx-xxxxx"
            autoComplete="one-time-code"
            isRequired
            validationBehavior="aria"
            value={code}
            onChange={(typed) => {
              setCode(typed);
              verify.reset();
            }}
            isInvalid={Boolean(codeError)}
            errorMessage={codeError}
          />
        )}
        <Button
          type="submit"
          className="w-full"
          disabled={verify.isPending || (method === "totp" ? code.length < 6 : code.trim() === "")}
        >
          {verify.isPending ? "Verifying…" : "Verify"}
        </Button>
      </form>
      <AuthFooter>
        {offers(methods, other) ? (
          <>
            <LinkButton onClick={() => switchTo(other)}>
              {other === "recovery_code" ? "Use a recovery code instead" : "Use your authenticator app"}
            </LinkButton>{" "}
            or{" "}
          </>
        ) : null}
        <LinkButton to="/login">start over</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}
