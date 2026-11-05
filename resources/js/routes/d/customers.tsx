import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/d/customers")({
  component: Outlet,
});
