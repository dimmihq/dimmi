/*
 * The notice at the top of the email settings while the workspace's
 * bulk email is paused: why it paused, with the numbers it was judged
 * by, what still goes out, and, for those who may, the way to resume.
 * Nothing shows while sending is active.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Notification } from "@/components/base/notification/notification";
import { toast } from "@/components/base/toast/toast";
import { type EmailSending, emailSendingKey, emailSendingQuery, resumeEmailSending } from "@/lib/api/email-sending";

const percent = new Intl.NumberFormat(undefined, { style: "percent", maximumFractionDigits: 2 });

function why(sending: EmailSending): string {
  const days = `the last ${sending.window_days} days`;

  if (sending.paused_reason === "complaint_rate") {
    return `${sending.complained} of ${sending.sent} emails in ${days} were reported as spam, past the ${percent.format(sending.complaint_pause_rate)} limit.`;
  }

  return `${sending.hard_bounced} of ${sending.sent} emails in ${days} bounced because the address doesn't exist, past the ${percent.format(sending.bounce_pause_rate)} limit.`;
}

export function SendingPaused({ workspace, canResume }: { workspace: string; canResume: boolean }) {
  const { data: sending } = useQuery(emailSendingQuery(workspace));
  const queryClient = useQueryClient();
  const resume = useMutation({
    mutationFn: () => resumeEmailSending(workspace),
    onSuccess: (next) => {
      queryClient.setQueryData(emailSendingKey(workspace), next);
      toast.success("Sending resumed. Resume each paused broadcast from Broadcasts.");
    },
    onError: (error) => toast.error(error.message),
  });

  if (sending?.status !== "paused") {
    return null;
  }

  const next = canResume
    ? "Clean the list you send to, then resume. The count starts again from there."
    : "Ask a workspace owner to resume sending once the list is clean.";

  return (
    <div className="pb-4">
      <Notification
        status="error"
        role="alert"
        dismissible={false}
        className="w-full"
        title="Broadcasts, series and rating emails are paused"
        description={`${why(sending)} Replies to customers still go out. ${next}`}
        actions={
          canResume
            ? [{ label: resume.isPending ? "Resuming…" : "Resume sending", onClick: () => resume.mutate() }]
            : undefined
        }
      />
    </div>
  );
}
