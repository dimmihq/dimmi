import { RiMenuLine } from "@remixicon/react";

import { ThemeToggle } from "@/components/application/theme/theme-toggle";
import { Button, ButtonLink } from "@/components/base/buttons/button";
import { Drawer, DrawerBody, DrawerFooter, DrawerHeader, DrawerTrigger } from "@/components/base/drawer/drawer";
import { cx } from "@/utils/cx";

import { CHANNELS } from "../channels";
import { APP_URL } from "../links";
import { PRODUCT_GROUPS } from "../products";
import { track } from "../track";
import { GitHubStars } from "./github-stars";

const heading = "px-3 pt-4 pb-1 font-mono text-[11px] tracking-[0.14em] text-text-tertiary uppercase";

const row =
  "flex items-center gap-3 rounded-lg px-3 py-2 text-body-medium text-text-primary outline-none transition-colors hover:bg-background-secondary-default focus-visible:bg-background-secondary-default";

/*
 * The navigation on screens too narrow for the bar's links: a menu button
 * that opens a drawer holding every page the bar and its two menus link,
 * the theme switch, and the sign-in and sign-up buttons.
 */
export function SiteNavMobile({
  path,
  links,
}: {
  path: string;
  links: ReadonlyArray<{ label: string; href: string }>;
}) {
  return (
    <DrawerTrigger>
      <Button variant="secondary" size="small" iconOnly leadingIcon={RiMenuLine} aria-label="Open menu" />
      <Drawer placement="end" size="sm">
        {({ close }) => (
          <>
            <DrawerHeader title="Menu" closeLabel="Close menu" />
            <DrawerBody className="px-3 pb-4">
              <nav aria-label="Main" className="flex flex-col">
                {PRODUCT_GROUPS.map((group) => (
                  <div key={group.title} className="flex flex-col">
                    <span className={heading}>{group.title}</span>
                    {group.products.map((product) => (
                      <a
                        key={product.href}
                        href={product.href}
                        onClick={close}
                        aria-current={product.href === path ? "page" : undefined}
                        className={cx(row, product.href === path && "bg-background-secondary-default")}
                      >
                        <product.icon
                          weight="duotone"
                          className="size-5 shrink-0 text-accent-600 dark:text-accent-400"
                          aria-hidden
                        />
                        {product.name}
                      </a>
                    ))}
                  </div>
                ))}
                <span className={heading}>Channels</span>
                {CHANNELS.map((channel) => (
                  <a
                    key={channel.href}
                    href={channel.href}
                    onClick={close}
                    aria-current={channel.href === path ? "page" : undefined}
                    className={cx(row, channel.href === path && "bg-background-secondary-default")}
                  >
                    {channel.name}
                  </a>
                ))}
                <span className={heading}>More</span>
                {links.map((link) => (
                  <a
                    key={link.href}
                    href={link.href}
                    onClick={close}
                    aria-current={link.href === path ? "page" : undefined}
                    className={cx(row, link.href === path && "bg-background-secondary-default")}
                  >
                    {link.label}
                  </a>
                ))}
                <a href="/docs" onClick={close} className={row}>
                  Docs
                </a>
                <GitHubStars variant="mobile" location="mobile_nav" onClick={close} />
              </nav>
            </DrawerBody>
            <DrawerFooter className="flex-col items-stretch gap-3">
              <ThemeToggle appearance="segmented" size="small" className="self-start" />
              <div className="grid grid-cols-2 gap-2">
                <ButtonLink href={`${APP_URL}/login`} variant="secondary">
                  Sign in
                </ButtonLink>
                <ButtonLink
                  href={`${APP_URL}/signup`}
                  onClick={() => {
                    track("cta_start_free", { location: "mobile_nav" });
                    close();
                  }}
                >
                  Start free
                </ButtonLink>
              </div>
            </DrawerFooter>
          </>
        )}
      </Drawer>
    </DrawerTrigger>
  );
}
