/*
 * A prompt's Markdown as the writer gives it back: the article writer's
 * Markdown with the underscores inside words left as typed, since a
 * model reads {{window_start}} and list_articles, not their escaped
 * forms.
 */
import { articleMarkdown } from "@/features/help-center/writer/article-markdown";

export function promptMarkdown(markdown: string): string {
  return articleMarkdown(markdown).replace(/(?<=[\w{])\\_(?=[\w}])/g, "_");
}
