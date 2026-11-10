/*
 * The button the integrations pages share to send the browser to a tool
 * to approve an install, or the permissions an installation still
 * needs.
 */
import { useMutation } from "@tanstack/react-query";

import { Button } from "@/components/base/buttons/button";
import { toast } from "@/components/base/toast/toast";
import { startInstallation } from "@/lib/api/integrations";

/*
 * ConnectButton starts installing the tool, or updating the
 * permissions of its installation, and sends the browser to approve.
 * The tool sends it back to the page it left.
 */
export function ConnectButton({
  workspace,
  provider,
  label,
  variant = "secondary",
}: {
  workspace: string;
  provider: string;
  label: string;
  variant?: "primary" | "secondary";
}) {
  const start = useMutation({
    mutationFn: () => startInstallation(workspace, provider, { return_to: window.location.pathname }),
    onSuccess: ({ authorize_url }) => window.location.assign(authorize_url),
    onError: (error) => toast.error(error.message),
  });

  return (
    <Button variant={variant} size="small" loading={start.isPending} onClick={() => start.mutate()}>
      {label}
    </Button>
  );
}
