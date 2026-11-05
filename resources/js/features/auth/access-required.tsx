/*
 * Shown over a page when the workspace on screen will not let the person
 * work until they meet one of its rules: what it asks and the button that
 * goes and does it. Signing in again comes back to the same page, so
 * nothing is lost. The page where a rule is met, such as adding a second
 * factor, is never covered, and an unknown rule still shows the server's
 * own message.
 */
import { useRouterState } from "@tanstack/react-router";
import { type ReactNode, useEffect, useSyncExternalStore } from "react";

import { Button } from "@/components/base/buttons/button";
import {
  type AccessRequirement,
  accessRequirement,
  reportAccessRequirement,
  subscribeToAccessRequirement,
} from "@/lib/api/access-required";

import { destination } from "./access-destination";

const titles: Record<string, string> = {
  ip_not_allowed: "Your address is not allowed here",
  sso_required: "Sign in with single sign-on",
  sign_in_method: "Sign in another way",
  reauthentication_required: "Sign in again to continue",
  two_factor: "Turn on two-factor authentication",
};

const actions: Record<string, string> = {
  sso_required: "Continue with single sign-on",
  sign_in_method: "Sign in again",
  reauthentication_required: "Sign in again",
  two_factor: "Open security settings",
};

function AccessRequiredPanel({ required, here }: { required: AccessRequirement; here: string }) {
  const { actionUrl } = required;

  return (
    <div role="alert" className="flex h-full min-w-0 flex-1 items-center justify-center p-6">
      <div className="flex max-w-md flex-col items-start gap-3 rounded-3xl border border-border-button-default bg-background-secondary-default p-6">
        <h1 className="text-title-3-semibold text-text-primary">
          {titles[required.requirement] ?? "This workspace needs something from you"}
        </h1>
        <p className="text-body-regular text-text-secondary">{required.message}</p>
        {required.ip ? (
          <p className="text-body-2-regular text-text-tertiary">
            Your address: <bdi dir="ltr">{required.ip}</bdi>
          </p>
        ) : null}
        {actionUrl ? (
          <Button onClick={() => window.location.assign(destination(actionUrl, here))}>
            {actions[required.requirement] ?? "Continue"}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export function AccessRequired({ children }: { children: ReactNode }) {
  const required = useSyncExternalStore(subscribeToAccessRequirement, accessRequirement);
  const path = useRouterState({ select: (state) => state.location.pathname });
  const here = useRouterState({ select: (state) => state.location.href });

  useEffect(() => {
    if (path) {
      reportAccessRequirement(null);
    }

    return () => reportAccessRequirement(null);
  }, [path]);

  if (!required || required.actionUrl === path) {
    return <>{children}</>;
  }

  return (
    <>
      <div className="hidden">{children}</div>
      <AccessRequiredPanel required={required} here={here} />
    </>
  );
}
