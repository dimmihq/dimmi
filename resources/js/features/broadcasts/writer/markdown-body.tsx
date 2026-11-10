/*
 * A message's Markdown as the person it goes to reads it, with every
 * variable filled in for the sample person, so the preview shows "Hi
 * Ada," rather than a placeholder. Raw HTML is never rendered.
 */
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { cx } from "@/utils/cx";

import { filled } from "./variables";

export function MarkdownBody({ markdown, className }: { markdown: string; className?: string }) {
  if (markdown.trim() === "") {
    return <p className="text-body-regular text-text-placeholder">Nothing written yet.</p>;
  }

  return (
    <div className={cx("article-brand prose", className)}>
      <Markdown remarkPlugins={[remarkGfm]} skipHtml>
        {filled(markdown)}
      </Markdown>
    </div>
  );
}
