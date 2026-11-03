/*
 * The signed-in landing page until the inbox exists: who is signed in,
 * the theme switch, and a way out.
 */
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";

import { ThemeToggle } from "@/components/application/theme/theme-toggle";
import { Button } from "@/components/base/buttons/button";
import { Notification } from "@/components/base/notification/notification";
import { meQuery, signOut } from "@/lib/api/auth";

export function Home() {
  const { data: me } = useSuspenseQuery(meQuery);
  const queryClient = useQueryClient();
  const router = useRouter();
  const leave = useMutation({
    mutationFn: signOut,
    onSuccess: () => {
      queryClient.setQueryData(meQuery.queryKey, null);
      router.history.push("/login");
    },
  });

  if (!me) {
    return null;
  }

  return (
    <div className="flex min-h-screen flex-col bg-background-full">
      <header className="flex items-center justify-between p-4 sm:p-6">
        <span className="text-title-3-semibold text-text-primary">Dimmi</span>
        <div className="flex items-center gap-3">
          <ThemeToggle appearance="segmented" size="small" />
          <Button variant="secondary" size="small" onClick={() => leave.mutate()} disabled={leave.isPending}>
            Sign out
          </Button>
        </div>
      </header>
      <main className="mx-auto flex w-full max-w-[560px] flex-col gap-4 px-4 pt-8">
        <h1 className="text-title-1-medium text-text-primary">Hi {me.user.name}</h1>
        <p className="text-body-regular text-text-secondary">You are signed in as {me.user.email}.</p>
        {me.user.email_verified ? null : (
          <Notification
            status="information"
            title="Verify your email address"
            description="We sent you a link. Open it to prove the address is yours."
            dismissible={false}
            className="w-full"
          />
        )}
      </main>
    </div>
  );
}
