/*
 * The ChatGPT subscription sign-in as a small state machine. It starts
 * with a device code: the person types it on OpenAI's page and the page
 * asks the server, at the pace the server names, whether they approved.
 * When device sign-in is off for their account, or they prefer it, the
 * browser sign-in hands out OpenAI's address instead, and the person
 * pastes back the address their browser ended on. The tokens never reach
 * the browser; the server keeps them on the new connection.
 */
import { useCallback, useEffect, useRef, useState } from "react";
import { type ChatGPTStart, completeChatGPTConnect, startChatGPTConnect } from "@/lib/api/ai";
import { ApiError } from "@/lib/api/client";

export type SignInMethod = ChatGPTStart["method"];

/*
 * Phase is where the sign-in stands.
 */
export type Phase =
  | { kind: "starting"; method: SignInMethod }
  | { kind: "device"; start: Extract<ChatGPTStart, { method: "device" }> }
  | { kind: "browser"; start: Extract<ChatGPTStart, { method: "browser" }> }
  | { kind: "failed"; method: SignInMethod; code: string; message: string };

const messages: Record<string, string> = {
  chatgpt_unavailable: "ChatGPT sign-in is not turned on for this install.",
  chatgpt_device_disabled:
    "Device sign-in is off for this ChatGPT account. Use browser sign-in instead, or turn device sign-in on in ChatGPT's security settings.",
  oauth_state_invalid: "This sign-in expired. Start again.",
  oauth_code_required: "Paste the full address from your browser's address bar.",
  oauth_state_mismatch: "That address belongs to another sign-in. Start the browser sign-in again.",
  oauth_exchange_failed: "OpenAI did not accept the sign-in. Try again.",
};

/*
 * failure reads a refusal as its code and the words a person reads.
 */
export function failure(error: unknown): { code: string; message: string } {
  if (error instanceof ApiError) {
    return { code: error.code, message: messages[error.code] ?? error.message };
  }

  return { code: "network", message: "dimmi could not reach the server. Check your connection and try again." };
}

/*
 * useChatGPTSignIn runs the sign-in and calls onConnected once the
 * connection exists. It starts with a device code as soon as it mounts.
 */
export function useChatGPTSignIn(workspace: string, enabled: boolean, onConnected: () => void) {
  const [phase, setPhase] = useState<Phase>({ kind: "starting", method: "device" });
  const connected = useRef(onConnected);
  const started = useRef(false);
  connected.current = onConnected;

  const begin = useCallback(
    async (method: SignInMethod) => {
      setPhase({ kind: "starting", method });
      try {
        const start = await startChatGPTConnect(workspace, method);
        setPhase(start.method === "device" ? { kind: "device", start } : { kind: "browser", start });
      } catch (error) {
        setPhase({ kind: "failed", method, ...failure(error) });
      }
    },
    [workspace],
  );

  useEffect(() => {
    if (!enabled || started.current) {
      return;
    }
    started.current = true;
    void begin("device");
  }, [begin, enabled]);

  const deviceState = phase.kind === "device" ? phase.start.state : null;
  const deviceInterval = phase.kind === "device" ? phase.start.interval_seconds : 5;

  useEffect(() => {
    if (!deviceState) {
      return;
    }
    let stopped = false;
    let timer: number | undefined;
    const tick = async () => {
      try {
        const progress = await completeChatGPTConnect(workspace, { state: deviceState });
        if (stopped) {
          return;
        }
        if (progress.status === "connected") {
          connected.current();
          return;
        }
        timer = window.setTimeout(tick, (progress.interval_seconds || deviceInterval) * 1000);
      } catch (error) {
        if (!stopped) {
          setPhase({ kind: "failed", method: "device", ...failure(error) });
        }
      }
    };
    timer = window.setTimeout(tick, deviceInterval * 1000);

    return () => {
      stopped = true;
      window.clearTimeout(timer);
    };
  }, [deviceInterval, deviceState, workspace]);

  /*
   * paste finishes a browser sign-in with the address the browser ended
   * on, answering the words to show when it was refused.
   */
  const paste = async (address: string): Promise<string | null> => {
    if (phase.kind !== "browser") {
      return null;
    }
    try {
      const progress = await completeChatGPTConnect(workspace, { state: phase.start.state, code: address.trim() });
      if (progress.status === "connected") {
        connected.current();
        return null;
      }
      return "OpenAI has not finished the sign-in yet. Try again in a moment.";
    } catch (error) {
      const refused = failure(error);
      if (refused.code === "oauth_state_invalid") {
        setPhase({ kind: "failed", method: "browser", ...refused });
      }
      return refused.message;
    }
  };

  return { phase, begin, paste };
}
