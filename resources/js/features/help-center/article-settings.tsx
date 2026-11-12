/*
 * An article's settings, apart from its words: the collection it is
 * filed in, picked from the collections as they nest, whether readers or
 * only dimmi may read it, the audience it is kept to and its address.
 * The audience saves at once; the rest saves with the article. Deleting
 * the article asks first.
 */
import { RiGlobalLine, RiRobot2Line } from "@remixicon/react";
import { useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Label } from "@/components/base/input/label";
import { AlertDialog, Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { RadioGroup } from "@/components/base/radio/radio";
import { RadioCard } from "@/components/base/radio/radio-card";
import type { Article, Collection } from "@/lib/api/help-centers";

import { ArticleAudience } from "./article-audience";
import { CollectionSelect } from "./collection-select";
import type { Draft } from "./docs-format";

export function ArticleSettings({
  workspace,
  article,
  draft,
  collections,
  onChange,
  onSaved,
  onDelete,
  onClose,
}: {
  workspace: string;
  article: Article;
  draft: Draft;
  collections: Collection[];
  onChange: (change: Partial<Draft>) => void;
  onSaved: (next: Article) => void;
  onDelete: () => Promise<unknown>;
  onClose: () => void;
}) {
  const [deleting, setDeleting] = useState(false);

  return (
    <Modal
      size="md"
      className="h-[min(640px,calc(100dvh-3rem))]"
      isOpen
      onOpenChange={(open) => (open ? undefined : onClose())}
    >
      <ModalHeader title="Article settings" description="Changes here are saved with the article." />
      <ModalBody className="pb-4">
        <div className="flex flex-col gap-4">
          <CollectionSelect
            collections={collections}
            value={draft.collection}
            onChange={(collection) => onChange({ collection })}
          />
          <div className="flex flex-col gap-1">
            <Label>Who can read it</Label>
            <RadioGroup
              aria-label="Who can read it"
              value={draft.visibility}
              onChange={(visibility) => onChange({ visibility: visibility as Draft["visibility"] })}
            >
              <RadioCard
                value="public"
                icon={RiGlobalLine}
                title="Public"
                description="On the help center, and dimmi answers from it."
              />
              <RadioCard
                value="agent_only"
                icon={RiRobot2Line}
                title="Agent only"
                description="Hidden from the help center; only dimmi reads it."
              />
            </RadioGroup>
          </div>
          <ArticleAudience workspace={workspace} article={article} onSaved={onSaved} />
          <Input
            label="Address"
            hint="The last part of the article's link."
            value={draft.slug}
            onChange={(slug) => onChange({ slug })}
            maxLength={120}
            inputDir="ltr"
          />
          <div className="mt-2 border-t border-separator-border pt-4">
            <Button variant="danger-secondary" onClick={() => setDeleting(true)}>
              Delete article
            </Button>
          </div>
        </div>
      </ModalBody>
      <ModalFooter>
        <Button onClick={onClose}>Done</Button>
      </ModalFooter>
      <AlertDialog
        isOpen={deleting}
        onOpenChange={(open) => (open ? undefined : setDeleting(false))}
        title="Delete this article?"
        description="It leaves the help center, and dimmi stops answering from it, right away."
        confirmLabel="Delete article"
        onConfirm={onDelete}
      />
    </Modal>
  );
}
