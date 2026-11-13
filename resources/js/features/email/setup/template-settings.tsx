/*
 * How replies from one address look: plain, as a person writes, or
 * branded with the company's color and a footer. The preview is a sample
 * reply from the server in the style chosen, before it is saved, so it
 * matches the mail customers get. It sits in a sandboxed frame because
 * it is mail, and the brand color is the customer's own data that only
 * the mail uses; the frame has an origin of its own and inherits the
 * page's content security policy, which runs no script without the
 * page's nonce. The style, color and footer are held in the address
 * page's draft, previewed as they change and sent with its save bar.
 */
import { useQuery } from "@tanstack/react-query";
import { useDeferredValue } from "react";

import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Textarea } from "@/components/base/textarea/textarea";
import { Colour } from "@/features/settings/controls";
import { Row } from "@/features/settings/layout";
import { type EmailAddress, type ReplyStyle, renderEmailTemplate } from "@/lib/api/email";

import type { AddressDraft } from "./address-card";
import { Failure } from "./failure";

/*
 * framed gives a plain reply room inside the preview, the margin a mail
 * client puts around a message, without changing the email itself.
 */
function framed(html: string): string {
  const room = "<style>body{margin:0;padding:20px 24px}</style>";
  const doctype = /^\s*<!doctype[^>]*>/i.exec(html)?.[0] ?? "";

  return doctype + room + html.slice(doctype.length);
}

const styleLabels: Record<ReplyStyle["template"], string> = { plain: "Plain", branded: "Branded" };

/*
 * usePreview renders a sample reply in the style, a moment after the
 * style stops changing.
 */
function usePreview(workspace: string, address: string, style: ReplyStyle) {
  const settled = useDeferredValue(style);

  return useQuery({
    queryKey: ["workspace", workspace, "email-addresses", address, "template-rendering", settled],
    queryFn: () => renderEmailTemplate(workspace, address, settled),
    retry: false,
    staleTime: Number.POSITIVE_INFINITY,
  });
}

export function TemplateSettings({
  workspace,
  address,
  draft,
  settings,
}: {
  workspace: string;
  address: EmailAddress;
  draft: AddressDraft;
  settings: EmailAddress["settings"];
}) {
  const style: ReplyStyle = {
    template: settings.template,
    brand_color: settings.brand_color,
    footer_markdown: settings.footer_markdown,
  };
  const preview = usePreview(workspace, address.id, style);

  return (
    <>
      <Row label="Reply style" description="Plain reads as a person wrote it. Branded adds your color and a footer.">
        <SegmentedControl
          aria-label={`Reply style for ${address.address}`}
          selectedKeys={[style.template]}
          onSelectionChange={(keys) => {
            const [picked] = [...keys];
            if (picked === "plain" || picked === "branded") {
              draft.set("template", picked);
            }
          }}
        >
          {Object.entries(styleLabels).map(([key, label]) => (
            <SegmentedControlItem key={key} id={key}>
              {label}
            </SegmentedControlItem>
          ))}
        </SegmentedControl>
      </Row>
      {style.template === "branded" ? (
        <>
          <Row label="Brand color" description="The line above each reply.">
            <Colour
              key={`color-${draft.generation}`}
              label={`Brand color for ${address.address}`}
              value={style.brand_color}
              onChange={(hex) => draft.set("brand_color", hex)}
            />
          </Row>
          <div className="flex flex-col gap-3 py-3 pe-3">
            <Textarea
              label="Footer"
              hint="Bold, italic and links, below every reply."
              rows={2}
              maxLength={500}
              value={style.footer_markdown}
              onChange={(footer) => draft.set("footer_markdown", footer)}
            />
          </div>
        </>
      ) : null}
      {preview.data ? (
        <div className="py-3 pe-3">
          <iframe
            title={`Reply preview for ${address.address}`}
            sandbox="allow-scripts"
            srcDoc={style.template === "plain" ? framed(preview.data.html) : preview.data.html}
            className="h-80 w-full rounded-xl border border-border-button-default bg-background-primary-default"
          />
        </div>
      ) : null}
      <Failure error={preview.error} />
    </>
  );
}
