/*
 * What to do after adding a support address: forward its mail to the
 * workspace's private receiving address. The address is shown with a copy
 * button, then the steps for the mail service it lives in, the Gmail
 * confirmation code once Gmail sends one, and a test email to check the
 * forward. The page asks again every few seconds until the first email
 * arrives, so the status turns to receiving without a reload.
 */
import { useMutation, useQuery } from "@tanstack/react-query";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { CopyButton } from "@/features/email/setup/copy-button";
import { ForwardingGuide } from "@/features/email/setup/forwarding-guide";
import { Row } from "@/features/settings/layout";
import { type EmailAddress, emailAddressesQuery, sendEmailAddressTest } from "@/lib/api/email";

const checkEvery = 5000;

function Status({ address }: { address: EmailAddress }) {
  if (address.inbound_verified_at) {
    return (
      <Chip variant="caption" color="lime">
        Receiving
      </Chip>
    );
  }

  return (
    <Chip variant="caption" color="neutral">
      Waiting for the first email
    </Chip>
  );
}

export function EmailForwarding({ workspace, address }: { workspace: string; address: EmailAddress }) {
  const { data } = useQuery({
    ...emailAddressesQuery(workspace),
    refetchInterval: (query) =>
      query.state.data?.find((a) => a.id === address.id)?.inbound_verified_at ? false : checkEvery,
  });
  const current = data?.find((a) => a.id === address.id) ?? address;
  const test = useMutation({ mutationFn: () => sendEmailAddressTest(workspace, current.id) });
  const confirmation = current.forwarding_confirmation;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <bdi dir="ltr" className="text-body-2-medium text-text-primary">
          {current.address}
        </bdi>
        <Status address={current} />
      </div>
      <div className="flex flex-col rounded-2xl bg-background-secondary-default ps-3">
        <Row label="Forward to" description="Forward every email sent to your address here.">
          <bdi dir="ltr" className="font-mono text-body-2-regular text-text-primary">
            {current.forwarding_address}
          </bdi>
          <CopyButton value={current.forwarding_address} label={`Copy ${current.forwarding_address}`} />
        </Row>
        <ForwardingGuide guides={current.setup_guides} />
        {confirmation ? (
          <Row label="Gmail confirmation code" description="Enter it in Gmail to finish turning on forwarding.">
            <span className="font-mono text-body-medium text-text-primary">{confirmation.code}</span>
          </Row>
        ) : null}
        <Row
          label="Check the forward"
          description={
            test.data
              ? `Sent to ${test.data.sent_to}. The status above turns to receiving once it comes back.`
              : "Send a test email to your address once forwarding is on."
          }
        >
          <Button variant="secondary" size="small" loading={test.isPending} onClick={() => test.mutate()}>
            Send test email
          </Button>
        </Row>
      </div>
    </div>
  );
}
