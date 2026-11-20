import {
  type RemixiconComponentType,
  RiArrowDownSLine,
  RiArrowRightUpLine,
  RiCheckLine,
  RiClaudeFill,
  RiFileCopyLine,
  RiMarkdownLine,
  RiOpenaiFill,
  RiPerplexityFill,
} from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import {
  Dropdown,
  DropdownDivider,
  DropdownItem,
  DropdownPopover,
  DropdownTrigger,
} from "@/components/base/dropdown/dropdown";
import { cx } from "@/utils/cx";

/*
 * The page's actions, beside its title: copy the page as Markdown, and,
 * behind the chevron, view that Markdown or open it in an assistant.
 * The Markdown lives at the page's path plus ".md" (src/pages).
 */

type Action = { label: string; detail: string; icon: RemixiconComponentType; run: () => void; external?: boolean };

// Resolves true once the page's Markdown is on the clipboard.
async function copyMarkdown(url: string): Promise<boolean> {
  // A promised blob keeps Safari's clipboard permission across the fetch.
  const text = fetch(url).then((response) => response.text());
  try {
    const blob = text.then((value) => new Blob([value], { type: "text/plain" }));
    await navigator.clipboard.write([new ClipboardItem({ "text/plain": blob })]);
    return true;
  } catch {
    try {
      await navigator.clipboard.writeText(await text);
      return true;
    } catch {
      return false;
    }
  }
}

export function PageActions({ markdownUrl, site }: { markdownUrl: string; site: string }) {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);

  const copy = () => {
    void copyMarkdown(markdownUrl).then((done) => {
      if (!done) {
        // No clipboard access: show the Markdown to copy by hand.
        window.open(markdownUrl, "_blank", "noopener");
        return;
      }
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    });
  };

  const ask = (base: string) => () => {
    const prompt = `Read ${site || location.origin}${markdownUrl} so I can ask questions about it.`;
    window.open(`${base}${encodeURIComponent(prompt)}`, "_blank", "noopener");
  };

  const actions: Action[] = [
    { label: "Copy page", detail: "Copy this page as Markdown for LLMs", icon: RiFileCopyLine, run: copy },
    {
      label: "View as Markdown",
      detail: "Open this page as plain text",
      icon: RiMarkdownLine,
      run: () => window.open(markdownUrl, "_blank", "noopener"),
      external: true,
    },
    {
      label: "Open in Claude",
      detail: "Ask questions about this page",
      icon: RiClaudeFill,
      run: ask("https://claude.ai/new?q="),
      external: true,
    },
    {
      label: "Open in ChatGPT",
      detail: "Ask questions about this page",
      icon: RiOpenaiFill,
      run: ask("https://chatgpt.com/?hints=search&q="),
      external: true,
    },
    {
      label: "Open in Perplexity",
      detail: "Ask questions about this page",
      icon: RiPerplexityFill,
      run: ask("https://www.perplexity.ai/search/new?q="),
      external: true,
    },
  ];

  return (
    <div className="flex items-stretch">
      <Button
        variant="secondary"
        size="small"
        leadingIcon={copied ? RiCheckLine : RiFileCopyLine}
        onClick={copy}
        className="rounded-e-none"
      >
        {copied ? "Copied" : "Copy page"}
      </Button>
      <Dropdown isOpen={open} onOpenChange={setOpen}>
        <DropdownTrigger
          aria-label="More page actions"
          className={cx(
            "-ms-px flex h-8 items-center rounded-e-2lg border border-border-button-default bg-background-primary-default px-1.5 shadow-xs",
            "text-foreground-icon-secondary transition-colors hover:border-border-button-hover hover:text-foreground-icon-primary",
          )}
        >
          <RiArrowDownSLine
            aria-hidden
            className={cx(
              "size-4.5 transition-[rotate] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none",
              open && "rotate-180",
            )}
          />
        </DropdownTrigger>
        <DropdownPopover aria-label="Page actions" placement="bottom end" className="w-72">
          {actions.map((action, index) => (
            <div key={action.label} className="contents">
              {index === 2 ? <DropdownDivider /> : null}
              <DropdownItem
                onSelect={() => {
                  setOpen(false);
                  action.run();
                }}
                className="items-start gap-3"
              >
                <span className="mt-0.5 grid size-7 shrink-0 place-items-center rounded-lg border border-separator-border">
                  <action.icon className="size-4 text-foreground-icon-secondary" aria-hidden />
                </span>
                <span className="flex min-w-0 flex-1 flex-col text-start">
                  <span className="text-body-medium text-text-primary">{action.label}</span>
                  <span className="text-caption-1-regular text-text-tertiary">{action.detail}</span>
                </span>
                {action.external ? (
                  <RiArrowRightUpLine className="mt-1 size-4 shrink-0 text-foreground-icon-tertiary" aria-hidden />
                ) : null}
              </DropdownItem>
            </div>
          ))}
        </DropdownPopover>
      </Dropdown>
    </div>
  );
}
