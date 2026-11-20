/*
 * WhatsApp in the composer. Within a day of the customer's last message a
 * reply goes out as typed, and a chip says how long that lasts. Typing /
 * finds the number's approved templates, like # finds saved replies; a
 * picked template takes the field's place with its variables (the
 * customer's name ready where it fits) and the template as the customer
 * will read it, and the send button sends it. After the day is over only
 * a template goes out, and the field says so.
 */
import { RiTimeLine, RiWhatsappLine } from "@remixicon/react";
import { useEffect, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Divider } from "@/components/base/divider/divider";
import { Input } from "@/components/base/input/input";
import { Kbd } from "@/components/base/kbd/kbd";
import type { WhatsAppConversation, WhatsAppTemplate } from "@/lib/api/whatsapp";

import { type TemplateReply, timeLeft, variableKey, variableLabel } from "./whatsapp-state";

/*
 * WindowChip shows how long the window stays open, warning in its last
 * two hours.
 */
export function WindowChip({ state }: { state: WhatsAppConversation | undefined }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const tick = window.setInterval(() => setNow(Date.now()), 60_000);
    return () => window.clearInterval(tick);
  }, []);

  const left = state?.window.open ? timeLeft(state.window.expires_at, now) : null;
  if (!left) {
    return null;
  }

  const closing = Date.parse(state?.window.expires_at ?? "") - now < 2 * 3_600_000;

  return (
    <Chip
      variant="caption"
      color={closing ? "orange" : "soft"}
      className="gap-1"
      title="WhatsApp takes free text for 24 hours after the customer's last message."
    >
      <RiTimeLine aria-hidden className="size-3" />
      {left}
    </Chip>
  );
}

/*
 * ClosedWindow says, above the field, that the window closed and how to
 * send a template instead.
 */
export function ClosedWindow({
  customer,
  state,
  reply,
}: {
  customer: string;
  state: WhatsAppConversation | undefined;
  reply: TemplateReply;
}) {
  if (!state || state.window.open || reply.picked) {
    return null;
  }

  return (
    <p className="flex items-center gap-1.5 px-1.5 text-body-2-regular text-text-secondary">
      <RiTimeLine aria-hidden className="size-3.5 shrink-0 text-foreground-icon-tertiary" />
      <span>
        More than 24 hours passed since {customer} last wrote. Type <Kbd>/</Kbd> to send an approved template; their
        reply opens the window again.
      </span>
    </p>
  );
}

/*
 * TemplateCard stands in the field's place once a template is picked:
 * its variables, the template as the customer will read it, and a way
 * back to typing.
 */
export function TemplateCard({ reply, customer }: { reply: TemplateReply; customer: string }) {
  const picked = reply.picked;
  if (!picked) {
    return null;
  }

  return (
    <div className="flex flex-col gap-3 px-1.5">
      <div className="flex items-center gap-2">
        <RiWhatsappLine aria-hidden className="size-4 shrink-0 text-foreground-icon-secondary" />
        <span className="min-w-0 truncate text-body-medium text-text-primary">/{picked.name}</span>
        <span className="shrink-0 text-body-2-regular text-text-tertiary">
          {picked.language} · {picked.category}
        </span>
        <Button variant="ghost" size="xs" className="ms-auto" onClick={reply.clear}>
          Remove
        </Button>
      </div>
      <div className="flex flex-col gap-3 rounded-2xl bg-background-secondary-default px-3.5 py-3">
        {picked.variables.length > 0 ? (
          <div className="grid gap-2 sm:grid-cols-2">
            {picked.variables.map((v) => (
              <Input
                key={variableKey(v)}
                size="small"
                label={variableLabel(picked, v)}
                value={reply.values[variableKey(v)] ?? ""}
                onChange={(text) => reply.setValue(variableKey(v), text)}
              />
            ))}
          </div>
        ) : null}
        {picked.variables.length > 0 ? <Divider /> : null}
        <div>
          <div className="mb-1.5 text-caption-1-medium text-text-tertiary">{customer} will read</div>
          <Preview text={fill(picked, reply.values)} />
        </div>
      </div>
    </div>
  );
}

/*
 * Preview shows a filled template the way WhatsApp draws it: its header
 * bold, its footer quiet, each part a paragraph.
 */
function Preview({ text }: { text: string }) {
  return (
    <div className="flex flex-col gap-2 text-body-2-regular text-text-primary">
      {text.split("\n\n").map((part, i) => {
        const bold = /^\*\*(.+)\*\*$/s.exec(part);
        const quiet = /^_(.+)_$/s.exec(part);
        const key = `${i}-${part.slice(0, 12)}`;

        if (bold) {
          return (
            <p key={key} className="text-body-2-medium">
              {bold[1]}
            </p>
          );
        }

        if (quiet) {
          return (
            <p key={key} className="text-text-tertiary">
              {quiet[1]}
            </p>
          );
        }

        return (
          <p key={key} className="whitespace-pre-wrap">
            {part}
          </p>
        );
      })}
    </div>
  );
}

/*
 * fill shows a template as the customer will read it, a variable not yet
 * given left as written.
 */
function fill(t: WhatsAppTemplate, values: Record<string, string>): string {
  return t.preview.replace(/\{\{\s*([A-Za-z0-9_]+)\s*\}\}/g, (match, key: string, offset: number) => {
    const header = t.preview.indexOf("\n\n");
    const part = t.variables.some((v) => v.part === "header") && header > 0 && offset < header ? "header" : "body";
    return values[`${part}.${key}`]?.trim() || match;
  });
}
