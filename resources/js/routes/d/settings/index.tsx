import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/d/settings/")({
  beforeLoad: () => {
    throw redirect({ to: "/d/settings/$section", params: { section: "profile" } });
  },
});
