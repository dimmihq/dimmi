/*
 * The frame every sign-in page shares, laid out like BoardUI's auth
 * card: a bordered card with a title and description, and small print
 * below.
 */
import type { ReactNode } from "react";

export function AuthCard({ children }: { children: ReactNode }) {
  return (
    <div className="flex w-full max-w-[400px] flex-col rounded-3xl border border-border-button-default bg-background-primary-default p-6 shadow-xs sm:p-8 dark:bg-background-secondary-default">
      {children}
    </div>
  );
}

export function AuthHeader({ title, description }: { title: ReactNode; description?: ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <h1 className="text-title-2-medium text-text-primary">{title}</h1>
      {description ? <p className="text-body-regular text-text-secondary">{description}</p> : null}
    </div>
  );
}

export function AuthFooter({ children }: { children: ReactNode }) {
  return <p className="mt-6 text-center text-body-regular text-text-secondary">{children}</p>;
}
