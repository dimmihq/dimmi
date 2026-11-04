/*
 * A message's text as the thread shows it. Teammates write replies and
 * notes in Markdown, and email arrives converted to it, so the thread
 * renders it: emphasis, lists, quotes, code and links, with single line
 * breaks kept the way the writer typed them. Raw HTML in a message is
 * dropped, never rendered, and links open in a new tab without handing
 * the thread to the page they open.
 */
import type { ComponentProps } from "react";
import Markdown, { type Components } from "react-markdown";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";

import { cx } from "@/utils/cx";

const plugins = [remarkGfm, remarkBreaks];

function Link({ href, children }: ComponentProps<"a">) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="underline outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      {children}
    </a>
  );
}

const components: Components = {
  a: Link,
  p: ({ children }) => <p>{children}</p>,
  ul: ({ children }) => <ul className="list-disc ps-5">{children}</ul>,
  ol: ({ children }) => <ol className="list-decimal ps-5">{children}</ol>,
  blockquote: ({ children }) => (
    <blockquote className="border-s-2 border-separator-border ps-3 text-text-secondary">{children}</blockquote>
  ),
  code: ({ children }) => (
    <code className="rounded-md bg-background-secondary-default px-1 font-mono text-body-2-regular">{children}</code>
  ),
  pre: ({ children }) => (
    <pre className="overflow-x-auto rounded-xl bg-background-secondary-default p-2 [&_code]:bg-transparent [&_code]:p-0">
      {children}
    </pre>
  ),
  h1: ({ children }) => <p className="text-body-medium">{children}</p>,
  h2: ({ children }) => <p className="text-body-medium">{children}</p>,
  h3: ({ children }) => <p className="text-body-medium">{children}</p>,
};

export function MessageBody({ text, className }: { text: string; className?: string }) {
  return (
    <div className={cx("flex flex-col gap-2 wrap-break-word", className)}>
      <Markdown remarkPlugins={plugins} components={components} skipHtml>
        {text}
      </Markdown>
    </div>
  );
}
