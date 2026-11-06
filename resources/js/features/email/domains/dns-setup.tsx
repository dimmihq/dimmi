/*
 * Automatic setup at the domain's DNS host, where the host offers it:
 * one button sends the customer to approve the records there, and the
 * host sends them back here. Back on the page, an approval starts a
 * check at once, and a refusal or error says so in plain words; the
 * records below stay the same either way.
 */
import { useMutation } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Failure } from "@/features/email/setup/failure";
import { Row } from "@/features/settings/layout";
import { type EmailDomain, startDNSSetup } from "@/lib/api/email-domains";

/*
 * useReturn reads how the host sent the customer back, once, and clears
 * it from the address so a reload does not repeat it.
 */
function useReturn(onApplied: () => void): string | null {
  const [refused, setRefused] = useState<string | null>(null);
  const handled = useRef(false);

  useEffect(() => {
    if (handled.current) {
      return;
    }
    handled.current = true;

    const params = new URLSearchParams(window.location.search);
    if (!params.has("dns_setup") && !params.has("error")) {
      return;
    }

    window.history.replaceState(null, "", window.location.pathname);
    if (params.has("error")) {
      setRefused("Your DNS host did not add the records. Add them by hand below, or try again.");
      return;
    }
    onApplied();
  }, [onApplied]);

  return refused;
}

export function DNSSetup({
  workspace,
  domain,
  manage,
  onApplied,
}: {
  workspace: string;
  domain: EmailDomain;
  manage: boolean;
  onApplied: () => void;
}) {
  const refused = useReturn(onApplied);
  const start = useMutation({
    mutationFn: () => startDNSSetup(workspace, domain.id),
    onSuccess: (setup) => window.location.assign(setup.apply_url),
  });

  return (
    <>
      {refused ? <Row label="Automatic setup" description={refused} /> : null}
      {domain.dns_setup && manage && domain.status !== "verified" ? (
        <Row
          label="Automatic setup"
          description={`${domain.dns_setup.host_name} can add every record for you after you approve it there.`}
        >
          <Button size="small" loading={start.isPending} onClick={() => start.mutate()}>
            {`Set up automatically at ${domain.dns_setup.host_name}`}
          </Button>
        </Row>
      ) : null}
      <Failure error={start.error} />
    </>
  );
}
