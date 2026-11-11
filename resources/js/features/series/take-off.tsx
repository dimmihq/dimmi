/*
 * Taking someone off a series: a search over the people standing on one
 * step, or on the whole series from its entry, by name or email or the
 * company's name. The teammate picks one person and takes them off; the
 * journey ends for good and nothing more is sent to them.
 */
import { RiSearchLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useDeferredValue, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Input } from "@/components/base/input/input";
import { Modal, ModalBody, ModalFooter, ModalHeader } from "@/components/base/modal/modal";
import { Radio, RadioGroup } from "@/components/base/radio/radio";
import { toast } from "@/components/base/toast/toast";
import { removeFromSeries, type SeriesJourney, seriesJourneysQuery, seriesKey } from "@/lib/api/series";
import { formatRelative } from "@/lib/i18n/format";

const travelling = new Set(["active", "waiting"]);

function nameOf(j: SeriesJourney) {
  return j.contact_name || j.company_name || j.contact_email || "Someone";
}

function whereOf(j: SeriesJourney) {
  return j.wake_at ? `Moves on ${formatRelative(j.wake_at)}` : `Entered ${formatRelative(j.entered_at)}`;
}

/*
 * TakeOff is the dialog. node narrows it to one step; without it, it
 * looks across everyone still on the series.
 */
export function TakeOff({
  workspace,
  series,
  node,
  onClose,
}: {
  workspace: string;
  series: string;
  node?: string;
  onClose: () => void;
}) {
  const client = useQueryClient();
  const [search, setSearch] = useState("");
  const [picked, setPicked] = useState<string | null>(null);
  const q = useDeferredValue(search.trim());
  const found = useQuery(seriesJourneysQuery(workspace, series, node, q));
  const people = (found.data ?? []).filter((j) => travelling.has(j.status));
  const remove = useMutation({
    mutationFn: (journey: string) => removeFromSeries(workspace, series, journey),
    onSuccess: () => {
      void client.invalidateQueries({ queryKey: seriesKey(workspace) });
      toast.success("Taken off the series.");
      onClose();
    },
    onError: (error) => void toast.error(error instanceof Error ? error.message : "They were not taken off."),
  });

  return (
    <Modal isOpen onOpenChange={(open) => (open ? undefined : onClose())} size="md">
      <ModalHeader
        title="Take someone off"
        description="Their journey ends now and nothing more is sent to them. This can't be undone."
      />
      <ModalBody className="flex flex-col gap-3">
        <Input
          aria-label="Find someone"
          placeholder="Find by name or email"
          leadingIcon={RiSearchLine}
          autoFocus
          value={search}
          onChange={setSearch}
        />
        <div className="flex max-h-72 min-h-24 flex-col overflow-y-auto">
          {people.length === 0 ? (
            <p className="py-6 text-center text-body-2-regular text-text-tertiary">
              {found.isPending ? "Looking…" : q ? "Nobody here matches." : "Nobody is here right now."}
            </p>
          ) : (
            <RadioGroup aria-label="People" value={picked} onChange={setPicked} className="gap-0">
              {people.map((j) => (
                <Radio
                  key={j.id}
                  value={j.id}
                  className="w-full rounded-xl px-2 py-2 hover:bg-background-secondary-hover"
                >
                  <span className="flex min-w-0 flex-col">
                    <span className="truncate text-body-2-medium text-text-primary">{nameOf(j)}</span>
                    <span className="truncate text-caption-1-medium text-text-tertiary">{whereOf(j)}</span>
                  </span>
                </Radio>
              ))}
            </RadioGroup>
          )}
        </div>
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" size="small" onClick={onClose}>
          Cancel
        </Button>
        <Button
          variant="danger"
          size="small"
          disabled={picked === null}
          loading={remove.isPending}
          onClick={() => (picked ? remove.mutate(picked) : undefined)}
        >
          Take off
        </Button>
      </ModalFooter>
    </Modal>
  );
}
