/*
 * The composer's writing field. The formatting editor loads on its own;
 * until it arrives, and should it fail to, a plain field takes the same
 * Markdown, so the person can start typing at once. A field opened
 * after the editor arrived uses it straight away; the choice is kept for
 * the field's life, so the editor never remounts under the person.
 */
import { lazy, Suspense, useCallback, useState } from "react";

import { loadEditor, loadedEditor } from "./load-editor";
import type { RichEditorProps } from "./rich-editor";

const RichEditor = lazy(() => loadEditor().catch(() => ({ default: PlainField })));

export function ComposerField(props: RichEditorProps) {
  const [Ready] = useState(loadedEditor);
  if (Ready) {
    return <Ready key={props.label} {...props} />;
  }

  return (
    <Suspense fallback={<PlainField {...props} />}>
      <RichEditor key={props.label} {...props} />
    </Suspense>
  );
}

/*
 * sizeToText sizes the field to its text, between its resting height and
 * a cap past which it scrolls.
 */
function sizeToText(field: HTMLTextAreaElement) {
  field.style.height = "auto";
  field.style.height = `${Math.min(field.scrollHeight, 280)}px`;
}

function PlainField({ value, label, placeholder, focusKey, onChange, onSubmit, onBlur, onFiles }: RichEditorProps) {
  const arrive = useCallback((element: HTMLTextAreaElement | null) => {
    if (element) {
      sizeToText(element);
      element.focus();
    }
  }, []);

  return (
    <textarea
      key={focusKey}
      ref={arrive}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      onBlur={onBlur}
      onInput={(event) => sizeToText(event.currentTarget)}
      onPaste={(event) => onFiles(event.clipboardData.files)}
      onKeyDown={(event) => {
        if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
          event.preventDefault();
          onSubmit(event.shiftKey ? "close" : event.altKey ? "snooze" : undefined);
        }
      }}
      rows={4}
      aria-label={label}
      placeholder={placeholder}
      className="block max-h-[280px] min-h-24 w-full resize-none bg-transparent p-0 text-body-regular text-text-primary caret-accent-500 outline-none placeholder:text-text-tertiary"
    />
  );
}
