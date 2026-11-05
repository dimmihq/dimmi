/*
 * A link in a customer's message, drawn so a teammate sees where it goes
 * before they open it: hovering or focusing shows the whole address with
 * its host set apart. A link the checks flag asks first, naming the host
 * and the reason. A blocked one cannot be opened at all. When the
 * workspace hides email links, they read as plain text until someone
 * allowed to see raw mail shows them. Opening never hands the thread to
 * the page.
 */
import { RiErrorWarningLine, RiForbidLine } from "@remixicon/react";
import { type ComponentProps, isValidElement, type ReactNode, useContext, useState } from "react";
import { Link as AriaLink } from "react-aria-components";

import { AlertDialog } from "@/components/base/modal/modal";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";

import { check, LinkPolicyContext, reasonText } from "./link-checks";

const linkStyle =
  "cursor-pointer rounded-sm underline outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring";

function textOf(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") {
    return String(node);
  }
  if (Array.isArray(node)) {
    return node.map(textOf).join("");
  }
  if (isValidElement<{ children?: ReactNode }>(node)) {
    return textOf(node.props.children);
  }

  return "";
}

function open(href: string) {
  window.open(href, "_blank", "noopener,noreferrer");
}

function Address({ href, host }: { href: string; host: string }) {
  const at = href.toLowerCase().indexOf(host);
  if (at < 0) {
    return <span dir="ltr">{href}</span>;
  }

  return (
    <span dir="ltr" className="break-all">
      <span className="text-text-secondary">{href.slice(0, at)}</span>
      <span className="text-text-primary">{href.slice(at, at + host.length)}</span>
      <span className="text-text-secondary">{href.slice(at + host.length)}</span>
    </span>
  );
}

function Flagged({
  href,
  host,
  reasons,
  children,
}: {
  href: string;
  host: string;
  reasons: string[];
  children: ReactNode;
}) {
  const [asking, setAsking] = useState(false);

  return (
    <>
      <TooltipTrigger>
        <AriaLink className={linkStyle} onPress={() => setAsking(true)}>
          {children}
          <RiErrorWarningLine aria-label="Check before opening" className="ms-0.5 inline size-3.5 align-text-top" />
        </AriaLink>
        <Tooltip>
          <Address href={href} host={host} />
        </Tooltip>
      </TooltipTrigger>
      <AlertDialog
        isOpen={asking}
        onOpenChange={setAsking}
        variant="default"
        title="Open this link?"
        description={`Opens ${host}. Continue?`}
        confirmLabel="Open link"
        onConfirm={() => open(href)}
      >
        <ul className="flex list-disc flex-col gap-1 ps-5 text-body-2-regular text-text-secondary">
          {reasons.map((reason) => (
            <li key={reason}>{reason}</li>
          ))}
        </ul>
      </AlertDialog>
    </>
  );
}

/*
 * SafeLink replaces the plain link of a customer's message.
 */
export function SafeLink({ href, children }: ComponentProps<"a">) {
  const policy = useContext(LinkPolicyContext);
  const verdict = href && policy ? check(href, textOf(children), policy.settings) : null;
  if (policy?.plain) {
    return <span>{children}</span>;
  }
  if (!(href && verdict)) {
    return (
      <a href={href} target="_blank" rel="noopener noreferrer" className={linkStyle}>
        {children}
      </a>
    );
  }
  if (verdict.kind === "blocked") {
    return (
      <span className="text-text-tertiary line-through" title={`Blocked: ${verdict.host}`}>
        {children}
        <RiForbidLine
          aria-label={`Blocked link to ${verdict.host}`}
          className="ms-0.5 inline size-3.5 align-text-top"
        />
      </span>
    );
  }
  if (verdict.kind === "warn") {
    return (
      <Flagged href={href} host={verdict.host} reasons={verdict.reasons.map(reasonText)}>
        {children}
      </Flagged>
    );
  }

  return (
    <TooltipTrigger>
      <AriaLink href={href} target="_blank" rel="noopener noreferrer" className={linkStyle}>
        {children}
      </AriaLink>
      <Tooltip>
        <Address href={href} host={verdict.host} />
      </Tooltip>
    </TooltipTrigger>
  );
}
