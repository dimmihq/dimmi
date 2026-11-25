/*
 * The workspace's standing as a sender: whether its bulk email
 * (broadcasts, series and rating emails) is paused because too much of
 * its mail bounced or was reported as spam, the last week's counts it
 * is judged by, and the call an owner makes to resume it.
 */
import { queryOptions } from "@tanstack/react-query";

import { request } from "./client";

export type SendingPauseReason = "bounce_rate" | "complaint_rate";

export interface EmailSending {
  id?: never;
  object: "email_sending";
  status: "active" | "paused";
  paused_at: string | null;
  paused_reason: SendingPauseReason | null;
  sent: number;
  hard_bounced: number;
  complained: number;
  window_days: number;
  minimum_sent: number;
  bounce_pause_rate: number;
  complaint_pause_rate: number;
}

export function emailSendingKey(workspace: string) {
  return ["workspace", workspace, "email-sending"];
}

export function emailSendingQuery(workspace: string) {
  return queryOptions({
    queryKey: emailSendingKey(workspace),
    queryFn: () => request<EmailSending>("GET", "/email-sending", undefined, { workspace }),
  });
}

export function resumeEmailSending(workspace: string): Promise<EmailSending> {
  return request<EmailSending>("POST", "/email-sending/resumptions", undefined, { workspace });
}
