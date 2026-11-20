/*
 * A field that searches as you type and offers what it finds in a
 * dropdown under it: skeleton rows while the search runs, the matches
 * to pick with the pointer or the arrow keys, and a line saying so when
 * nothing matches. Picking writes the pick's text into the field; typing
 * again lets go of the pick and searches anew. The caller runs the
 * search from the words it is told about and hands back what it found.
 */
import { RiSearchLine } from "@remixicon/react";
import { type ReactNode, useState } from "react";

import { ComboBox, ComboBoxItem } from "@/components/base/combobox/combobox";
import { Skeleton } from "@/components/base/skeleton/skeleton";

/*
 * minWords is how many characters a search needs before it runs.
 */
export const minWords = 2;

function Searching() {
  return (
    <div role="status" aria-label="Searching" className="flex flex-col gap-1 py-1">
      {[0, 1, 2].map((row) => (
        <div key={row} className="flex items-center gap-3 px-2 py-2">
          <Skeleton shape="circle" className="size-3.5" />
          <Skeleton shape="text" className="h-3.5 w-24" />
          <Skeleton shape="text" className="h-3.5 flex-1" />
        </div>
      ))}
    </div>
  );
}

export interface SearchComboBoxProps<T> {
  label: string;
  placeholder?: string;
  autoFocus?: boolean;
  /* What the search found for the words last told about. */
  items: T[];
  /* Whether that search is still running. */
  searching: boolean;
  keyOf: (item: T) => string;
  /* What a pick reads as in the field. */
  textOf: (item: T) => string;
  /* Told the words to search for, trimmed, or "" when there are too few. */
  onSearch: (words: string) => void;
  /* Told the pick, or null once the field lets go of it. */
  onPick: (item: T | null) => void;
  /* Told everything typed, for a caller that also takes typed values. */
  onType?: (text: string) => void;
  empty?: string;
  idle?: string;
  children: (item: T) => ReactNode;
}

export function SearchComboBox<T extends object>({
  label,
  placeholder,
  autoFocus,
  items,
  searching,
  keyOf,
  textOf,
  onSearch,
  onPick,
  onType,
  empty = "Nothing matches.",
  idle = "Type to search.",
  children,
}: SearchComboBoxProps<T>) {
  const [text, setText] = useState("");
  const [chosen, setChosen] = useState("");
  const words = text.trim();
  const live = words.length >= minWords && words !== chosen;

  return (
    <ComboBox
      label={label}
      placeholder={placeholder}
      leadingIcon={RiSearchLine}
      autoFocus={autoFocus}
      menuTrigger="input"
      allowsCustomValue
      inputValue={text}
      items={live && !searching ? items : []}
      emptyMessage={live && searching ? <Searching /> : live ? empty : idle}
      onInputChange={(value) => {
        const next = value.trim();
        setText(value);
        onType?.(value);
        if (next !== chosen) {
          if (chosen !== "") {
            setChosen("");
            onPick(null);
          }
          onSearch(next.length >= minWords ? next : "");
        }
      }}
      onSelectionChange={(key) => {
        const item = items.find((i) => keyOf(i) === key);
        if (item) {
          const shown = textOf(item);
          setChosen(shown.trim());
          setText(shown);
          onSearch("");
          onPick(item);
        }
      }}
    >
      {(item: T) => (
        <ComboBoxItem id={keyOf(item)} textValue={textOf(item)}>
          {children(item)}
        </ComboBoxItem>
      )}
    </ComboBox>
  );
}
