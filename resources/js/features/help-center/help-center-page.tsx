/*
 * The help center a workspace publishes: its default help center's articles,
 * with the way to write a new one and to open the help center as readers
 * see it. On a wide screen the collections stand as a tree in a pane of
 * their own, where they are also arranged, and picking one lists its
 * articles beside it; on a narrow one they are chips above the list, and
 * the tree slides in from the side. The address keeps the pick, so
 * coming back from an article returns to the same collection. Opening an
 * article edits it.
 */
import { RiAddLine, RiArticleLine, RiCornerDownRightLine, RiExternalLinkLine, RiFolder3Line } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { type FormEvent, type ReactNode, useRef, useState } from "react";
import { Panel as Pane } from "react-resizable-panels";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Drawer } from "@/components/base/drawer/drawer";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import { PaneGroup, PaneHandle } from "@/features/shell/resizable";
import { useWide } from "@/features/shell/use-wide";
import {
  type Article,
  articlesQuery,
  type Collection,
  collectionsQuery,
  createArticle,
  type HelpCenter,
  helpCentersQuery,
} from "@/lib/api/help-centers";
import { cx } from "@/utils/cx";

import { CollectionPanel } from "./collection-panel";
import { CollectionSelect } from "./collection-select";
import { allKey, articlesIn, childrenOf, unfiledKey } from "./collection-tree";
import { articleDetail, articleState } from "./docs-format";

type Dialog = "article" | "collections" | null;

const pane = { group: "dimmi:docs", tree: "collections", list: "articles" } as const;

const visible = { overflow: "visible" } as const;

/*
 * pickTitle names what a pick shows, with a nested collection's parent
 * before it.
 */
function pickTitle(pick: string, collections: Collection[]): string {
  if (pick === allKey) {
    return "All articles";
  }
  if (pick === unfiledKey) {
    return "No collection";
  }
  const picked = collections.find((c) => c.id === pick);
  const parent = collections.find((c) => c.id === picked?.parent_id);

  return [parent?.name, picked?.name ?? "Collection"].filter(Boolean).join(" / ");
}

/*
 * usePick reads the collection the address names and moves to another.
 */
function usePick(): [string, (pick: string) => void] {
  const { collection } = useSearch({ from: "/d/help-center/" });
  const navigate = useNavigate({ from: "/d/help-center/" });

  return [
    collection ?? allKey,
    (pick) => void navigate({ search: pick === allKey ? {} : { collection: pick }, replace: true }),
  ];
}

function useDocs() {
  const workspace = useAccount().workspace.id;
  const enabled = workspace !== "";
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled });
  const center = centers[0];
  const hc = center?.id ?? "";
  const { data: collections = [] } = useQuery({ ...collectionsQuery(workspace, hc), enabled: hc !== "" });
  const { data: articles = [], isPending } = useQuery({ ...articlesQuery(workspace, hc), enabled: hc !== "" });

  return { workspace, center, hc, collections, articles, isPending };
}

export function HelpCenterPage() {
  const docs = useDocs();
  const { workspace, center, hc, collections, articles } = docs;
  const [pick, setPick] = usePick();
  const [dialog, setDialog] = useState<Dialog>(null);
  const wide = useWide(1024);
  const inCollection = pick !== allKey && pick !== unfiledKey ? pick : "";
  const tree = (onPicked: (pick: string) => void) => (
    <CollectionPanel
      workspace={workspace}
      helpCenter={hc}
      title={center?.name ?? "Help center"}
      collections={collections}
      articles={articles}
      pick={pick}
      onPick={onPicked}
    />
  );
  const list = (
    <ArticleList
      pick={pick}
      center={center}
      title={wide ? pickTitle(pick, collections) : (center?.name ?? "Help center")}
      articles={articlesIn(pick, articles, collections)}
      collections={collections}
      isPending={docs.isPending}
      onDialog={setDialog}
      chips={wide ? null : <CollectionChips collections={collections} pick={pick} onPick={setPick} />}
    />
  );

  return (
    <>
      {wide ? (
        <PaneGroup id={pane.group} panelIds={[pane.tree, pane.list]}>
          <Pane
            id={pane.tree}
            defaultSize={260}
            minSize={200}
            maxSize={420}
            groupResizeBehavior="preserve-pixel-size"
            style={visible}
            className="flex h-full"
          >
            {tree(setPick)}
          </Pane>
          <PaneHandle />
          <Pane id={pane.list} minSize={480} style={visible} className="flex h-full">
            {list}
          </Pane>
        </PaneGroup>
      ) : (
        list
      )}
      {dialog === "article" && center ? (
        <NewArticleDialog
          workspace={workspace}
          helpCenter={hc}
          collections={collections}
          collection={inCollection}
          onClose={() => setDialog(null)}
        />
      ) : null}
      {wide ? null : (
        <Drawer
          isOpen={dialog === "collections"}
          onOpenChange={(open) => setDialog(open ? "collections" : null)}
          placement="start"
          size="sm"
          aria-label="Collections"
          className="p-0"
        >
          {tree((next) => {
            setPick(next);
            setDialog(null);
          })}
        </Drawer>
      )}
    </>
  );
}

/*
 * useMoved says whether the pick has changed since the list first
 * showed, so a newly picked collection's articles arrive with the app's
 * page reveal while the first list is simply there.
 */
function useMoved(pick: string): boolean {
  const first = useRef(pick);
  const moved = useRef(false);
  if (pick !== first.current) {
    moved.current = true;
  }

  return moved.current;
}

function ArticleList({
  pick,
  center,
  title,
  articles,
  collections,
  isPending,
  onDialog,
  chips,
}: {
  pick: string;
  center: HelpCenter | undefined;
  title: string;
  articles: Article[];
  collections: Collection[];
  isPending: boolean;
  onDialog: (dialog: Dialog) => void;
  chips: ReactNode;
}) {
  const moved = useMoved(pick);

  return (
    <Panel label="Help center" className="h-full w-full flex-1">
      <PageHeader title={title}>
        {center ? (
          <a
            href={center.url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 rounded-lg px-2 py-1.5 text-body-2-medium text-text-secondary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            <RiExternalLinkLine aria-hidden className="size-4 text-foreground-icon-tertiary" />
            View help center
          </a>
        ) : null}
        {chips ? (
          <Button
            size="small"
            variant="secondary"
            leadingIcon={RiFolder3Line}
            onClick={() => onDialog("collections")}
            disabled={!center}
          >
            Collections
          </Button>
        ) : null}
        <Button size="small" leadingIcon={RiAddLine} onClick={() => onDialog("article")} disabled={!center}>
          New article
        </Button>
      </PageHeader>
      {chips}
      <ScrollArea>
        <div key={pick} className={cx("flex flex-col gap-1 px-2 pb-4", moved && "animate-page-in")}>
          {!isPending && articles.length === 0 ? (
            <p className="px-2 py-6 text-body-2-regular text-text-tertiary">
              No articles here yet. Write one; it stays a draft until you publish it.
            </p>
          ) : null}
          {articles.map((article) => (
            <ArticleRow key={article.id} article={article} detail={articleDetail(article, collections)} />
          ))}
        </div>
      </ScrollArea>
    </Panel>
  );
}

function chipClass(on: boolean): string {
  return cx(
    "rounded-full px-3 py-1 text-body-2-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-border-focus-ring",
    on ? "bg-background-tertiary-default text-text-primary" : "text-text-secondary hover:bg-background-secondary-hover",
  );
}

/*
 * CollectionChips is the narrow screen's way to pick a collection: the
 * top level as chips, and the collections inside the open one below.
 */
function CollectionChips({
  collections,
  pick,
  onPick,
}: {
  collections: Collection[];
  pick: string;
  onPick: (pick: string) => void;
}) {
  const parent = collections.find((c) => c.id === pick)?.parent_id ?? null;
  const openTop = parent ?? (pick === allKey || pick === unfiledKey ? null : pick);
  const subs = openTop ? childrenOf(collections, openTop) : [];

  return (
    <>
      <nav aria-label="Collections" className="flex flex-wrap items-center gap-1 px-4 pb-3">
        {[{ id: allKey, name: "All articles" }, ...childrenOf(collections, null)].map((c) => {
          const on = pick === c.id || c.id === parent;

          return (
            <button key={c.id} type="button" aria-pressed={on} onClick={() => onPick(c.id)} className={chipClass(on)}>
              {c.name}
            </button>
          );
        })}
      </nav>
      {subs.length > 0 && openTop ? (
        <nav aria-label="Collections inside" className="flex flex-wrap items-center gap-1 px-4 pb-3">
          <RiCornerDownRightLine aria-hidden className="size-4 text-foreground-icon-quaternary" />
          {subs.map((c) => (
            <button
              key={c.id}
              type="button"
              aria-pressed={pick === c.id}
              onClick={() => onPick(pick === c.id ? openTop : c.id)}
              className={chipClass(pick === c.id)}
            >
              {c.name}
            </button>
          ))}
        </nav>
      ) : null}
    </>
  );
}

function ArticleRow({ article, detail }: { article: Article; detail: string }) {
  const state = articleState(article);

  return (
    <Link
      to="/d/help-center/$article"
      params={{ article: article.id }}
      className="flex items-center gap-3 rounded-xl px-2 py-2.5 outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
    >
      <RiArticleLine aria-hidden className="size-5 shrink-0 text-foreground-icon-tertiary" />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-body-regular text-text-primary">{article.title}</span>
        <span className="truncate text-body-2-regular text-text-secondary">{detail}</span>
      </span>
      <Chip variant="caption" color={state === "Published" ? "lime" : state === "Draft" ? "gray" : "orange"}>
        {state}
      </Chip>
    </Link>
  );
}

/*
 * NewArticleDialog asks for a title and the collection to file the
 * article in, starting from the collection the page is showing.
 */
function NewArticleDialog({
  workspace,
  helpCenter,
  collections,
  collection,
  onClose,
}: {
  workspace: string;
  helpCenter: string;
  collections: Collection[];
  collection: string;
  onClose: () => void;
}) {
  const [home, setHome] = useState(collection);
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [title, setTitle] = useState("");
  const create = useMutation({
    mutationFn: () => createArticle(workspace, helpCenter, { title, ...(home ? { collection_id: home } : {}) }),
    onSuccess: (article) => {
      void queryClient.invalidateQueries({ queryKey: articlesQuery(workspace, helpCenter).queryKey });
      void navigate({ to: "/d/help-center/$article", params: { article: article.id } });
    },
  });
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    create.mutate();
  };

  return (
    <Modal isOpen size="md" onOpenChange={(open) => !open && onClose()}>
      <form onSubmit={submit} className="flex min-h-0 flex-col gap-4">
        <ModalHeader icon={RiArticleLine} title="New article" description="It stays a draft until you publish it." />
        <ModalBody className="flex flex-col gap-4">
          {create.error ? (
            <Notification status="error" title={create.error.message} dismissible={false} role="alert" />
          ) : null}
          <Input label="Title" value={title} onChange={setTitle} isRequired maxLength={150} autoFocus />
          <CollectionSelect collections={collections} value={home} onChange={setHome} />
        </ModalBody>
        <ModalFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={title.trim() === "" || create.isPending}>
            {create.isPending ? "Creating…" : "Create article"}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}
