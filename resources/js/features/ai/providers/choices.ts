/*
 * Which providers the Add provider picker offers, and under which
 * heading. dimmi's own models come with the workspace and Engy is not
 * offered for new connections, so neither is ever a choice. The rest
 * split by how they connect: an API key billed per use, or a plan the
 * person already pays for. A heading with nothing under it is left out.
 */
import type { AiProviderDescriptor } from "@/lib/api/ai";

/*
 * ProviderGroup is one heading of the picker: its name, the line under
 * it, and the providers it holds in the order the server listed them.
 */
export interface ProviderGroup {
  id: "keys" | "plans";
  name: string;
  detail: string;
  providers: AiProviderDescriptor[];
}

const hidden = new Set(["dimmi", "engy"]);

/*
 * providerGroups sorts the providers the server describes into the
 * picker's headings.
 */
export function providerGroups(providers: AiProviderDescriptor[]): ProviderGroup[] {
  const offered = providers.filter((descriptor) => !hidden.has(descriptor.id));
  const groups: ProviderGroup[] = [
    {
      id: "keys",
      name: "API keys",
      detail: "Your own key. Usage is billed by the provider.",
      providers: offered.filter((descriptor) => descriptor.auth_type !== "oauth"),
    },
    {
      id: "plans",
      name: "Subscriptions",
      detail: "Sign in with a plan you already pay for.",
      providers: offered.filter((descriptor) => descriptor.auth_type === "oauth"),
    },
  ];

  return groups.filter((group) => group.providers.length > 0);
}
