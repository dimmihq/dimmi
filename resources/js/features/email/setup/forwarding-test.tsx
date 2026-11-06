/*
 * The forwarding test of an address: one real email goes to the public
 * address and the row follows it live until it comes back, bounces or
 * runs out of time, with what was learned on the way. A server that
 * cannot send mail still stores the test and says what to send by hand.
 * It reads the test again on every live update and polls only while
 * the live connection is down.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Spinner } from "@/components/base/spinner/spinner";
import { Row } from "@/features/settings/layout";
import { useAccount } from "@/features/settings/use-account";
import { ApiError } from "@/lib/api/client";
import { type EmailAddress, type EmailAddressTest, emailAddressTestQuery, sendEmailAddressTest } from "@/lib/api/email";
import { useRealtime } from "@/lib/realtime/context";
import { channels } from "@/lib/realtime/keys";
import { useChannel } from "@/lib/realtime/use-channel";

import { Failure } from "./failure";

/*
 * ByHand is a test the server could not send: where to send it and the
 * subject that names it.
 */
interface ByHand {
  sendTo: string;
  subject: string;
}

const settled = new Set(["received", "failed", "timed_out"]);

/*
 * statusLine says where the test stands in one sentence.
 */
function statusLine(result: EmailAddressTest | undefined, byHand: ByHand | null): string {
  if (byHand && (!result || !settled.has(result.status))) {
    return `This server cannot send email. Send an email from your own mail to ${byHand.sendTo} with the subject ${byHand.subject}.`;
  }

  switch (result?.status) {
    case "sending":
      return "Sending test";
    case "waiting":
      return "Waiting for it to come back (usually under a minute)";
    case "received":
      return `Forwarding works, took ${Math.max(1, Math.round((result.latency_ms ?? 0) / 1000))} s`;
    case "failed":
      return "The test email did not get through.";
    case "timed_out":
      return "The test email did not come back within 10 minutes.";
    default:
      return "Send a test email to your address to check the forward.";
  }
}

/*
 * Findings lists what the test learned, most serious first as the
 * server sent them.
 */
function Findings({ result }: { result: EmailAddressTest }) {
  return result.findings.map((finding) => (
    <Row
      key={finding.code}
      label={
        <span className={finding.severity === "error" ? "text-text-error-primary" : undefined}>
          {finding.severity === "info" ? "Good to know" : "Fix this"}
        </span>
      }
      description={finding.message}
    >
      {finding.severity === "warn" ? (
        <Chip variant="caption" color="neutral">
          Check
        </Chip>
      ) : null}
    </Row>
  ));
}

/*
 * ForwardingTest is the test row of an address and its findings.
 */
export function ForwardingTest({ address }: { address: EmailAddress }) {
  const me = useAccount();
  const workspace = me.workspace.id;
  const queryClient = useQueryClient();
  const { state } = useRealtime();
  const [testID, setTestID] = useState<string | null>(null);
  const [byHand, setByHand] = useState<ByHand | null>(null);

  const send = useMutation({
    mutationFn: () => sendEmailAddressTest(workspace, address.id),
    onMutate: () => setByHand(null),
    onSuccess: (result) => setTestID(result.id),
    onError: (error) => {
      if (error instanceof ApiError && error.code === "no_sender_available") {
        setTestID(String(error.details.test ?? "") || null);
        setByHand({ sendTo: String(error.details.send_to ?? ""), subject: String(error.details.subject ?? "") });
      }
    },
  });

  const query = useQuery({
    ...emailAddressTestQuery(workspace, address.id, testID ?? ""),
    enabled: testID !== null,
    refetchInterval: (q) => (state === "connected" || settled.has(q.state.data?.status ?? "") ? false : 5_000),
  });
  const result = testID ? query.data : undefined;

  useChannel(channels.workspace(workspace), {
    onEnvelope: (envelope) => {
      const data = envelope.data as { id?: string; status?: string };
      if (envelope.event !== "email_address_test.updated" || data.id !== testID) {
        return;
      }

      void queryClient.invalidateQueries({ queryKey: emailAddressTestQuery(workspace, address.id, data.id).queryKey });
      if (data.status === "received") {
        void queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "email-addresses"], exact: true });
      }
    },
  });

  const running = send.isPending || result?.status === "sending" || result?.status === "waiting";
  const shownError = send.error instanceof ApiError && send.error.code === "no_sender_available" ? null : send.error;

  return (
    <>
      <Row
        label="Forwarding test"
        description={
          <span aria-live="polite" data-testid="forwarding-test-status">
            {send.isPending ? "Sending test" : statusLine(result, byHand)}
          </span>
        }
      >
        {running ? <Spinner size="sm" aria-label="Test running" /> : null}
        {result?.status === "received" ? (
          <Chip variant="caption" color="lime">
            Works
          </Chip>
        ) : null}
        <Button variant="secondary" size="small" disabled={running} onClick={() => send.mutate()}>
          {result && settled.has(result.status) && result.status !== "received" ? "Retry test" : "Test forwarding"}
        </Button>
      </Row>
      {result ? <Findings result={result} /> : null}
      <Failure error={shownError} />
    </>
  );
}
