/*
 * useMessageMoves gives the thread what it needs to act on messages:
 * who the reader is, whether they may moderate, and the calls that
 * delete a message, rewrite a note or react to one, reading the timeline
 * again after.
 */
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { deleteMessage, editNote, react } from "@/lib/api/inbox";
import { callerQuery } from "@/lib/api/workspace";
import { useMutationId } from "@/lib/realtime/context";

import { useInbox } from "../conversations";

export function useMessageMoves(conversation: string) {
  const { workspace } = useInbox();
  const client = useQueryClient();
  const mutationId = useMutationId();
  const { data: caller } = useQuery({ ...callerQuery(workspace), enabled: workspace !== "" });
  const refresh = () => client.invalidateQueries({ queryKey: ["workspace", workspace] });

  return {
    me: caller?.member?.id ?? null,
    moderates: caller?.member?.permissions.includes("conversations:moderate") ?? false,
    remove: async (message: string) => {
      await deleteMessage({ workspace, mutation: mutationId() }, conversation, message);
      await refresh();
    },
    edit: async (message: string, body: string) => {
      await editNote({ workspace, mutation: mutationId() }, conversation, message, body);
      await refresh();
    },
    react: async (message: string, emoji: string, remove: boolean) => {
      await react({ workspace, mutation: mutationId() }, conversation, message, emoji, remove);
      await refresh();
    },
    link: (message: string) => {
      const address = `${window.location.origin}/d/inbox/all/${conversation}#${message}`;
      void navigator.clipboard?.writeText(address).catch(() => undefined);
    },
  };
}
