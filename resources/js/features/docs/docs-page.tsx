/*
 * The docs a workspace publishes: its default help center's articles,
 * narrowed by collection, with the way to write a new one and to open
 * the help center as readers see it. Opening an article edits it.
 */
import { RiAddLine, RiArticleLine, RiExternalLinkLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { Chip } from "@/components/base/badges/chip";
import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Notification } from "@/components/base/notification/notification";
import { useAccount } from "@/features/settings/use-account";
import { PageHeader, Panel, ScrollArea } from "@/features/shell/page";
import {
  type Article,
  articlesQuery,
  collectionsQuery,
  createArticle,
  createCollection,
  helpCentersQuery,
} from "@/lib/api/help-centers";
import { cx } from "@/utils/cx";

import { articleDetail, articleState } from "./docs-format";

type Dialog = "article" | "collection" | null;

export function DocsPage() {
  const workspace = useAccount().workspace.id;
  const enabled = workspace !== "";
  const { data: centers = [] } = useQuery({ ...helpCentersQuery(workspace), enabled });
  const center = centers[0];
  const hc = center?.id ?? "";
  const { data: collections = [] } = useQuery({ ...collectionsQuery(workspace, hc), enabled: hc !== "" });
  const { data: articles = [], isPending } = useQuery({ ...articlesQuery(workspace, hc), enabled: hc !== "" });
  const [filter, setFilter] = useState<string>("");
  const [dialog, setDialog] = useState<Dialog>(null);
  const shown = filter === "" ? articles : articles.filter((a) => a.collection_id === filter);

  return (
    <Panel label="Docs" className="h-full flex-1">
      <PageHeader title={center?.name ?? "Docs"}>
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
        <Button size="small" variant="secondary" onClick={() => setDialog("collection")} disabled={!center}>
          New collection
        </Button>
        <Button size="small" leadingIcon={RiAddLine} onClick={() => setDialog("article")} disabled={!center}>
          New article
        </Button>
      </PageHeader>
      <nav aria-label="Collections" className="flex flex-wrap items-center gap-1 px-4 pb-3">
        {[{ id: "", name: "All articles" }, ...collections].map((c) => (
          <button
            key={c.id || "all"}
            type="button"
            aria-pressed={filter === c.id}
            onClick={() => setFilter(c.id)}
            className={cx(
              "rounded-full px-3 py-1 text-body-2-medium outline-none transition-colors focus-visible:ring-2 focus-visible:ring-border-focus-ring",
              filter === c.id
                ? "bg-background-tertiary-default text-text-primary"
                : "text-text-secondary hover:bg-background-secondary-hover",
            )}
          >
            {c.name}
          </button>
        ))}
      </nav>
      <ScrollArea>
        <div className="flex flex-col gap-1 px-2 pb-4">
          {!isPending && shown.length === 0 ? (
            <p className="px-2 py-6 text-body-2-regular text-text-tertiary">
              No articles yet. Write the first one; it stays a draft until you publish it.
            </p>
          ) : null}
          {shown.map((article) => (
            <ArticleRow key={article.id} article={article} detail={articleDetail(article, collections)} />
          ))}
        </div>
      </ScrollArea>
      {dialog === "article" && center ? (
        <NewArticleDialog workspace={workspace} helpCenter={hc} collection={filter} onClose={() => setDialog(null)} />
      ) : null}
      {dialog === "collection" && center ? (
        <NewCollectionDialog workspace={workspace} helpCenter={hc} onClose={() => setDialog(null)} />
      ) : null}
    </Panel>
  );
}

function ArticleRow({ article, detail }: { article: Article; detail: string }) {
  const state = articleState(article);

  return (
    <Link
      to="/d/docs/$article"
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

function NewArticleDialog({
  workspace,
  helpCenter,
  collection,
  onClose,
}: {
  workspace: string;
  helpCenter: string;
  collection: string;
  onClose: () => void;
}) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [title, setTitle] = useState("");
  const create = useMutation({
    mutationFn: () =>
      createArticle(workspace, helpCenter, { title, ...(collection ? { collection_id: collection } : {}) }),
    onSuccess: (article) => {
      void queryClient.invalidateQueries({ queryKey: articlesQuery(workspace, helpCenter).queryKey });
      void navigate({ to: "/d/docs/$article", params: { article: article.id } });
    },
  });
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    create.mutate();
  };

  return (
    <Modal isOpen onOpenChange={(open) => !open && onClose()}>
      <form onSubmit={submit} className="flex min-h-0 flex-col gap-4">
        <ModalHeader icon={RiArticleLine} title="New article" description="It stays a draft until you publish it." />
        <ModalBody className="flex flex-col gap-4">
          {create.error ? (
            <Notification status="error" title={create.error.message} dismissible={false} role="alert" />
          ) : null}
          <Input label="Title" value={title} onChange={setTitle} isRequired maxLength={150} autoFocus />
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

function NewCollectionDialog({
  workspace,
  helpCenter,
  onClose,
}: {
  workspace: string;
  helpCenter: string;
  onClose: () => void;
}) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const create = useMutation({
    mutationFn: () => createCollection(workspace, helpCenter, name),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: collectionsQuery(workspace, helpCenter).queryKey });
      onClose();
    },
  });
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    create.mutate();
  };

  return (
    <Modal isOpen onOpenChange={(open) => !open && onClose()}>
      <form onSubmit={submit} className="flex min-h-0 flex-col gap-4">
        <ModalHeader icon={RiArticleLine} title="New collection" description="Group related articles together." />
        <ModalBody className="flex flex-col gap-4">
          {create.error ? (
            <Notification status="error" title={create.error.message} dismissible={false} role="alert" />
          ) : null}
          <Input label="Name" value={name} onChange={setName} isRequired maxLength={80} autoFocus />
        </ModalBody>
        <ModalFooter>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" disabled={name.trim() === "" || create.isPending}>
            {create.isPending ? "Creating…" : "Create collection"}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  );
}
