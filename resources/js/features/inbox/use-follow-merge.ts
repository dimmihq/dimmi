/*
 * A link to a conversation that was merged into another opens the one
 * it went into: the server answers merged_into with the target, and the
 * screen moves there, replacing the old address, with a short note.
 */
import { useNavigate } from "@tanstack/react-router";
import { useEffect } from "react";

import { toast } from "@/components/base/toast/toast";
import { ApiError } from "@/lib/api/client";

/*
 * mergedTarget returns where a merged conversation went, from the error
 * the server answered, or null for any other error.
 */
export function mergedTarget(error: unknown): { id: string; number: number | null } | null {
  if (!(error instanceof ApiError) || error.code !== "merged_into") {
    return null;
  }
  const id = error.details.target_id;
  const number = error.details.target_number;

  return typeof id === "string" ? { id, number: typeof number === "number" ? number : null } : null;
}

/*
 * useFollowMerge moves to the conversation a merged one went into, and
 * reports whether it is doing so.
 */
export function useFollowMerge(view: string, error: unknown): boolean {
  const navigate = useNavigate();
  const target = mergedTarget(error);
  const to = target?.id;
  const number = target?.number;

  useEffect(() => {
    if (!to) {
      return;
    }
    toast.success(number ? `Merged into #${number}` : "Merged into another conversation");
    void navigate({ to: "/d/inbox/$view/$conversation", params: { view, conversation: to }, replace: true });
  }, [to, number, view, navigate]);

  return to !== undefined;
}
