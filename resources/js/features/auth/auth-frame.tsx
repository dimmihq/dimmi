/*
 * The frame every sign-in page shares, laid out like BoardUI's auth
 * card: a bordered card that opens with the Dimmi icon above a title and
 * description, and small print below. A page that reports where things
 * stand, such as a mail on its way or a link that expired, swaps the
 * whole card for a status screen, as BoardUI's verify step does, rather
 * than boxing a notice inside the card.
 */
import type { ComponentType, ReactNode } from "react";
import { DimmiIcon } from "@/features/brand/logo";
import { cx } from "@/utils/cx";

export function AuthCard({ children }: { children: ReactNode }) {
  return (
    <div className="flex w-full max-w-100 flex-col rounded-3xl border border-border-button-default bg-background-primary-default p-6 shadow-xs sm:p-8 dark:bg-background-secondary-default">
      {children}
    </div>
  );
}

export function AuthHeader({ title, description }: { title: ReactNode; description?: ReactNode }) {
  return (
    <>
      <div className="mb-5 flex justify-start">
        <DimmiIcon className="size-10 text-text-primary" />
      </div>
      <div className="flex flex-col gap-1.5">
        <h1 className="text-title-2-medium text-text-primary">{title}</h1>
        {description ? <p className="text-body-regular text-text-secondary">{description}</p> : null}
      </div>
    </>
  );
}

export function AuthFooter({ children }: { children: ReactNode }) {
  return <p className="mt-6 text-center text-body-regular text-text-secondary">{children}</p>;
}

export interface AuthStatusProps {
  /** The state at a glance, shown where the logo sits. */
  icon: ComponentType<{ className?: string }>;
  /** Failures draw the icon in the error colour; everything else in the primary one. */
  tone?: "neutral" | "error";
  title: ReactNode;
  description: ReactNode;
  /** What to do next: buttons under the text, then the footer. */
  children?: ReactNode;
}

/*
 * AuthStatus is a card that reports a state instead of asking for input:
 * an icon, a heading saying what happened, the explanation at full
 * width, and what to do next.
 */
export function AuthStatus({ icon: Icon, tone = "neutral", title, description, children }: AuthStatusProps) {
  return (
    <AuthCard>
      <div className="mb-5 flex justify-start" aria-hidden>
        <Icon
          className={cx("size-8", tone === "error" ? "text-foreground-icon-error" : "text-foreground-icon-primary")}
        />
      </div>
      <div className="flex flex-col gap-1.5" role={tone === "error" ? "alert" : "status"}>
        <h1 className="text-title-2-medium text-text-primary">{title}</h1>
        <p className="text-body-regular text-text-secondary">{description}</p>
      </div>
      {children}
    </AuthCard>
  );
}
