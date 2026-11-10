import { createFileRoute } from "@tanstack/react-router";

import { BroadcastList } from "@/features/broadcasts/broadcast-list";

export const Route = createFileRoute("/d/broadcasts/")({
  component: BroadcastList,
});
