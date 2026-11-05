/*
 * A message's text as the thread shows it. Teammates write replies and
 * notes in Markdown, and email arrives converted to it, so the thread
 * renders it: emphasis, lists, quotes, code and links, with single line
 * breaks kept the way the writer typed them. Raw HTML in a message is
 * dropped, never rendered, and links open in a new tab without handing
 * the thread to the page they open. Code blocks are highlighted once the
 * highlighter, loaded only for a message that has one, arrives; an image
 * opens large on a click. A note's mentions read as @Name chips.
 */
import { type ComponentProps, lazy, Suspense, useEffect, useState } from "react";
import Markdown, { type Components, defaultUrlTransform, type Options } from "react-markdown";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";

import type { MentionRef } from "@/lib/api/inbox";
import { cx } from "@/utils/cx";

import { CodeBlock } from "./thread/code-block";
import { mentionScheme, withMentions } from "./thread/mention-text";

const ImageViewer = lazy(() => import("./thread/image-viewer"));

type Plugins = NonNullable<Options["rehypePlugins"]>;

const plugins = [remarkGfm, remarkBreaks];

function Link({ href, children }: ComponentProps<"a">) {
  if (href?.startsWith(mentionScheme)) {
    return (
      <span className="rounded-md bg-background-tertiary-default px-1 text-body-medium text-text-primary">
        {children}
      </span>
    );
  }

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

function Picture({ src, alt }: ComponentProps<"img">) {
  const [open, setOpen] = useState(false);
  if (typeof src !== "string" || src === "") {
    return alt ? <span>{alt}</span> : null;
  }

  return (
    <>
      <button
        type="button"
        aria-label={alt ? `Open image ${alt}` : "Open image"}
        onClick={() => setOpen(true)}
        className="block cursor-zoom-in rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <img src={src} alt={alt ?? ""} className="max-h-60 rounded-xl" />
      </button>
      {open ? (
        <Suspense fallback={null}>
          <ImageViewer src={src} alt={alt ?? ""} onClose={() => setOpen(false)} />
        </Suspense>
      ) : null}
    </>
  );
}

const components: Components = {
  a: Link,
  img: Picture,
  p: ({ children }) => <p>{children}</p>,
  ul: ({ children }) => <ul className="list-disc ps-5">{children}</ul>,
  ol: ({ children }) => <ol className="list-decimal ps-5">{children}</ol>,
  blockquote: ({ children }) => (
    <blockquote className="border-s-2 border-separator-border ps-3 text-text-secondary">{children}</blockquote>
  ),
  code: ({ children, className }) => (
    <code className={cx("rounded-md bg-background-secondary-default px-1 font-mono text-body-2-regular", className)}>
      {children}
    </code>
  ),
  pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
  h1: ({ children }) => <p className="text-body-medium">{children}</p>,
  h2: ({ children }) => <p className="text-body-medium">{children}</p>,
  h3: ({ children }) => <p className="text-body-medium">{children}</p>,
};

/*
 * useHighlighter loads the code highlighter when the text holds a code
 * block, and gives the plugins that apply it once it has loaded.
 */
function useHighlighter(text: string): Plugins {
  const fenced = text.includes("```");
  const [highlight, setHighlight] = useState<Plugins>([]);
  useEffect(() => {
    if (!fenced) {
      return;
    }
    let live = true;
    void import("rehype-highlight").then((loaded) => {
      if (live) {
        setHighlight([loaded.default]);
      }
    });

    return () => {
      live = false;
    };
  }, [fenced]);

  return highlight;
}

function keepMentions(url: string): string {
  return url.startsWith(mentionScheme) ? url : defaultUrlTransform(url);
}

export function MessageBody({
  text,
  className,
  mentions = [],
}: {
  text: string;
  className?: string;
  mentions?: MentionRef[];
}) {
  const highlight = useHighlighter(text);

  return (
    <div className={cx("flex flex-col gap-2 wrap-break-word", className)}>
      <Markdown
        remarkPlugins={plugins}
        rehypePlugins={highlight}
        components={components}
        urlTransform={keepMentions}
        skipHtml
      >
        {withMentions(text, mentions)}
      </Markdown>
    </div>
  );
}
