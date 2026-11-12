/*
 * The composer's add menu, on BoardUI's composer add menu: the round plus
 * turns into a close, and a panel rises above it with what a teammate can
 * bring into the message. Each source opens a searchable list in the
 * same panel: the team's saved replies, the help center's articles, feature requests and changelog
 * entries; typing narrows the list, Enter takes the first match and the
 * down arrow moves into the list. Emoji and, when the workspace allows
 * it, GIFs from GIPHY have pickers of their own. A pick drops in a line
 * naming it with its link, so it reaches the customer with the message;
 * a saved reply drops in its text and hands its macro to the composer.
 * Files are not offered until replies can carry them. Moving between the menu
 * and a list slides the view the way the person is heading while the
 * panel eases to its new height, on BoardUI's timings. The panel always
 * reopens on the menu.
 */
import {
  RiAddLine,
  RiArrowLeftSLine,
  RiArticleLine,
  RiChatQuoteLine,
  RiEmotionLine,
  RiFileGifLine,
  RiHistoryLine,
  RiLightbulbLine,
  RiSearchLine,
} from "@remixicon/react";
import { type ComponentType, type ReactNode, useDeferredValue, useRef, useState } from "react";
import { Button as AriaButton, Dialog, DialogTrigger, ListBox, ListBoxItem, Popover } from "react-aria-components";

import { CHIP_COLORS, type ChipColor } from "@/components/base/badges/chip";
import { MENU_POPOVER_SURFACE } from "@/components/base/dropdown/menu-styles";
import { Input } from "@/components/base/input/input";
import type { SavedReply } from "@/lib/api/saved-replies";
import { cx } from "@/utils/cx";

import { EmojiPicker } from "./composer/emoji-picker";
import { GifPicker, type PickedGif } from "./composer/gif-picker";
import { useInbox } from "./conversations";
import { type LibraryItem, type SourceKind, useLibrary } from "./library";
import { Resize, Swap } from "./motion";

interface Source {
  kind: SourceKind;
  label: string;
  description: string;
  icon: ComponentType<{ className?: string }>;
  noun: string;
  tint?: ChipColor;
}

const basics: Source[] = [
  {
    kind: "saved",
    label: "Saved reply",
    description: "Insert one of your team's answers",
    icon: RiChatQuoteLine,
    noun: "saved replies",
  },
  {
    kind: "article",
    label: "Help article",
    description: "Link an article from your help center",
    icon: RiArticleLine,
    noun: "articles",
  },
];

const workspace: Source[] = [
  {
    kind: "request",
    label: "Feature request",
    description: "Link what customers asked for",
    icon: RiLightbulbLine,
    noun: "feature requests",
    tint: "yellow",
  },
  {
    kind: "changelog",
    label: "Changelog",
    description: "Share what shipped",
    icon: RiHistoryLine,
    noun: "changelog entries",
    tint: "lime",
  },
];

const row =
  "flex w-full cursor-pointer items-center gap-2 rounded-2lg px-2 py-1.5 text-start outline-none transition-colors hover:bg-background-primary-hover focus-visible:bg-background-primary-hover";

function Mark({ source }: { source: Source }) {
  const Icon = source.icon;
  if (!source.tint) {
    return <Icon aria-hidden className="size-5 shrink-0 text-foreground-icon-secondary" />;
  }

  return (
    <span
      aria-hidden
      className={cx("flex size-6 shrink-0 items-center justify-center rounded-md", CHIP_COLORS[source.tint])}
    >
      <Icon className="size-4" />
    </span>
  );
}

function Row({ source, onPress }: { source: Source; onPress: () => void }) {
  return (
    <button type="button" onClick={onPress} className={row}>
      <Mark source={source} />
      <span className="truncate text-body-medium whitespace-nowrap">
        <span className="text-text-primary">{source.label}</span>
        <span className="ms-1.5 text-text-secondary">{source.description}</span>
      </span>
    </button>
  );
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex w-full flex-col gap-1.5 pt-1">
      <span className="ps-2 text-body-medium text-text-secondary">{title}</span>
      <div className="flex w-full flex-col gap-1">{children}</div>
    </div>
  );
}

function Back({ label, onBack }: { label: string; onBack: () => void }) {
  return (
    <div className="flex items-center gap-1">
      <button
        type="button"
        aria-label="Back"
        onClick={onBack}
        className="flex size-7 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors hover:bg-background-primary-hover focus-visible:bg-background-primary-hover"
      >
        <RiArrowLeftSLine aria-hidden className="size-5" />
      </button>
      <span className="text-body-medium text-text-primary">{label}</span>
    </div>
  );
}

function Picker({
  source,
  onBack,
  onPick,
}: {
  source: Source;
  onBack: () => void;
  onPick: (item: LibraryItem) => void;
}) {
  const [query, setQuery] = useState("");
  const results = useRef<HTMLDivElement>(null);
  const { workspace: current } = useInbox();
  const library = useLibrary(current, source.kind, useDeferredValue(query));
  const found = library.items;
  const nothing = source.kind === "saved" && query.trim() === "" ? "No saved replies yet" : "Nothing matches";
  const empty = library.loading ? "Loading…" : library.missing ? "Set up a help center first" : nothing;

  return (
    <div className="flex flex-col gap-2">
      <Back label={source.label} onBack={onBack} />
      <Input
        aria-label={`Search ${source.noun}`}
        placeholder={`Search ${source.noun}`}
        leadingIcon={RiSearchLine}
        fieldClassName="bg-background-secondary-default"
        value={query}
        onChange={setQuery}
        onKeyDown={(event) => {
          const first = found[0];
          if (event.key === "Enter" && first) {
            onPick(first);
          }
          if (event.key === "ArrowDown") {
            event.preventDefault();
            results.current?.querySelector<HTMLElement>("[role=option]")?.focus();
          }
        }}
        autoFocus
      />
      <ListBox
        ref={results}
        aria-label={source.label}
        items={found}
        onAction={(id) => {
          const item = found.find((candidate) => candidate.id === id);
          if (item) {
            onPick(item);
          }
        }}
        renderEmptyState={() => <p className="px-2 py-3 text-body-regular text-text-tertiary">{empty}</p>}
        className="flex max-h-64 flex-col gap-1 overflow-y-auto outline-none"
      >
        {(item) => (
          <ListBoxItem
            id={item.id}
            textValue={item.title}
            className={cx(row, "data-[focused]:bg-background-primary-hover")}
          >
            <Mark source={source} />
            <span className="flex min-w-0 flex-col">
              <span className="truncate text-body-medium text-text-primary">{item.title}</span>
              <span className="truncate text-body-2-regular text-text-secondary">{item.detail}</span>
            </span>
          </ListBoxItem>
        )}
      </ListBox>
    </div>
  );
}

/*
 * Gifs is the GIF search the add menu may offer: GIPHY's key and the
 * teammate searching, or null when the workspace has none.
 */
export interface Gifs {
  apiKey: string;
  member: string;
  onPick: (picked: PickedGif) => void;
}

type Panel = Source | "emoji" | "gif";

const emojiRow: Source = {
  kind: "article",
  label: "Emoji",
  description: "Add an emoji",
  icon: RiEmotionLine,
  noun: "emoji",
};

const gifRow: Source = {
  kind: "article",
  label: "GIF",
  description: "Search GIPHY",
  icon: RiFileGifLine,
  noun: "GIFs",
};

function panelKey(panel: Panel | null): string {
  if (panel === null) {
    return "menu";
  }

  return typeof panel === "string" ? panel : panel.kind;
}

export function AddMenu({
  onInsert,
  onEmoji,
  onSavedReply,
  gifs,
}: {
  onInsert: (text: string) => void;
  onEmoji: (emoji: string) => void;
  onSavedReply: (reply: SavedReply) => void;
  gifs: Gifs | null;
}) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<Panel | null>(null);

  const toggle = (next: boolean) => {
    setOpen(next);
    if (!next) {
      setSource(null);
    }
  };
  const pick = (item: LibraryItem) => {
    if (item.reply) {
      onSavedReply(item.reply);
      toggle(false);
      return;
    }
    onInsert(item.url ? `${item.title}: ${item.url}` : item.title);
    toggle(false);
  };

  return (
    <DialogTrigger isOpen={open} onOpenChange={toggle}>
      <AriaButton
        aria-label="Add to message"
        className="flex size-9 shrink-0 cursor-pointer items-center justify-center rounded-full bg-composer-panel-add-background p-2 text-foreground-icon-primary outline-none transition-colors duration-150 ease hover:bg-composer-panel-add-hover-background focus-visible:ring-2 focus-visible:ring-border-focus-ring"
      >
        <RiAddLine
          aria-hidden
          className={cx(
            "size-5 transition-[rotate] duration-200 ease-out motion-reduce:transition-none",
            open && "rotate-45",
          )}
        />
      </AriaButton>
      <Popover placement="top start" offset={8} className={cx(MENU_POPOVER_SURFACE, "w-[361px] p-2")}>
        <Dialog aria-label="Add to message" className="outline-none">
          <Resize>
            <Swap view={panelKey(source)} forward={source !== null}>
              <div className="flex flex-col gap-2">
                {source === "emoji" ? (
                  <>
                    <Back label="Emoji" onBack={() => setSource(null)} />
                    <EmojiPicker
                      onPick={(emoji) => {
                        onEmoji(emoji);
                        toggle(false);
                      }}
                    />
                  </>
                ) : source === "gif" && gifs ? (
                  <>
                    <Back label="GIF" onBack={() => setSource(null)} />
                    <GifPicker
                      apiKey={gifs.apiKey}
                      member={gifs.member}
                      onPick={(picked) => {
                        gifs.onPick(picked);
                        toggle(false);
                      }}
                    />
                  </>
                ) : source && typeof source !== "string" ? (
                  <Picker source={source} onBack={() => setSource(null)} onPick={pick} />
                ) : (
                  <>
                    <Section title="Add">
                      {basics.map((item) => (
                        <Row key={item.kind} source={item} onPress={() => setSource(item)} />
                      ))}
                      <Row source={emojiRow} onPress={() => setSource("emoji")} />
                      {gifs ? <Row source={gifRow} onPress={() => setSource("gif")} /> : null}
                    </Section>
                    <Section title="From your workspace">
                      {workspace.map((item) => (
                        <Row key={item.kind} source={item} onPress={() => setSource(item)} />
                      ))}
                    </Section>
                  </>
                )}
              </div>
            </Swap>
          </Resize>
        </Dialog>
      </Popover>
    </DialogTrigger>
  );
}
