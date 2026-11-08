/*
 * Who an audience matches right now, refreshed a moment after each edit:
 * how many people, how many more match only on some pages, a few of them
 * by name, and the mistakes the rule probably holds. A count read from a
 * sample of a large workspace says so.
 */
import { Link } from "@tanstack/react-router";

import { Skeleton } from "@/components/base/skeleton/skeleton";
import type { AudiencePreview } from "@/lib/api/audiences";

import { personName } from "../customers/names";

function count(n: number): string {
  return n.toLocaleString();
}

function headline(p: AudiencePreview): string {
  const people = p.count === 1 ? "1 person" : `${count(p.count)} people`;
  const about = p.approximate ? "About " : "";

  return `${about}${people}`;
}

export function PreviewPane({
  preview,
  loading,
  error,
}: {
  preview: AudiencePreview | undefined;
  loading: boolean;
  error: string | null;
}) {
  if (error) {
    return <p className="text-body-2-regular text-text-error-primary">{error}</p>;
  }
  if (!preview) {
    return (
      <div className="flex flex-col gap-2" aria-busy={loading}>
        <Skeleton shape="text" className="h-6 w-32" />
        <Skeleton shape="text" className="h-3.5 w-48" />
      </div>
    );
  }
  const share = preview.total_contacts > 0 ? Math.round((preview.count / preview.total_contacts) * 100) : 0;

  return (
    <div className="flex flex-col gap-4" aria-busy={loading}>
      <div className="flex flex-col gap-0.5">
        <p className="text-title-3-semibold text-text-primary" aria-live="polite">
          {headline(preview)}
        </p>
        <p className="text-body-2-regular text-text-secondary">
          {share}% of {count(preview.total_contacts)} people
          {preview.approximate ? ", counted from a sample" : ""}
        </p>
        {preview.pages > 0 ? (
          <p className="text-body-2-regular text-text-secondary">
            {count(preview.pages)} more match on some pages only.
          </p>
        ) : null}
      </div>
      {preview.warnings.length > 0 ? (
        <ul className="flex flex-col gap-2">
          {preview.warnings.map((w) => (
            <li
              key={`${w.code}-${w.path}`}
              className="rounded-xl border border-separator-border px-3 py-2 text-body-2-regular text-text-secondary"
            >
              {w.message}
            </li>
          ))}
        </ul>
      ) : null}
      {preview.sample.length > 0 ? (
        <div className="flex flex-col gap-1">
          <p className="text-body-2-medium text-text-secondary">Some of them</p>
          <ul className="flex flex-col">
            {preview.sample.map((person) => (
              <li key={person.id}>
                <Link
                  to="/d/customers/$contact"
                  params={{ contact: person.id }}
                  className="block truncate rounded-lg px-2 py-1.5 text-body-2-regular text-text-primary outline-none hover:bg-background-secondary-hover focus-visible:ring-2 focus-visible:ring-border-focus-ring"
                >
                  {personName(person)}
                </Link>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
