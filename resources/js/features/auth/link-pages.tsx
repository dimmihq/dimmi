/*
 * Pages a mailed link opens: signing in with a magic link, and proving
 * an email address. Only this page's script spends the link's secret, by
 * posting it once, so a mail scanner that opens the address spends
 * nothing.
 */
import { useMutation } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { ButtonLink } from "@/components/base/buttons/button";
import { LinkButton } from "@/components/base/buttons/link-button";
import { Notification } from "@/components/base/notification/notification";
import { Spinner } from "@/components/base/spinner/spinner";
import { type Me, signInWithLink, verifyEmail } from "@/lib/api/auth";

import { AuthCard, AuthFooter, AuthHeader } from "./auth-frame";
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
      <AuthCard>
        <AuthHeader title="This sign-in link does not work" />
        <div className="mt-6">
          <Notification
            status="error"
            title={consume.error.message}
            dismissible={false}
            role="alert"
            className="w-full"
          />
        </div>
        <AuthFooter>
          <LinkButton href="/login">Request a new link</LinkButton>
        </AuthFooter>
      </AuthCard>
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

  if (verify.isSuccess || verify.isError) {
    return (
      <AuthCard>
        <AuthHeader title={verify.isSuccess ? "Email verified" : "This verification link does not work"} />
        <div className="mt-6">
          <Notification
            status={verify.isSuccess ? "success" : "error"}
            title={verify.isSuccess ? "Your email address is proven." : (verify.error?.message ?? "")}
            dismissible={false}
            className="w-full"
          />
        </div>
        <ButtonLink href="/d" className="mt-6 w-full">
          Continue to Dimmi
        </ButtonLink>
      </AuthCard>
    );
  }

  return <Working title="Verifying your email…" />;
}
