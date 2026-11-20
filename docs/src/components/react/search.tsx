import { RiArrowRightSLine, RiFileTextLine, RiSearchLine } from "@remixicon/react";
import { useEffect, useRef, useState } from "react";

import { Input } from "@/components/base/input/input";
import { Kbd } from "@/components/base/kbd/kbd";
import { Modal, ModalBody } from "@/components/base/modal/modal";
import { Spinner } from "@/components/base/spinner/spinner";
import { cx } from "@/utils/cx";

import { BASE } from "../../base";

/*
 * Search over the Pagefind index Starlight builds into dist/pagefind.
 * The index exists only in a production build, so `astro dev` says so
 * instead of searching. Opens from the header button, Cmd/Ctrl+K or "/".
 */

type Result = { url: string; title: string; excerpt: string; sections: { url: string; title: string }[] };

type Pagefind = {
  options: (options: { baseUrl: string }) => Promise<void>;
  init: () => Promise<void>;
  debouncedSearch: (query: string) => Promise<{ results: { data: () => Promise<PagefindData> }[] } | null>;
};

type PagefindData = {
  url: string;
  excerpt: string;
  meta: { title?: string };
  sub_results?: { url: string; title: string }[];
};

let loading: Promise<Pagefind | null> | null = null;

function loadPagefind(): Promise<Pagefind | null> {
  // The index sits under the docs' /docs prefix, and so do its results.
  loading ??= import(/* @vite-ignore */ `${location.origin}${BASE}/pagefind/pagefind.js`)
    .then(async (module: Pagefind) => {
      await module.options({ baseUrl: `${BASE}/` });
      await module.init();
      return module;
    })
    .catch(() => null);
  return loading;
}

export function DocsSearch() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const typing = event.target instanceof HTMLElement && event.target.closest("input, textarea, [contenteditable]");
      if ((event.key === "k" && (event.metaKey || event.ctrlKey)) || (event.key === "/" && !typing)) {
        event.preventDefault();
        setOpen(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label="Search docs"
        className={cx(
          "flex h-9 w-full cursor-pointer items-center gap-2 rounded-2lg border border-border-button-default bg-background-primary-default px-2.5",
          "text-body-regular text-text-tertiary transition-colors hover:border-border-button-hover hover:text-text-secondary",
          "outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring",
        )}
      >
        <RiSearchLine className="size-4.5 shrink-0 text-foreground-icon-secondary" aria-hidden />
        <span className="flex-1 truncate text-start">Search docs</span>
        <span className="hidden items-center gap-1 sm:flex" aria-hidden>
          <Kbd className="size-5 p-0">⌘</Kbd>
          <Kbd className="size-5 p-0">K</Kbd>
        </span>
      </button>
      <Modal
        isOpen={open}
        onOpenChange={setOpen}
        size="lg"
        aria-label="Search docs"
        overlayClassName="items-start pt-[10vh]"
      >
        <SearchPanel onNavigate={() => setOpen(false)} />
      </Modal>
    </>
  );
}

function SearchPanel({ onNavigate }: { onNavigate: () => void }) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Result[]>([]);
  const [state, setState] = useState<"idle" | "searching" | "unavailable">("idle");
  const [active, setActive] = useState(0);
  const list = useRef<HTMLUListElement>(null);

  useEffect(() => {
    let stale = false;
    const term = query.trim();
    if (!term) {
      setResults([]);
      setState("idle");
      return;
    }
    setState("searching");
    void loadPagefind().then(async (pagefind) => {
      if (!pagefind) {
        return !stale && setState("unavailable");
      }
      const found = await pagefind.debouncedSearch(term);
      if (stale || !found) {
        return;
      }
      const data = await Promise.all(found.results.slice(0, 8).map((result) => result.data()));
      if (stale) {
        return;
      }
      setResults(
        data.map((item) => ({
          url: item.url,
          title: item.meta.title ?? item.url,
          excerpt: item.excerpt,
          sections: (item.sub_results ?? []).filter((sub) => sub.url !== item.url).slice(0, 3),
        })),
      );
      setActive(0);
      setState("idle");
    });
    return () => {
      stale = true;
    };
  }, [query]);

  const onKeyDown = (event: React.KeyboardEvent) => {
    if (!results.length) {
      return;
    }
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const next = (active + (event.key === "ArrowDown" ? 1 : -1) + results.length) % results.length;
      setActive(next);
      list.current?.children[next]?.scrollIntoView({ block: "nearest" });
    } else if (event.key === "Enter") {
      event.preventDefault();
      onNavigate();
      location.assign(results[active].url);
    }
  };

  return (
    <ModalBody className="docs-search flex flex-col gap-3 p-3">
      <Input
        onKeyDown={onKeyDown}
        aria-label="Search docs"
        placeholder="Search guides, API and components"
        value={query}
        onChange={setQuery}
        leadingIcon={RiSearchLine}
        autoFocus
      />
      <div className="max-h-[55vh] min-h-24 overflow-y-auto">
        {state === "unavailable" ? (
          <Empty>Search runs on the built site. Run the docs build, then preview it.</Empty>
        ) : state === "searching" && !results.length ? (
          <div className="grid h-24 place-items-center">
            <Spinner />
          </div>
        ) : !query.trim() ? (
          <Empty>Type to search every page of the docs.</Empty>
        ) : !results.length ? (
          <Empty>No pages match “{query.trim()}”.</Empty>
        ) : (
          <ul ref={list} className="flex flex-col gap-1">
            {results.map((result, index) => (
              <li key={result.url}>
                <a
                  href={result.url}
                  onClick={onNavigate}
                  onMouseEnter={() => setActive(index)}
                  className={cx(
                    "flex gap-3 rounded-2lg p-2.5 outline-none transition-colors",
                    index === active ? "bg-background-secondary-default" : "hover:bg-background-secondary-default",
                  )}
                >
                  <RiFileTextLine className="mt-0.5 size-4.5 shrink-0 text-foreground-icon-secondary" aria-hidden />
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="text-body-medium text-text-primary">{result.title}</span>
                    <span
                      className="line-clamp-2 text-caption-1-regular text-text-secondary"
                      // biome-ignore lint/security/noDangerouslySetInnerHtml: Pagefind's excerpt is escaped text with <mark> around matches
                      dangerouslySetInnerHTML={{ __html: result.excerpt }}
                    />
                    {result.sections.length > 0 && (
                      <span className="mt-1 flex flex-wrap gap-1.5">
                        {result.sections.map((section) => (
                          <span key={section.url} className="text-caption-1-medium text-text-tertiary">
                            # {section.title}
                          </span>
                        ))}
                      </span>
                    )}
                  </span>
                  <RiArrowRightSLine className="mt-0.5 size-4.5 shrink-0 text-foreground-icon-tertiary" aria-hidden />
                </a>
              </li>
            ))}
          </ul>
        )}
      </div>
    </ModalBody>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return (
    <p className="grid h-24 place-items-center px-6 text-center text-body-regular text-text-tertiary">{children}</p>
  );
}
