import { createFileRoute } from "@tanstack/react-router";

import { ChatGPTCallback } from "@/features/ai/providers/chatgpt-callback";

/*
 * text reads one answer OpenAI put on the address.
 */
function text(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}

export const Route = createFileRoute("/d/settings/ai/chatgpt/callback")({
  validateSearch: (search: Record<string, unknown>): { code?: string; state?: string; error?: string } => ({
    code: text(search.code),
    state: text(search.state),
    error: text(search.error),
  }),
  component: CallbackRoute,
});

function CallbackRoute() {
  const { code, state, error } = Route.useSearch();

  return <ChatGPTCallback code={code} state={state} error={error} />;
}
