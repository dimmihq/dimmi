/*
 * Keeps an open conversation current: its record and its timeline
 * follow the conversation's channel for as long as it is on screen.
 */
import { useRealtime } from "./context";
import { channels } from "./keys";
import { useChannel } from "./use-channel";

/*
 * useConversationRealtime follows the conversation, by its public id,
 * and applies what it hears to the query cache.
 */
export function useConversationRealtime(conversation: string | null) {
  const { workspace, bridge } = useRealtime();

  useChannel(conversation ? channels.conversation(conversation) : null, {
    onEnvelope: (envelope, channel) => {
      if (workspace && bridge) {
        bridge.apply(workspace, channel, envelope);
      }
    },
    onSubscribed: (recovered, wasRecovering) => {
      if (workspace && bridge && wasRecovering && !recovered) {
        bridge.resync(workspace);
      }
    },
  });
}
