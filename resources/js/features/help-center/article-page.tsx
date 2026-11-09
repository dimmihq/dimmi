/*
 * The article as it will read, laid out like the help center's article
 * page: its collection in the brand color, the title, the description as
 * a subtitle, a rule, then the body in the article writer. Enter in the
 * title moves to the description, and from there into the body.
 */
import { type CSSProperties, type KeyboardEvent, lazy, Suspense, useRef } from "react";
import { TextArea, TextField } from "react-aria-components";

import { Notification } from "@/components/base/notification/notification";
import { EditorSkeleton } from "@/features/shell/table-skeleton";
import { cx } from "@/utils/cx";

import type { Draft } from "./docs-format";

const ArticleWriter = lazy(() => import("./writer/article-writer"));

/*
 * bareField is a field with no box, so the title and subtitle read as
 * the page's own text; it grows with what is typed.
 */
const bareField =
  "w-full resize-none border-0 bg-transparent p-0 outline-none [field-sizing:content] placeholder:text-text-placeholder";

/*
 * nextOnEnter moves on instead of breaking the line.
 */
function nextOnEnter(event: KeyboardEvent, next: () => void) {
  if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
    event.preventDefault();
    next();
  }
}

/*
 * focusIn puts the caret in the first element of the page that matches.
 */
function focusIn(root: HTMLElement | null, selector: string) {
  root?.querySelector<HTMLElement>(selector)?.focus();
}

export function ArticlePage({
  workspace,
  draft,
  brand,
  collection,
  revision,
  failure,
  onChange,
  onPickCollection,
}: {
  workspace: string;
  draft: Draft;
  brand: string | undefined;
  collection: string;
  revision: number;
  failure: Error | null;
  onChange: (change: Partial<Draft>) => void;
  onPickCollection: () => void;
}) {
  const page = useRef<HTMLElement | null>(null);

  return (
    <article
      ref={page}
      className="article-brand mx-auto flex w-full max-w-[42.5rem] flex-col px-6 pt-12 md:pt-16"
      style={brand ? ({ "--brand": brand } as CSSProperties) : undefined}
    >
      {failure ? (
        <Notification status="error" title={failure.message} dismissible={false} role="alert" className="mb-6 w-full" />
      ) : null}
      <button
        type="button"
        onClick={onPickCollection}
        className="self-start rounded-md text-body-2-medium text-[var(--link)] outline-none hover:underline focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        {collection || "Choose a collection"}
      </button>
      <TextField aria-label="Title" value={draft.title} onChange={(title) => onChange({ title })} maxLength={150}>
        <TextArea
          rows={1}
          placeholder="Untitled article"
          onKeyDown={(event) => nextOnEnter(event, () => focusIn(page.current, "textarea[data-description]"))}
          className={cx(bareField, "mt-2 text-display-4-medium tracking-tight text-balance text-text-primary")}
        />
      </TextField>
      <TextField
        aria-label="Description"
        value={draft.description}
        onChange={(description) => onChange({ description })}
        maxLength={300}
      >
        <TextArea
          rows={1}
          data-description=""
          placeholder="Add a short description"
          onKeyDown={(event) => nextOnEnter(event, () => focusIn(page.current, ".ProseMirror"))}
          className={cx(bareField, "mt-3 text-title-3-regular text-text-secondary")}
        />
      </TextField>
      <div className="mt-6 mb-8 border-b border-separator-border" />
      <Suspense fallback={<EditorSkeleton />}>
        <ArticleWriter
          key={revision}
          workspace={workspace}
          value={draft.body}
          label="Article"
          brand={brand}
          onChange={(body) => onChange({ body })}
        />
      </Suspense>
    </article>
  );
}
