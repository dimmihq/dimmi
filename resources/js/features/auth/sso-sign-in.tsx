/*
 * Signing in through a company's identity provider. The person types
 * their work address and is sent to their company's sign-in page; a
 * company that requires it sends every password sign-in there too. When
 * the identity provider is down, an owner may sign in with their
 * password around it, and every owner hears about it.
 */
import { useMutation } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { Notification } from "@/components/base/notification/notification";
import { LinkButton } from "@/features/navigation/links";
import { lookupSSO, type SSOLookup } from "@/lib/api/auth";

import { ssoStart } from "./sso-start";

/*
 * SSOButton looks the address up and sends the browser to its company's
 * identity provider, or says the address has none.
 */
export function SSOButton({ email, redirect }: { email: string; redirect: string }) {
  const lookup = useMutation({
    mutationFn: () => lookupSSO(email),
    onSuccess: (found) => {
      const start = ssoStart(found, redirect);
      if (start) {
        window.location.assign(start);
      }
    },
  });
  const missing = lookup.data && !lookup.data.redirect_url;

  return (
    <>
      <Button
        variant="secondary"
        className="w-full"
        disabled={!email || lookup.isPending}
        onClick={() => lookup.mutate()}
      >
        {lookup.isPending ? "Looking up…" : "Continue with single sign-on"}
      </Button>
      {missing ? (
        <p role="alert" className="text-body-2-regular text-text-error-primary">
          Your address does not sign in with single sign-on here.
        </p>
      ) : null}
    </>
  );
}

/*
 * SSORequired tells the person their company signs in through its
 * identity provider, sends them there, and offers owners the way around
 * it.
 */
export function SSORequired({
  lookup,
  redirect,
  bypassing,
  onBypass,
}: {
  lookup: SSOLookup;
  redirect: string;
  bypassing: boolean;
  onBypass: () => void;
}) {
  const start = ssoStart(lookup, redirect);

  return (
    <div className="mt-6 flex w-full flex-col gap-3">
      <Notification
        status="information"
        title={
          bypassing
            ? "Owner sign-in: enter your password. Every owner will be told."
            : `Your company signs in with ${lookup.name ?? "single sign-on"}.`
        }
        dismissible={false}
        role="status"
      />
      {start && !bypassing ? (
        <Button className="w-full" onClick={() => window.location.assign(start)}>
          {`Continue with ${lookup.name ?? "single sign-on"}`}
        </Button>
      ) : null}
      {bypassing ? null : (
        <div className="flex justify-center">
          <LinkButton onClick={onBypass}>Owner sign-in without single sign-on</LinkButton>
        </div>
      )}
    </div>
  );
}
