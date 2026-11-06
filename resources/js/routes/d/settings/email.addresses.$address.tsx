import { createFileRoute } from "@tanstack/react-router";

import { AddressPage } from "@/features/email/setup/address-page";

export const Route = createFileRoute("/d/settings/email/addresses/$address")({
  component: AddressRoute,
});

function AddressRoute() {
  const { address } = Route.useParams();

  return <AddressPage key={address} address={address} />;
}
