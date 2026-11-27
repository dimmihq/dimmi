/*
 * The composer's writing field. The formatting editor loads on its own;
 * until it arrives, and should it fail to, a plain field takes the same
 * Markdown, so the person can start typing at once. The editor never
 * takes the place of a field mid-typing: keys pressed while the fields
 * swap would land nowhere. In a field nobody is in, it arrives at once;
 * in the field being written in, after a pause in the typing, and it
 * takes the focus over so writing carries on.
 */
import { type ComponentType, useCallback, useEffect, useRef, useState } from "react";

import { loadEditor } from "./load-editor";
import type { RichEditorProps } from "./rich-editor";

type Editor = ComponentType<RichEditorProps>;

/* How long typing must pause before the editor takes a focused field's place. */
const pause = 600;

let loaded: Editor | undefined;

/** The formatting editor, or the plain field should it fail to load. */
function load(): Promise<Editor> {
  return loadEditor().then(
    (module) => {
      loaded = module.default;
      return loaded;
    },
    () => {
      loaded = PlainField;
      return loaded;
    },
  );
}

/*
 * How the editor arrived: before the field showed, so it focuses as asked;
 * in a field nobody was in, so it leaves the focus where it is; or in the
 * field being written in, so it takes the focus.
 */
type Arrival = "first" | "aside" | "inside";

export function ComposerField(props: RichEditorProps) {
  const [editor, setEditor] = useState<{ component: Editor; arrival: Arrival } | undefined>(() =>
    loaded ? { component: loaded, arrival: "first" } : undefined,
  );
  const focused = useRef(false);
  const typedAt = useRef(0);

  useEffect(() => {
    if (editor) {
      return;
    }
    let live = true;
    let timer: number | undefined;
    const arrive = (component: Editor) => {
      if (!live) {
        return;
      }
      if (!focused.current) {
        setEditor({ component, arrival: "aside" });
        return;
      }
      const quiet = performance.now() - typedAt.current;
      if (quiet >= pause) {
        setEditor({ component, arrival: "inside" });
      } else {
        timer = window.setTimeout(() => arrive(component), pause - quiet);
      }
    };
    void load().then(arrive);

    return () => {
      live = false;
      window.clearTimeout(timer);
    };
  }, [editor]);

  if (editor) {
    const Rich = editor.component;
    const autoFocus = { first: props.autoFocus, aside: false, inside: true }[editor.arrival];

    return <Rich key={props.label} {...props} autoFocus={autoFocus} />;
  }

  return (
    <PlainField
      {...props}
      onChange={(value) => {
        typedAt.current = performance.now();
        props.onChange(value);
      }}
      onFocus={() => {
        focused.current = true;
      }}
      onBlur={() => {
        focused.current = false;
        props.onBlur();
      }}
    />
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

function PlainField({
  value,
  label,
  placeholder,
  focusKey,
  autoFocus = true,
  onChange,
  onSubmit,
  onBlur,
  onFocus,
  onFiles,
}: RichEditorProps & { onFocus?: () => void }) {
  const arrive = useCallback(
    (element: HTMLTextAreaElement | null) => {
      if (element) {
        sizeToText(element);
        if (autoFocus) {
          element.focus();
        }
      }
    },
    [autoFocus],
  );

  return (
    <textarea
      key={focusKey}
      ref={arrive}
      value={value}
      onChange={(event) => onChange(event.target.value)}
      onFocus={onFocus}
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
