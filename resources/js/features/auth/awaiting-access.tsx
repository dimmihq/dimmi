/*
 * What a person brought over from another tool sees once signed in,
 * while their only way into a workspace still waits for an admin: the
 * workspace by name, and nothing of what is inside it. They can check
 * again or sign out; an email tells them once they are let in.
 */
import { RiTimeLine } from "@remixicon/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { type AwaitingAccess, meQuery, signOut } from "@/lib/api/auth";

import { AuthStatus } from "./auth-frame";
import { awaitingLine } from "./awaiting-text";

export function AwaitingAccessPage({ awaiting }: { awaiting: AwaitingAccess[] }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const check = useMutation({
    mutationFn: () => queryClient.invalidateQueries({ queryKey: meQuery.queryKey }),
    onSuccess: () => router.history.push("/d"),
  });
  const leave = useMutation({
    mutationFn: signOut,
    onSuccess: () => {
      queryClient.setQueryData(meQuery.queryKey, null);
      router.history.push("/login");
    },
  });

  return (
    <AuthStatus
      icon={RiTimeLine}
      title="Waiting for an admin to enable your access"
      description={awaitingLine(awaiting)}
    >
      <div className="mt-6 flex flex-col gap-2">
        <Button className="w-full" loading={check.isPending} onClick={() => check.mutate()}>
          Check again
        </Button>
        <Button className="w-full" variant="secondary" loading={leave.isPending} onClick={() => leave.mutate()}>
          Sign out
        </Button>
      </div>
    </AuthStatus>
  );
}
