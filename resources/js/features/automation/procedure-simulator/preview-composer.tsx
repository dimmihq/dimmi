/*
 * The preview's composer: a card like the inbox reply composer's, not a
 * one-line pill. What was added to the preview, the customer to test as
 * or the conversation to replay, sits inside it above the text, so it
 * reads as part of the message being written. The text grows over
 * several lines; Enter sends and Shift+Enter starts a new line. The
 * plus and the send button sit on the row beneath.
 */
import { RiArrowUpLine, RiCloseLine, RiStopFill } from "@remixicon/react";
import { type FormEvent, type KeyboardEvent, type ReactNode, useEffect, useId, useRef } from "react";

import { ComposerLoader } from "@/components/application/composer-loader/composer-loader";
import { cx } from "@/utils/cx";

const surface = "bg-background-primary-default shadow-[0_1px_0.5px_rgba(0,0,0,0.02),0_4px_2px_rgba(0,0,0,0.02)]";

/* The tallest the text grows before it scrolls, in pixels. */
const tallest = 168;

/*
 * Added is one thing added to the preview, shown inside the composer as
 * a small card: a mark, what it is, a line about it, and a way to take
 * it off.
 */
export function Added({
  mark,
  title,
  detail,
  onRemove,
}: {
  mark: ReactNode;
  title: string;
  detail: string;
  onRemove: () => void;
}) {
  return (
    <span className="group/added flex max-w-full min-w-0 items-center gap-2.5 rounded-2xl border border-separator-border bg-background-secondary-default py-1.5 ps-1.5 pe-1.5">
      <span className="flex size-8 shrink-0 items-center justify-center">{mark}</span>
      <span className="flex min-w-0 flex-col pe-1">
        <span className="truncate text-body-2-medium text-text-primary">{title}</span>
        <span className="truncate text-body-2-regular text-text-secondary">{detail}</span>
      </span>
      <button
        type="button"
        aria-label={`Remove ${title}`}
        onClick={onRemove}
        className="flex size-6 shrink-0 cursor-pointer items-center justify-center rounded-full text-foreground-icon-tertiary outline-none transition-colors duration-150 hover:bg-background-secondary-hover hover:text-foreground-icon-primary focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiCloseLine aria-hidden className="size-4" />
      </button>
    </span>
  );
}

export function PreviewComposer({
  value,
  onValueChange,
  onSubmit,
  onStop,
  busy,
  placeholder,
  canSubmit,
  added,
  menu,
}: {
  value: string;
  onValueChange: (value: string) => void;
  onSubmit: () => void;
  onStop: () => void;
  busy: boolean;
  placeholder: string;
  /* Lets the message be sent with nothing typed, when a conversation is replayed. */
  canSubmit: boolean;
  /* What was added to the preview, or nothing. */
  added: ReactNode;
  menu: ReactNode;
}) {
  const field = useRef<HTMLTextAreaElement>(null);
  const id = useId();

  // biome-ignore lint/correctness/useExhaustiveDependencies: the field is measured again whenever its text changes.
  useEffect(() => {
    const el = field.current;
    if (el) {
      el.style.height = "auto";
      el.style.height = `${Math.min(el.scrollHeight, tallest)}px`;
    }
  }, [value]);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    onSubmit();
  };
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
      event.preventDefault();
      onSubmit();
    }
  };

  return (
    <ComposerLoader active={busy} radius={24}>
      <form
        onSubmit={submit}
        className={cx("flex w-full flex-col gap-2 rounded-3xl p-2", busy ? "bg-transparent" : surface)}
      >
        {added}
        <label className="sr-only" htmlFor={id}>
          {placeholder}
        </label>
        <textarea
          id={id}
          ref={field}
          rows={2}
          value={value}
          onChange={(event) => onValueChange(event.target.value)}
          onKeyDown={onKeyDown}
          placeholder={placeholder}
          className="min-h-12 w-full resize-none bg-transparent px-2 pt-1.5 text-body-regular text-text-primary caret-text-primary outline-none placeholder:text-text-tertiary"
        />
        <div className="flex items-center gap-2">
          {menu}
          <span className="ms-auto" />
          {busy ? (
            <button
              type="button"
              onClick={onStop}
              aria-label="Stop generating"
              className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-background-secondary-default text-foreground-icon-secondary outline-none transition-colors duration-150 hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
            >
              <RiStopFill aria-hidden className="size-5" />
            </button>
          ) : (
            <button
              type="submit"
              aria-label="Send message"
              disabled={value.trim().length === 0 && !canSubmit}
              className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-button-primary p-2 text-text-white outline-none transition-opacity duration-200 ease focus-visible:ring-2 focus-visible:ring-border-focus-ring disabled:cursor-not-allowed disabled:opacity-40"
            >
              <RiArrowUpLine aria-hidden className="size-5" />
            </button>
          )}
        </div>
      </form>
    </ComposerLoader>
  );
}
