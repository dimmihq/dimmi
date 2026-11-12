/*
 * The page the unsubscribe link in a teammate email opens, with no
 * sign-in: the link's token names the member and the kind of email.
 * Opening it turns that email off, then says so plainly, offers it
 * back, and points to the full notification settings. A link that no
 * longer works says so calmly instead of showing an error.
 */
import { RiCheckLine, RiLinkUnlinkM, RiMailCheckLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { Spinner } from "@/components/base/spinner/spinner";
import { AuthCard, AuthStatus } from "@/features/auth/auth-frame";
import { ApiError } from "@/lib/api/client";
import {
  type NotificationUnsubscribe,
  resubscribeNotifications,
  unsubscribeNotifications,
} from "@/lib/api/notification-deliveries";

function Gone() {
  return (
    <AuthStatus
      icon={RiLinkUnlinkM}
      title="This link no longer works"
      description="Change which emails reach you in your notification settings, or use the link in a newer email."
    />
  );
}

function Done({ token, state }: { token: string; state: NotificationUnsubscribe }) {
  const queryClient = useQueryClient();
  const key = ["notification-unsubscribes", token, "off"];
  const change = useMutation({
    mutationFn: () => (state.subscribed ? unsubscribeNotifications(token) : resubscribeNotifications(token)),
    onSuccess: (next) => queryClient.setQueryData(key, next),
  });
  const what = state.label.toLowerCase();

  return (
    <AuthStatus
      icon={state.subscribed ? RiMailCheckLine : RiCheckLine}
      title={state.subscribed ? "You're subscribed again" : "You're unsubscribed"}
      description={
        state.subscribed
          ? `${state.workspace} will email ${state.email} about ${what} again.`
          : `${state.workspace} will no longer email ${state.email} about ${what}. The bell in the dashboard still keeps them.`
      }
    >
      {change.isError ? (
        <p role="alert" className="mt-4 text-body-2-regular text-text-error-primary">
          {change.error.message}
        </p>
      ) : null}
      <div className="mt-6 flex flex-wrap items-center gap-3">
        <Button variant="secondary" onClick={() => change.mutate()} loading={change.isPending}>
          {state.subscribed ? "Unsubscribe" : "Resubscribe"}
        </Button>
        <a href={state.settings_url} className="text-body-medium text-text-primary underline underline-offset-2">
          Notification settings
        </a>
      </div>
    </AuthStatus>
  );
}

export function NotificationUnsubscribePage({ token }: { token: string }) {
  const { data, isPending, error } = useQuery({
    queryKey: ["notification-unsubscribes", token, "off"],
    queryFn: () => unsubscribeNotifications(token),
    enabled: token !== "",
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
  });
  if (!token || (error instanceof ApiError && error.status === 404)) {
    return <Gone />;
  }
  if (error) {
    return <AuthStatus icon={RiLinkUnlinkM} tone="error" title="Something went wrong" description={error.message} />;
  }
  if (isPending || !data) {
    return (
      <AuthCard>
        <Spinner size="sm" aria-label="Loading" />
      </AuthCard>
    );
  }

  return <Done token={token} state={data} />;
}
