/*
 * Pages a mailed link opens: signing in with a magic link, and proving
 * an email address. Only this page's script spends the link's secret, by
 * posting it once, so a mail scanner that opens the address spends
 * nothing.
 */
import { RiCheckboxCircleLine, RiErrorWarningLine } from "@remixicon/react";
import { useMutation } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { Spinner } from "@/components/base/spinner/spinner";
import { ButtonLink } from "@/features/navigation/links";
import { type Me, signInWithLink, verifyEmail } from "@/lib/api/auth";

import { AuthCard, AuthHeader, AuthStatus } from "./auth-frame";
import { BrokenLink } from "./password-reset-forms";

/*
 * useOnce runs the action once when the page opens, even when React
 * mounts the page twice in development.
 */
function useOnce(action: () => void) {
  const ran = useRef(false);

  useEffect(() => {
    if (!ran.current) {
      ran.current = true;
      action();
    }
  }, [action]);
}

function Working({ title }: { title: string }) {
  return (
    <AuthCard>
      <div className="flex items-center gap-3">
        <Spinner size="sm" aria-hidden />
        <AuthHeader title={title} />
      </div>
    </AuthCard>
  );
}

export function MagicLinkPage({ token, onSignedIn }: { token: string; onSignedIn: (me: Me) => void }) {
  const consume = useMutation({ mutationFn: signInWithLink, onSuccess: onSignedIn });
  useOnce(() => {
    if (token) {
      consume.mutate(token);
    }
  });

  if (!token) {
    return <BrokenLink />;
  }

  if (consume.isError) {
    return (
      <AuthStatus
        icon={RiErrorWarningLine}
        tone="error"
        title="This sign-in link no longer works"
        description={consume.error.message}
      >
        <ButtonLink to="/login" className="mt-6 w-full">
          Request a new link
        </ButtonLink>
      </AuthStatus>
    );
  }

  return <Working title="Signing you in…" />;
}

export function VerifyEmailPage({ id, token }: { id: string; token: string }) {
  const verify = useMutation({ mutationFn: () => verifyEmail(id, token) });
  useOnce(() => {
    if (id && token) {
      verify.mutate();
    }
  });

  if (!(id && token)) {
    return <BrokenLink />;
  }

  if (verify.isSuccess) {
    return (
      <AuthStatus
        icon={RiCheckboxCircleLine}
        title="Email verified"
        description="Your email address is proven. Everything that mails your team is now open to you."
      >
        <ButtonLink to="/d" className="mt-6 w-full">
          Continue to Dimmi
        </ButtonLink>
      </AuthStatus>
    );
  }

  if (verify.isError) {
    return (
      <AuthStatus
        icon={RiErrorWarningLine}
        tone="error"
        title="This verification link does not work"
        description={verify.error.message}
      >
        <ButtonLink to="/d" className="mt-6 w-full">
          Continue to Dimmi
        </ButtonLink>
      </AuthStatus>
    );
  }

  return <Working title="Verifying your email…" />;
}
