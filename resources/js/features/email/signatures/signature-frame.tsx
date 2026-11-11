/*
 * A rendered signature in a sandboxed frame, alone or closing a sample
 * reply. The frame has an origin of its own, so nothing in it reaches
 * the page, and it inherits the page's content security policy, which
 * runs no script without the page's nonce.
 */
import { cx } from "@/utils/cx";

import { framedSignature, replyWith } from "./signature-preview";

/*
 * SignatureFrame shows a signature's rendered HTML, alone or closing the
 * sample reply.
 */
export function SignatureFrame({
  title,
  html,
  reply = false,
  className,
}: {
  title: string;
  html: string;
  reply?: boolean;
  className?: string;
}) {
  const document = reply ? replyWith(html) : framedSignature(html);

  return (
    <iframe
      title={title}
      sandbox="allow-scripts"
      srcDoc={document}
      className={cx("w-full rounded-xl border border-border-button-default bg-background-primary-default", className)}
    />
  );
}
