import { useSuspenseQuery } from "@tanstack/react-query";
import { createFileRoute } from "@tanstack/react-router";

import { ResetPasswordForm } from "@/features/auth/password-reset-forms";
import { useSignedIn } from "@/features/auth/use-signed-in";
import { authConfigQuery } from "@/lib/api/auth";
import { fragment } from "@/lib/location";

export const Route = createFileRoute("/_auth/reset-password")({
  component: ResetPasswordPage,
});

function ResetPasswordPage() {
  const { data: config } = useSuspenseQuery(authConfigQuery);

  return (
    <ResetPasswordForm
      id={fragment("id")}
      token={fragment("token")}
      minLength={config.password_min_length}
      onSignedIn={useSignedIn("/d")}
    />
  );
}
