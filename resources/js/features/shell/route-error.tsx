/*
 * What a page shows when it could not load: what went wrong in plain
 * words and a way to try again, instead of a raw error. Going too fast
 * says so, since waiting a moment is all it takes.
 */
import { RiErrorWarningLine, RiTimerLine } from "@remixicon/react";
import { type ErrorComponentProps, useRouter } from "@tanstack/react-router";

import { Button } from "@/components/base/buttons/button";
import { ApiError } from "@/lib/api/client";

import { EmptyState } from "./page";

export function RouteError({ error, reset }: ErrorComponentProps) {
  const router = useRouter();
  const tooFast = error instanceof ApiError && error.status === 429;
  const retry = () => {
    reset();
    void router.invalidate();
  };

  return (
    <div className="flex min-h-dvh flex-col bg-background-secondary-default">
      <EmptyState
        icon={tooFast ? RiTimerLine : RiErrorWarningLine}
        title={tooFast ? "Slow down a moment" : "This page did not load"}
        description={
          tooFast
            ? "Too many requests were sent in a short time. Wait a few seconds, then try again."
            : "Check your connection and try again. If it keeps happening, reload the page."
        }
      >
        <Button size="small" onClick={retry}>
          Try again
        </Button>
      </EmptyState>
    </div>
  );
}
