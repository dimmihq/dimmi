/*
 * Signing in with an account people already have is the quickest way
 * in, so every page that signs people in offers it first: one button per
 * identity provider the server offers, above the email form. Each is a
 * plain link to the server's redirect, which takes the browser to the
 * provider and back, landing on the page the person was headed to.
 */
import { Divider } from "@/components/base/divider/divider";
import { SocialButton } from "@/components/base/social-button/social-button";
import type { SocialProvider } from "@/components/base/social-button/social-providers";
import type { Provider } from "@/lib/api/auth";

const brands: Record<string, SocialProvider> = { github: "github", google: "google" };

export interface ProviderButtonsProps {
  providers: Provider[];
  redirect: string;
  /** What the buttons do, such as "Continue with" or "Sign up with". */
  action: string;
  /** Names the email form below the buttons, such as "or sign in with email". */
  emailLabel: string;
}

export function ProviderButtons({ providers, redirect, action, emailLabel }: ProviderButtonsProps) {
  const known = providers.filter((provider) => brands[provider.key]);
  if (known.length === 0) {
    return null;
  }

  return (
    <div className="mt-6 flex flex-col gap-2.5">
      {known.map((provider) => (
        <SocialButton
          key={provider.key}
          brand={brands[provider.key] as SocialProvider}
          appearance="colorful"
          fullWidth
          href={`${provider.redirect_url}?redirect=${encodeURIComponent(redirect)}`}
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
