/*
 * A link in a customer's message, drawn so a teammate sees where it goes
 * before they open it: hovering or focusing shows the whole address with
 * its host set apart. A link the checks flag asks first, naming the host
 * and the reason. A blocked one cannot be opened at all. When the
 * workspace hides email links, they read as plain text until someone
 * allowed to see raw mail shows them. Opening never hands the thread to
 * the page. Any other link asks first too, with a choice to trust its
 * domain so the teammate is never asked about it again.
 */
import { RiErrorWarningLine, RiForbidLine } from "@remixicon/react";
import { type ComponentProps, isValidElement, type ReactNode, useContext, useState } from "react";
import { Link as AriaLink } from "react-aria-components";
import { Checkbox } from "@/components/base/checkbox/checkbox";
import { AlertDialog } from "@/components/base/modal/modal";
import { Tooltip, TooltipTrigger } from "@/components/base/tooltip/tooltip";

import { check, LinkPolicyContext, reasonText } from "./link-checks";
import { isTrusted, trustDomain, useTrustedDomains } from "./trusted-domains";

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

/*
 * Flagged is a link that asks before it opens. Its reasons say why it
 * looks risky; one with none is only unknown, and offers to trust its
 * domain in the workspace.
 */
function Flagged({
  href,
  host,
  reasons,
  workspace,
  children,
}: {
  href: string;
  host: string;
  reasons: string[];
  workspace: string | null;
  children: ReactNode;
}) {
  const [asking, setAsking] = useState(false);
  const [always, setAlways] = useState(false);

  return (
    <>
      <TooltipTrigger>
        <AriaLink className={linkStyle} onPress={() => setAsking(true)}>
          {children}
          {reasons.length > 0 ? (
            <RiErrorWarningLine aria-label="Check before opening" className="ms-0.5 inline size-3.5 align-text-top" />
          ) : null}
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
        onConfirm={() => {
          if (always && workspace !== null) {
            trustDomain(workspace, host);
          }
          open(href);
        }}
      >
        <div className="flex flex-col gap-3">
          <p className="text-body-2-regular text-text-secondary" dir="ltr">
            <Address href={href} host={host} />
          </p>
          {reasons.length > 0 ? (
            <ul className="flex list-disc flex-col gap-1 ps-5 text-body-2-regular text-text-secondary">
              {reasons.map((reason) => (
                <li key={reason}>{reason}</li>
              ))}
            </ul>
          ) : (
            <p className="text-body-2-regular text-text-secondary">
              Be careful: only open links you expect, from people you know.
            </p>
          )}
          {workspace !== null ? (
            <Checkbox size="sm" isSelected={always} onChange={setAlways}>
              Always allow links from {host}
            </Checkbox>
          ) : null}
        </div>
      </AlertDialog>
    </>
  );
}

/*
 * SafeLink replaces the plain link of a customer's message.
 */
export function SafeLink({ href, children }: ComponentProps<"a">) {
  const policy = useContext(LinkPolicyContext);
  const trusted = useTrustedDomains(policy?.workspace ?? "");
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
      <Flagged href={href} host={verdict.host} reasons={verdict.reasons.map(reasonText)} workspace={null}>
        {children}
      </Flagged>
    );
  }
  if (verdict.kind === "ask" && !isTrusted(trusted, verdict.host)) {
    return (
      <Flagged href={href} host={verdict.host} reasons={[]} workspace={policy?.workspace ?? null}>
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
