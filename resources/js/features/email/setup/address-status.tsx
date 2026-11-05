/*
 * Whether mail reached an address yet, and whether it is the address
 * new mail leaves from, as small chips beside the address.
 */
import { Chip } from "@/components/base/badges/chip";
import type { EmailAddress } from "@/lib/api/email";

/*
 * AddressStatus says whether mail reached the address yet.
 */
export function AddressStatus({ address }: { address: EmailAddress }) {
  return (
    <span className="flex items-center gap-2">
      {address.is_default ? (
        <Chip variant="caption" color="neutral">
          Default
        </Chip>
      ) : null}
      {address.inbound_verified_at ? (
        <Chip variant="caption" color="lime">
          Receiving
        </Chip>
      ) : (
        <Chip variant="caption" color="neutral">
          Waiting for first email
        </Chip>
      )}
    </span>
  );
}
