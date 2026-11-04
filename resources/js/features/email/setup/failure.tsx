import { ApiError } from "@/lib/api/client";

/*
 * Failure shows what went wrong with the last change, or nothing.
 */
export function Failure({ error }: { error: Error | null }) {
  if (!error) {
    return null;
  }

  return (
    <p role="alert" className="px-3 text-body-2-regular text-text-error-primary">
      {error instanceof ApiError ? (Object.values(error.fields)[0] ?? error.message) : error.message}
    </p>
  );
}
