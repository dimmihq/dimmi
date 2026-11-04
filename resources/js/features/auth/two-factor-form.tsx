/*
 * The second step of signing in to an account protected by a second
 * factor: a passkey on one of the person's devices, the six-digit code
 * from the authenticator app, or, when neither is at hand, one of the
 * recovery codes saved when two-factor was turned on. A challenge lasts
 * a few minutes and takes a few wrong answers; once it is spent the
 * person starts again from the first step.
 */
import { RiErrorWarningLine, RiKey2Line } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";
import { type FormEvent, Fragment, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { HintText } from "@/components/base/input/hint-text";
import { Input } from "@/components/base/input/input";
import { InputOtp } from "@/components/base/input-otp/input-otp";
import { ButtonLink, LinkButton } from "@/features/navigation/links";
import { type ChallengeMethod, type Me, signInWithPasskey, signInWithSecondFactor } from "@/lib/api/auth";
import { ApiError } from "@/lib/api/client";

import { AuthCard, AuthFooter, AuthHeader, AuthStatus } from "./auth-frame";
import { fieldError } from "./form-errors";
import { firstMethod, offers, passkeyOptions, type SecondFactor, secondFactors } from "./second-factor";

export interface TwoFactorFormProps {
  methods: ChallengeMethod[] | null;
  onSignedIn: (me: Me) => void;
  onExpired: () => void;
}

const headers: Record<SecondFactor, { title: string; description: string }> = {
  passkey: {
    title: "Use your passkey",
    description: "Confirm it is you with the passkey on this device, your phone or a security key.",
  },
  totp: { title: "Two-factor authentication", description: "Enter the six-digit code from your authenticator app." },
  recovery_code: {
    title: "Use a recovery code",
    description: "Enter one of the recovery codes you saved when you turned on two-factor. Each works once.",
  },
};

const switches: Record<SecondFactor, string> = {
  passkey: "Use a passkey",
  totp: "Use your authenticator app",
  recovery_code: "Use a recovery code instead",
};

/*
 * answerError returns what to show under the answer: the server's reason
 * for a wrong code or passkey, or why the passkey prompt ended.
 */
function answerError(error: Error | null): string | undefined {
  if (error === null) {
    return undefined;
  }

  if (!(error instanceof ApiError)) {
    return error.message;
  }

  return (
    fieldError(error, "code", ["code", "credential", "method"], "code") ?? error.fields.credential ?? error.message
  );
}

export function TwoFactorForm({ methods, onSignedIn, onExpired }: TwoFactorFormProps) {
  const [method, setMethod] = useState<SecondFactor>(() => firstMethod(methods));
  const [code, setCode] = useState("");
  const options = passkeyOptions(methods);
  const verify = useMutation({
    mutationFn: (typed: string) =>
      method === "passkey" && options ? signInWithPasskey(options) : signInWithSecondFactor(method, typed),
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
  const failure = answerError(verify.error);
  const others = secondFactors.filter((other) => other !== method && offers(methods, other));

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
      <AuthHeader title={headers[method].title} description={headers[method].description} />
      <form onSubmit={submit} className="mt-6 flex flex-col gap-4" aria-busy={verify.isPending}>
        <AnswerField
          method={method}
          code={code}
          failure={failure}
          onChange={(typed) => {
            setCode(typed);
            verify.reset();
          }}
          onComplete={check}
        />
        {method === "passkey" ? (
          <Button type="submit" className="w-full" leadingIcon={RiKey2Line} disabled={verify.isPending}>
            {verify.isPending ? "Waiting for your passkey…" : "Continue with passkey"}
          </Button>
        ) : (
          <Button type="submit" className="w-full" disabled={verify.isPending || !ready(method, code)}>
            {verify.isPending ? "Verifying…" : "Verify"}
          </Button>
        )}
      </form>
      <AuthFooter>
        {others.map((other) => (
          <Fragment key={other}>
            <LinkButton onClick={() => switchTo(other)}>{switches[other]}</LinkButton>
            {", "}
          </Fragment>
        ))}
        {others.length > 0 ? "or " : null}
        <LinkButton to="/login">start over</LinkButton>
      </AuthFooter>
    </AuthCard>
  );
}

/*
 * ready reports whether the typed code is complete enough to send.
 */
function ready(method: SecondFactor, code: string): boolean {
  return method === "totp" ? code.length >= 6 : code.trim() !== "";
}

/*
 * AnswerField is where the person answers: the six digits of the
 * authenticator, a recovery code, or nothing to type for a passkey, only
 * why the last prompt failed.
 */
function AnswerField({
  method,
  code,
  failure,
  onChange,
  onComplete,
}: {
  method: SecondFactor;
  code: string;
  failure: string | undefined;
  onChange: (typed: string) => void;
  onComplete: (typed: string) => void;
}) {
  if (method === "passkey") {
    return failure ? (
      <HintText isInvalid role="alert">
        {failure}
      </HintText>
    ) : null;
  }

  if (method === "recovery_code") {
    return (
      <Input
        name="code"
        label="Recovery code"
        placeholder="xxxxx-xxxxx"
        autoComplete="one-time-code"
        isRequired
        validationBehavior="aria"
        value={code}
        onChange={onChange}
        isInvalid={Boolean(failure)}
        errorMessage={failure}
      />
    );
  }

  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-body-medium text-text-secondary">Authentication code</span>
      <InputOtp
        value={code}
        onChange={onChange}
        onComplete={onComplete}
        length={6}
        groupEvery={3}
        isInvalid={Boolean(failure)}
        aria-label="Authentication code"
        className="justify-start"
      />
      {failure ? (
        <HintText isInvalid role="alert">
          {failure}
        </HintText>
      ) : null}
    </div>
  );
}
