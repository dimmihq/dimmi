/*
 * How replies from one address look: plain, as a person writes, or
 * branded with the company's color and a footer. The preview is a sample
 * reply from the server in the style chosen, before it is saved, so it
 * matches the mail customers get. It sits in a sandboxed frame because
 * it is mail, and the brand color is the customer's own data that only
 * the mail uses; the frame has an origin of its own and inherits the
 * page's content security policy, which runs no script without the
 * page's nonce. The style and color save as they change; the footer
 * saves with its button.
 */
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { SegmentedControl, SegmentedControlItem } from "@/components/base/segmented-control/segmented-control";
import { Textarea } from "@/components/base/textarea/textarea";
import { Colour } from "@/features/settings/controls";
import { Row } from "@/features/settings/layout";
import { type EmailAddress, type ReplyStyle, renderEmailTemplate, updateEmailAddress } from "@/lib/api/email";

import { Failure } from "./failure";

const fallbackColor = "#171717";

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

export function TemplateSettings({ workspace, address }: { workspace: string; address: EmailAddress }) {
  const queryClient = useQueryClient();
  const saved = address.settings;
  const [style, setStyle] = useState<ReplyStyle>({
    template: saved.template ?? "plain",
    brand_color: saved.brand_color || fallbackColor,
    footer_markdown: saved.footer_markdown ?? "",
  });
  const preview = usePreview(workspace, address.id, style);

  const save = useMutation({
    mutationFn: (changes: Partial<ReplyStyle>) => updateEmailAddress(workspace, address.id, { settings: changes }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "email-addresses"] }),
  });
  const change = (changes: Partial<ReplyStyle>, keep = true) => {
    setStyle((current) => ({ ...current, ...changes }));
    if (keep) {
      save.mutate(changes);
    }
  };

  return (
    <>
      <Row label="Reply style" description="Plain reads as a person wrote it. Branded adds your color and a footer.">
        <SegmentedControl
          aria-label={`Reply style for ${address.address}`}
          selectedKeys={[style.template]}
          onSelectionChange={(keys) => {
            const [picked] = [...keys];
            if (picked === "plain" || picked === "branded") {
              change({ template: picked });
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
              label={`Brand color for ${address.address}`}
              value={style.brand_color}
              onChange={(hex) => change({ brand_color: hex })}
            />
          </Row>
          <div className="flex flex-col gap-3 py-3 pe-3">
            <Textarea
              label="Footer"
              hint="Bold, italic and links, below every reply."
              rows={2}
              maxLength={500}
              value={style.footer_markdown}
              onChange={(footer) => change({ footer_markdown: footer }, false)}
            />
            <div className="flex">
              <Button
                variant="secondary"
                size="small"
                loading={save.isPending}
                onClick={() => save.mutate({ footer_markdown: style.footer_markdown })}
              >
                Save footer
              </Button>
            </div>
          </div>
        </>
      ) : null}
      {preview.data ? (
        <div className="py-3 pe-3">
          <iframe
            title={`Reply preview for ${address.address}`}
            sandbox="allow-scripts"
            srcDoc={preview.data.html}
            className="h-80 w-full rounded-xl border border-border-button-default bg-background-primary-default"
          />
        </div>
      ) : null}
      <Failure error={preview.error ?? save.error} />
    </>
  );
}
