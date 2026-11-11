/*
 * How a signature reads at the end of a real reply. The server renders
 * it, so the details are filled in for the teammate looking and the
 * address it signs for, and a line whose detail is empty is left out
 * exactly as a reply leaves it out. The server also strips the
 * signature down to text, emphasis and links before it is shown.
 */
import { useQuery } from "@tanstack/react-query";
import { useDeferredValue } from "react";

import { renderSignature, signaturesKey } from "@/lib/api/email-signatures";

/*
 * useSignaturePreview renders the text as a reply shows it, a moment
 * after typing stops changing it.
 */
export function useSignaturePreview(workspace: string, body: string, address?: string) {
  const settled = useDeferredValue(body.trim());

  return useQuery({
    queryKey: [...signaturesKey(workspace), "rendering", settled, address ?? ""],
    queryFn: () => renderSignature(workspace, settled, address),
    enabled: settled !== "",
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
  });
}

const replyStyle = [
  "body{margin:0;padding:20px 24px;background:#fff;color:#171717;",
  "font:14px/1.5 -apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif}",
  "p{margin:0 0 12px}a{color:inherit}",
  ".quote{margin-top:24px;padding-top:12px;border-top:1px solid #e5e5e5;color:#737373}",
  "blockquote{margin:8px 0 0;padding-left:12px;border-left:2px solid #e5e5e5}",
].join("");

const sampleReply = [
  "Hi Alex,",
  "Thanks for getting in touch. The refund is on its way and should reach your account within five days.",
  "Let us know if anything else comes up.",
];

/*
 * replyWith writes a sample reply closed by the signature's HTML, above
 * the message it answers, the way a mail client shows it.
 */
export function replyWith(signature: string): string {
  const reply = sampleReply.map((line) => `<p>${line}</p>`).join("");
  const quote =
    '<div class="quote">Alex wrote:<blockquote>Hello, I was charged twice for my order. ' +
    "Could you look into it?</blockquote></div>";

  return `<!doctype html><style>${replyStyle}</style>${reply}${signature}${quote}`;
}

/*
 * framedSignature writes the signature's HTML alone, in the reply's
 * type, for a small preview.
 */
export function framedSignature(signature: string): string {
  return `<!doctype html><style>${replyStyle}body{padding:12px 16px}</style>${signature}`;
}
