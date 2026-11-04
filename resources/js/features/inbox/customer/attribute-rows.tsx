/*
 * The customer's own attributes, raw keys as labels. A star on a row pins
 * the key for this teammate across every customer, and pinned keys show
 * first, above the contact details, so the plan or account size someone
 * checks on every conversation is the first thing they read. Pins are
 * saved with the person's inbox preferences.
 */
import { RiStarFill, RiStarLine } from "@remixicon/react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { IconButton } from "@/components/base/buttons/icon-button";
import type { ConversationContext } from "@/lib/api/customers";
import { type InboxPreferences, preferencesQuery, updatePreferences } from "@/lib/api/inbox";
import { useMutationId } from "@/lib/realtime/context";
import { realtimeKeys } from "@/lib/realtime/keys";

import { show } from "./format";
import { Copyable, Row } from "./rows";

function usePins(workspace: string) {
  const client = useQueryClient();
  const mutation = useMutationId();
  const { data } = useQuery({ ...preferencesQuery(workspace), enabled: workspace !== "" });
  const pinned = data?.pinned_attributes ?? [];
  const toggle = (key: string) => {
    const next = pinned.includes(key) ? pinned.filter((pin) => pin !== key) : [...pinned, key];
    const cacheKey = realtimeKeys.preferences(workspace);
    client.setQueryData<InboxPreferences>(cacheKey, (old) => (old ? { ...old, pinned_attributes: next } : old));
    void updatePreferences({ workspace, mutation: mutation() }, { pinned_attributes: next }).catch(() =>
      client.invalidateQueries({ queryKey: cacheKey }),
    );
  };

  return { pinned, toggle };
}

export function AttributeRows({
  workspace,
  context,
  pinnedOnly,
}: {
  workspace: string;
  context: ConversationContext;
  pinnedOnly: boolean;
}) {
  const { pinned, toggle } = usePins(workspace);
  const entries = Object.entries(context.contact.custom_attributes).filter(
    ([key]) => pinned.includes(key) === pinnedOnly,
  );
  if (pinnedOnly) {
    entries.sort(([a], [b]) => pinned.indexOf(a) - pinned.indexOf(b));
  }

  return (
    <>
      {entries.map(([key, value]) => (
        <Row
          key={key}
          label={
            <>
              <IconButton
                size="small"
                icon={pinnedOnly ? RiStarFill : RiStarLine}
                aria-label={pinnedOnly ? `Unpin ${key}` : `Pin ${key}`}
                onClick={() => toggle(key)}
              />
              <span className="truncate">{key}</span>
            </>
          }
        >
          <Copyable value={show(value)} label={key} />
        </Row>
      ))}
    </>
  );
}
