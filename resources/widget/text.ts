/*
 * Small text helpers the components share.
 */

export function initials(name: string): string {
  return name
    .split(" ")
    .map((part) => part[0])
    .join("")
    .slice(0, 2);
}

/*
 * The bar heights of a voice note's waveform, in pixels. Recordings will
 * carry their own levels; until then every note draws this shape.
 */
export const levels = [5, 9, 14, 8, 18, 12, 20, 10, 15, 7, 17, 11, 19, 9, 13, 6, 16, 10, 12, 8, 14, 6, 9, 5];

/*
 * ActionTexts are the words of an action card: its two buttons, the
 * steps of confirming an email with a code, and a line for each state
 * it can end in.
 */
export interface ActionTexts {
  confirm: string;
  cancel: string;
  verifyIntro: string;
  verifyEmail: string;
  verifySend: string;
  verifySent: string;
  verifyCode: string;
  verifySubmit: string;
  verifyResend: string;
  verifyOther: string;
  waiting: string;
  done: string;
  declined: string;
  canceled: string;
  expired: string;
  failed: string;
}

export const actionTexts: ActionTexts = {
  confirm: "Confirm",
  cancel: "Cancel",
  verifyIntro: "Confirm your email to continue",
  verifyEmail: "Email",
  verifySend: "Send code",
  verifySent: "Enter the 6-digit code we sent to {email}",
  verifyCode: "Code",
  verifySubmit: "Verify",
  verifyResend: "Send a new code",
  verifyOther: "Use another email",
  waiting: "Waiting for the team to review this",
  done: "Done",
  declined: "Not approved",
  canceled: "Canceled",
  expired: "This request expired",
  failed: "This didn't go through",
};
