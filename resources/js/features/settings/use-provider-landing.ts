/*
 * Reading what a tool said when it sent the browser back after an
 * install or a teammate's own grant. The return is read once and taken
 * out of the address, so a reload does not toast again.
 */
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { toast } from "@/components/base/toast/toast";

import { landingOf } from "./integrations";

/*
 * useProviderLanding toasts what the tool said when it sent the browser
 * back, refreshes the installations and cleans the address.
 */
export function useProviderLanding(workspace: string) {
  const queryClient = useQueryClient();
  const read = useRef(false);

  useEffect(() => {
    if (read.current) {
      return;
    }
    read.current = true;

    const landing = landingOf(window.location.search);
    if (!landing) {
      return;
    }

    window.history.replaceState(window.history.state, "", window.location.pathname);
    void queryClient.invalidateQueries({ queryKey: ["workspace", workspace, "integrations"] });

    if (landing.kind === "connected") {
      toast.success("Connected. Every feature can use it now.");
      return;
    }

    toast.error(landing.message);
  }, [queryClient, workspace]);
}
