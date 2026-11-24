import { RiArrowDownSLine, RiArrowRightLine, RiGithubFill } from "@remixicon/react";
import { useEffect, useId, useRef, useState } from "react";

import { ThemeToggle } from "@/components/application/theme/theme-toggle";
import { ButtonLink } from "@/components/base/buttons/button";
import { DimmiLogo } from "@/features/brand/logo";
import { cx } from "@/utils/cx";

import { APP_URL, GITHUB_URL } from "../links";
import { PRODUCT_GROUPS, PRODUCT_PATHS } from "../products";

const links = [
  { label: "Compare", href: "/compare" },
  { label: "Self-host", href: "/#self-host" },
  { label: "Pricing", href: "/pricing" },
];

const item =
  "rounded-lg px-3 py-1.5 text-body-medium transition-colors hover:bg-background-secondary-default hover:text-text-primary";

// path is the page showing; its link reads as the current one.
export function SiteNav({ path }: { path: string }) {
  return (
    <header className="sticky top-0 z-40 border-b border-separator-border bg-background-full/85 backdrop-blur-md">
      <div className="mx-auto flex h-16 w-full max-w-300 items-center gap-6 border-x border-separator-border px-5 sm:px-12">
        <a href="/" aria-label="dimmi home" className="shrink-0 text-text-primary">
          <DimmiLogo className="h-5.5 w-auto" title="dimmi" />
        </a>
        <nav aria-label="Main" className="hidden min-w-0 items-center gap-1 lg:flex">
          <ProductMenu path={path} />
          {links.map((link) => {
            const current = link.href === path;
            return (
              <a
                key={link.label}
                href={link.href}
                aria-current={current ? "page" : undefined}
                className={cx(
                  item,
                  current ? "bg-background-secondary-default text-text-primary" : "text-text-secondary",
                )}
              >
                {link.label}
              </a>
            );
          })}
        </nav>
        <div className="ms-auto flex items-center gap-2">
          <ThemeToggle appearance="segmented" size="small" className="max-sm:hidden" />
          <a
            href={GITHUB_URL}
            className="hidden items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-body-medium text-text-secondary hover:text-text-primary lg:inline-flex"
          >
            <RiGithubFill className="size-[18px]" aria-hidden />
            GitHub
          </a>
          <ButtonLink href={`${APP_URL}/login`} variant="secondary" size="small" className="hidden sm:inline-flex">
            Sign in
          </ButtonLink>
          <ButtonLink href={`${APP_URL}/signup`} size="small">
            Start free
          </ButtonLink>
        </div>
      </div>
    </header>
  );
}

/*
 * The Product menu: it opens on hover or click and drops a panel of every
 * product, grouped, each with an icon and a line on what it does. The
 * panel is always in the page (hidden while shut) so the prerendered
 * HTML links every product page.
 */
function ProductMenu({ path }: { path: string }) {
  const [open, setOpen] = useState(false);
  const closing = useRef<ReturnType<typeof setTimeout>>(undefined);
  const root = useRef<HTMLDivElement>(null);
  const panelId = useId();
  const current = PRODUCT_PATHS.has(path);

  const show = () => {
    clearTimeout(closing.current);
    setOpen(true);
  };
  // A short grace period lets the pointer cross the gap into the panel.
  const hide = () => {
    clearTimeout(closing.current);
    closing.current = setTimeout(() => setOpen(false), 140);
  };

  // A page swap closes the menu.
  // biome-ignore lint/correctness/useExhaustiveDependencies: path is the trigger, not a value read
  useEffect(() => {
    clearTimeout(closing.current);
    setOpen(false);
  }, [path]);

  // Escape, or a press anywhere outside, closes it.
  useEffect(() => {
    if (!open) {
      return;
    }
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
        root.current?.querySelector("button")?.focus();
      }
    };
    const onPress = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("pointerdown", onPress);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("pointerdown", onPress);
    };
  }, [open]);

  return (
    // Hover and focus on the wrapper only steer the button and panel inside it.
    // biome-ignore lint/a11y/noStaticElementInteractions: the button inside is the control
    <div
      ref={root}
      className="relative"
      onPointerEnter={(event) => event.pointerType === "mouse" && show()}
      onPointerLeave={(event) => event.pointerType === "mouse" && hide()}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setOpen(false);
        }
      }}
    >
      <button
        type="button"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((was) => !was)}
        className={cx(
          item,
          "inline-flex cursor-pointer items-center gap-1 outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring",
          current || open ? "bg-background-secondary-default text-text-primary" : "text-text-secondary",
        )}
      >
        Product
        <RiArrowDownSLine
          className={cx("size-4 transition-transform duration-200", open && "rotate-180")}
          aria-hidden
        />
      </button>
      <div
        id={panelId}
        className={cx(
          "absolute top-full -left-4 pt-3 transition-[opacity,translate,scale,visibility] duration-200 ease-[cubic-bezier(0.22,1,0.36,1)] origin-top-left",
          open ? "visible translate-y-0 scale-100 opacity-100" : "invisible -translate-y-1 scale-[0.98] opacity-0",
        )}
      >
        <span
          aria-hidden
          className="absolute top-[7px] left-11 size-3 rotate-45 rounded-tl-[3px] border-t border-l border-border-button-default bg-background-full"
        />
        <div className="w-[860px] overflow-hidden rounded-2xl border border-border-button-default bg-background-full shadow-[0_24px_60px_-20px_rgb(0_0_0/0.28)]">
          <div className="grid grid-cols-3 gap-x-2 p-3">
            {PRODUCT_GROUPS.map((group) => (
              <div key={group.title} className="flex flex-col gap-1">
                <span className="px-3 pt-2 pb-1 font-mono text-[11px] tracking-[0.14em] text-text-tertiary uppercase">
                  {group.title}
                </span>
                {group.products.map((product) => {
                  const here = product.href === path;
                  return (
                    <a
                      key={product.href}
                      href={product.href}
                      aria-current={here ? "page" : undefined}
                      className={cx(
                        "group flex items-center gap-3 rounded-xl px-3 py-2.5 outline-none transition-colors hover:bg-background-secondary-default focus-visible:bg-background-secondary-default",
                        here && "bg-background-secondary-default",
                      )}
                    >
                      <span className="flex size-10 shrink-0 items-center justify-center rounded-[10px] bg-accent-500/10 ring-1 ring-accent-500/15 ring-inset">
                        <product.icon
                          weight="duotone"
                          className="size-[22px] text-accent-600 dark:text-accent-400"
                          aria-hidden
                        />
                      </span>
                      <span className="flex min-w-0 flex-col">
                        <span className="text-[14px] leading-5 font-medium text-text-primary">{product.name}</span>
                        <span className="truncate text-[13px] leading-5 text-text-tertiary">{product.blurb}</span>
                      </span>
                    </a>
                  );
                })}
              </div>
            ))}
          </div>
          <div className="flex items-center justify-between gap-6 border-t border-separator-border bg-background-secondary-default/60 px-6 py-3.5">
            <span className="text-[13px] leading-5 text-text-secondary">
              Open source under MIT. On our cloud or your own servers.
            </span>
            <a
              href="/#self-host"
              className="inline-flex shrink-0 items-center gap-1 text-[13px] leading-5 font-medium text-text-primary hover:underline underline-offset-4"
            >
              Self-host it
              <RiArrowRightLine className="size-3.5" aria-hidden />
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
