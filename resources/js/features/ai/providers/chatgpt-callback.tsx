/*
 * Where OpenAI sends a person back after they sign in with ChatGPT. The
 * page hands the one-time code to the server, then returns to the
 * providers page with the new connection listed. When OpenAI reports a
 * refusal, or the code has expired, it says so and offers the way back.
 */
import { RiErrorWarningLine } from "@remixicon/react";
import { useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";

import { Spinner } from "@/components/base/spinner/spinner";
import { toast } from "@/components/base/toast/toast";
import { useAccount } from "@/features/settings/use-account";
import { EmptyState, Panel } from "@/features/shell/page";
import { aiKey } from "@/lib/api/ai";

import { refusal } from "../use-agent";
import { completeChatGPTConnect } from "./chatgpt";

export function ChatGPTCallback({ code, state, error }: { code?: string; state?: string; error?: string }) {
  const workspace = useAccount().workspace.id;
  const client = useQueryClient();
  const navigate = useNavigate();
  const [failure, setFailure] = useState<string | null>(
    error || !code || !state ? "ChatGPT did not finish the sign-in." : null,
  );
  const started = useRef(false);

  useEffect(() => {
    if (started.current || !code || !state || error || workspace === "") {
      return;
    }
    started.current = true;
    completeChatGPTConnect(workspace, { code, state })
      .then(async () => {
        await client.invalidateQueries({ queryKey: aiKey(workspace) });
        toast.success("ChatGPT connected");
        await navigate({ to: "/d/settings/$section", params: { section: "ai-providers" }, replace: true });
      })
      .catch((reason: unknown) => setFailure(refusal(reason, "The sign-in could not be finished.")));
  }, [client, code, error, navigate, state, workspace]);

  return (
    <Panel label="Connect ChatGPT" ground className="h-full flex-1">
      {failure ? (
        <EmptyState icon={RiErrorWarningLine} title="ChatGPT is not connected" description={failure}>
          <Link
            to="/d/settings/$section"
            params={{ section: "ai-providers" }}
            className="rounded-lg text-body-medium text-text-primary underline outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring"
          >
            Back to AI providers
          </Link>
        </EmptyState>
      ) : (
        <div className="flex flex-1 flex-col items-center justify-center gap-3">
          <Spinner size="md" />
          <p className="text-body-regular text-text-secondary">Connecting your ChatGPT account…</p>
        </div>
      )}
    </Panel>
  );
}
