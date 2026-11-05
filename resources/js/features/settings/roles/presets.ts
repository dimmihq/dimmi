/*
 * Starting points for a new custom role. A preset only fills in the
 * editor: the person still names the role and saves it, and nothing is
 * stored until they do. Each preset keeps to what its base role holds,
 * so a preset never offers a box the server would refuse.
 */
import type { Permission, RoleDraft } from "@/lib/api/roles";

export interface Preset {
  key: string;
  label: string;
  draft: RoleDraft;
}

const alwaysHeld = "members:read";

const supportAgent = [
  "conversations:read",
  "conversations:write",
  "conversations:assign",
  "contacts:read",
  "contacts:write",
  "articles:read",
  "notifications:read",
  "notifications:write",
  "email_signatures:manage_own",
  "ai:read",
  "ai:write",
];

const billingAdmin = ["billing:read", "billing:write", "settings:read"];

/*
 * withinBase keeps the permissions a custom role of the base role may
 * hold, always with the one every role keeps.
 */
export function withinBase(keys: string[], base: RoleDraft["base_role"], catalog: Permission[]): string[] {
  const allowed = new Set(
    catalog.filter((p) => p.assignable && p.base_roles.includes(base)).map((permission) => permission.key),
  );

  return [...new Set([alwaysHeld, ...keys])].filter((key) => allowed.has(key)).sort();
}

/*
 * presets returns the starting points, shaped by the catalog the server
 * offers: a support agent who works conversations without managing the
 * workspace, a reader who changes nothing, and a billing admin.
 */
export function presets(catalog: Permission[]): Preset[] {
  const reads = catalog.filter((p) => p.key.endsWith(":read")).map((permission) => permission.key);

  return [
    {
      key: "support",
      label: "Support agent",
      draft: {
        name: "Support agent",
        description: "Works conversations and contacts.",
        base_role: "member",
        permissions: withinBase(supportAgent, "member", catalog),
      },
    },
    {
      key: "read-only",
      label: "Read only",
      draft: {
        name: "Read only",
        description: "Sees everything a member sees and changes nothing.",
        base_role: "member",
        permissions: withinBase(reads, "member", catalog),
      },
    },
    {
      key: "billing",
      label: "Billing admin",
      draft: {
        name: "Billing admin",
        description: "Looks after the plan and invoices.",
        base_role: "admin",
        permissions: withinBase(billingAdmin, "admin", catalog),
      },
    },
  ];
}
