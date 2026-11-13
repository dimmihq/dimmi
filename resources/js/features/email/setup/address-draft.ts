/*
 * The draft of an address's settings: what the controls show, and the
 * changes held for the page's save bar until they are sent or discarded.
 */
import { useState } from "react";

import { type EmailAddress, updateEmailAddress } from "@/lib/api/email";

type AddressSettings = EmailAddress["settings"];

/*
 * fallbackColor is the brand color shown before one is picked.
 */
const fallbackColor = "#171717";

/*
 * savedSettings reads an address's settings as the controls show them,
 * filling a reply style never picked with its defaults.
 */
export function savedSettings(settings: AddressSettings): AddressSettings {
  return {
    ...settings,
    template: settings.template ?? "plain",
    brand_color: settings.brand_color || fallbackColor,
    footer_markdown: settings.footer_markdown ?? "",
  };
}

/*
 * useAddressDraft holds the address's settings changed but not saved
 * yet. A setting put back to its saved value drops out of the draft.
 * send writes what changed in one call; clear forgets it after a save or
 * on Discard. generation counts the clears, so controls that keep their
 * own value, such as the color well, start over from the saved one.
 */
export function useAddressDraft(workspace: string, address: EmailAddress | undefined) {
  const [pending, setPending] = useState<Partial<AddressSettings>>({});
  const [generation, setGeneration] = useState(0);
  const saved = address ? savedSettings(address.settings) : undefined;
  const dirty = Object.keys(pending).length > 0;

  return {
    settings: saved ? { ...saved, ...pending } : undefined,
    generation,
    dirty,
    set: <K extends keyof AddressSettings>(key: K, value: AddressSettings[K]) =>
      setPending(({ [key]: _, ...rest }) => (saved && saved[key] === value ? rest : { ...rest, [key]: value })),
    send: () => (address && dirty ? updateEmailAddress(workspace, address.id, { settings: pending }) : null),
    clear: () => {
      setPending({});
      setGeneration((count) => count + 1);
    },
  };
}

export type AddressDraft = ReturnType<typeof useAddressDraft>;
