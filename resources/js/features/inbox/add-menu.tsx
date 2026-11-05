/*
 * The composer's add menu, on BoardUI's composer add menu: the round plus
 * turns into a close, and a panel rises above it with what a teammate can
 * bring into the message. Each source opens a searchable list in the
 * same panel: help center
 * articles, saved replies, past conversations, issues, feature requests
 * and changelog entries; typing narrows the list, Enter takes the first
 * match and the down arrow moves into the list. A saved reply drops its
 * text into the draft, and anything else drops in a line naming it, so
 * whatever is picked reaches the customer with the message. Files are
 * not offered until replies can carry them. Moving between the menu
 * and a list slides the view the way the person is heading while the
 * panel eases to its new height, on BoardUI's timings. The panel always
 * reopens on the menu.
 */
import {
  RiAddLine,
  RiArrowLeftSLine,
  RiArticleLine,
  RiChat3Line,
  RiChatQuoteLine,
  RiGitPullRequestLine,
  RiHistoryLine,
  RiLightbulbLine,
  RiSearchLine,
} from "@remixicon/react";
import { type ComponentType, type ReactNode, useRef, useState } from "react";
import { Button as AriaButton, Dialog, DialogTrigger, ListBox, ListBoxItem, Popover } from "react-aria-components";

import { CHIP_COLORS, type ChipColor } from "@/components/base/badges/chip";
import { MENU_POPOVER_SURFACE } from "@/components/base/dropdown/menu-styles";
import { Input } from "@/components/base/input/input";
import { cx } from "@/utils/cx";

import { type LibraryItem, type SourceKind, search } from "./library";
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
    kind: "reply",
    label: "Saved reply",
    description: "Insert a reply your team saved",
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
    kind: "conversation",
    label: "Conversation",
    description: "Link a similar case",
    icon: RiChat3Line,
    noun: "conversations",
    tint: "blue",
  },
  {
    kind: "issue",
    label: "Issue",
    description: "Link a GitHub issue",
    icon: RiGitPullRequestLine,
    noun: "issues",
    tint: "purple",
  },
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
  const found = search(source.kind, query);

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-1">
        <button
          type="button"
          aria-label="Back"
          onClick={onBack}
          className="flex size-7 cursor-pointer items-center justify-center rounded-lg text-foreground-icon-secondary outline-none transition-colors hover:bg-background-primary-hover focus-visible:bg-background-primary-hover"
        >
          <RiArrowLeftSLine aria-hidden className="size-5" />
        </button>
        <span className="text-body-medium text-text-primary">{source.label}</span>
      </div>
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
        renderEmptyState={() => <p className="px-2 py-3 text-body-regular text-text-tertiary">Nothing matches</p>}
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

export function AddMenu({ onInsert }: { onInsert: (text: string) => void }) {
  const [open, setOpen] = useState(false);
  const [source, setSource] = useState<Source | null>(null);

  const toggle = (next: boolean) => {
    setOpen(next);
    if (!next) {
      setSource(null);
    }
  };
  const pick = (item: LibraryItem) => {
    onInsert(item.kind === "reply" ? (item.body ?? item.title) : `${item.title} (${item.detail})`);
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
            <Swap view={source?.kind ?? "menu"} forward={source !== null}>
              <div className="flex flex-col gap-2">
                {source ? (
                  <Picker source={source} onBack={() => setSource(null)} onPick={pick} />
                ) : (
                  <>
                    <Section title="Add">
                      {basics.map((item) => (
                        <Row key={item.kind} source={item} onPress={() => setSource(item)} />
                      ))}
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
