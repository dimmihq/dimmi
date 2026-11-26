/*
 * The words of the page a person waiting for an admin sees.
 */
import type { AwaitingAccess } from "@/lib/api/auth";

/*
 * awaitingLine names the workspaces the person waits in.
 */
export function awaitingLine(awaiting: AwaitingAccess[]): string {
  const names = awaiting.map((w) => w.name);
  const where =
    names.length <= 1 ? (names[0] ?? "your team's workspace") : `${names.slice(0, -1).join(", ")} and ${names.at(-1)}`;

  return `You're set up in ${where}. Its admins know you're here. We'll email you the moment you're in.`;
}
