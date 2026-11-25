/*
 * Signing in with an account people already have is the quickest way
 * in, so every page that signs people in offers it first: one button per
 * identity provider the server offers, above the email form. Each is a
 * plain link to the server's redirect, which takes the browser to the
 * provider and back, landing on the page the person was headed to.
 */
import type { MouseEvent } from "react";

import { Divider } from "@/components/base/divider/divider";
import { SocialButton } from "@/components/base/social-button/social-button";
import type { SocialProvider } from "@/components/base/social-button/social-providers";
import type { Provider } from "@/lib/api/auth";

import type { Agreement } from "./use-agreement";

const brands: Record<string, SocialProvider> = { github: "github", google: "google" };

/*
 * How each provider's button looks. Google's sign-in branding asks for
 * its full-colour G on a plain light or dark button, never on a
 * brand-coloured fill: the outlined button does that with our own border
 * and hover colours in both themes.
 */
const appearances: Record<string, "white" | "colorful"> = { google: "white" };

export interface ProviderButtonsProps {
  providers: Provider[];
  redirect: string;
  /** What the buttons do, such as "Continue with" or "Sign up with". */
  action: string;
  /** Names the email form below the buttons, such as "or sign in with email". */
  emailLabel: string;
  /** The token of the invitation the person is answering, carried through the provider's round trip. */
  invitation?: string;
  /** Signs in again on the session the browser already has. */
  reauth?: boolean;
  /** On a page that asks for the legal terms, lets the buttons through only once they are accepted. */
  agreement?: Agreement;
}

/*
 * Start is what the browser carries to the provider and back: where to
 * land, the invitation being answered, and whether the person accepted
 * the legal terms on the page they left.
 */
interface Start {
  redirect: string;
  invitation?: string;
  reauth?: boolean;
  termsAccepted?: boolean;
}

/*
 * startAddress is the server's redirect for the provider, with what the
 * browser carries through the round trip.
 */
function startAddress(provider: Provider, { redirect, invitation, reauth, termsAccepted }: Start): string {
  const query = new URLSearchParams({ redirect });
  if (reauth) {
    query.set("intent", "reauth");
  }
  if (invitation) {
    query.set("invitation", invitation);
  }
  if (termsAccepted) {
    query.set("terms_accepted", "true");
  }

  return `${provider.redirect_url}?${query.toString()}`;
}

export function ProviderButtons({
  providers,
  redirect,
  action,
  emailLabel,
  invitation,
  reauth,
  agreement,
}: ProviderButtonsProps) {
  const known = providers.filter((provider) => brands[provider.key]);
  if (known.length === 0) {
    return null;
  }

  const termsAccepted = Boolean(agreement?.terms && agreement.accepted);
  const hold = (event: MouseEvent) => {
    if (agreement && !agreement.confirm()) {
      event.preventDefault();
    }
  };

  return (
    <div className="mt-6 flex flex-col gap-2.5">
      {known.map((provider) => (
        <SocialButton
          key={provider.key}
          brand={brands[provider.key] as SocialProvider}
          appearance={appearances[provider.key] ?? "colorful"}
          fullWidth
          href={startAddress(provider, { redirect, invitation, reauth, termsAccepted })}
          onClick={hold}
        >
          {`${action} ${provider.name}`}
        </SocialButton>
      ))}
      <div className="mt-2.5">
        <Divider>{emailLabel}</Divider>
      </div>
    </div>
  );
}
