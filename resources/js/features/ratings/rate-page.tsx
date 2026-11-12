/*
 * The page a rating email's faces open, with no sign-in: the link's
 * token names the conversation's ask. The face the customer clicked in
 * the email is kept as soon as the page opens, so one click rates; the
 * page then shows the five faces to change it and a box for a comment.
 * An ask older than a week, or a link that does not work, says so
 * calmly instead of showing an error.
 */
import { RiCheckLine, RiLinkUnlinkM, RiTimeLine } from "@remixicon/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/base/buttons/button";
import { Spinner } from "@/components/base/spinner/spinner";
import { Textarea } from "@/components/base/textarea/textarea";
import { AuthCard, AuthStatus } from "@/features/auth/auth-frame";
import { ApiError } from "@/lib/api/client";
import { type CustomerRating, customerRatingQuery, rateByToken } from "@/lib/api/ratings";
import { cx } from "@/utils/cx";

function Gone() {
  return (
    <AuthStatus
      icon={RiLinkUnlinkM}
      title="This link no longer works"
      description="Open the latest email you got and pick a face from there."
    />
  );
}

function Closed() {
  return (
    <AuthStatus
      icon={RiTimeLine}
      title="This rating is closed"
      description="Ratings can be changed for a week after the conversation closes."
    />
  );
}

function Form({ token, rating, picked }: { token: string; rating: CustomerRating; picked?: number }) {
  const client = useQueryClient();
  const [comment, setComment] = useState(rating.comment ?? "");
  const save = useMutation({
    mutationFn: (answer: { score?: number; comment?: string }) => rateByToken(token, answer),
    onSuccess: (next) => client.setQueryData(customerRatingQuery(token).queryKey, next),
  });
  const kept = useRef(false);
  useEffect(() => {
    if (kept.current || !picked || !rating.editable || rating.score === picked) {
      return;
    }
    kept.current = true;
    save.mutate({ score: picked });
  }, [picked, rating.editable, rating.score, save]);

  if (!rating.editable) {
    return <Closed />;
  }

  const score = save.variables?.score ?? rating.score;

  return (
    <AuthCard>
      <div className="flex flex-col gap-1.5">
        <h1 className="text-title-2-medium text-text-primary">How did {rating.workspace} do?</h1>
        <p className="text-body-regular text-text-secondary">Pick a face. You can change it for a week.</p>
      </div>
      <div className="mt-6 flex justify-between gap-2">
        {rating.faces.map((face) => (
          <button
            key={face.score}
            type="button"
            aria-pressed={score === face.score}
            aria-label={face.label}
            title={face.label}
            onClick={() => save.mutate({ score: face.score })}
            className={cx(
              "flex size-12 items-center justify-center rounded-xl text-[28px] outline-none transition duration-150 focus-visible:ring-2 focus-visible:ring-border-focus-ring",
              score === face.score
                ? "scale-110 bg-background-secondary-hover"
                : score
                  ? "opacity-50 grayscale hover:opacity-100 hover:grayscale-0"
                  : "hover:bg-background-secondary-hover",
            )}
          >
            {face.face}
          </button>
        ))}
      </div>
      {score ? (
        <div className="mt-6 flex flex-col gap-3">
          <Textarea
            label="Anything to add?"
            placeholder="Tell us what went well or what we could do better"
            rows={3}
            autoResize
            maxRows={8}
            value={comment}
            onChange={setComment}
          />
          <div className="flex items-center gap-3">
            <Button onClick={() => save.mutate({ comment })} disabled={save.isPending}>
              Send
            </Button>
            {save.isSuccess ? (
              <span role="status" className="flex items-center gap-1 text-body-2-regular text-text-secondary">
                <RiCheckLine aria-hidden className="size-4 text-foreground-icon-secondary" />
                Thanks for your feedback
              </span>
            ) : null}
          </div>
        </div>
      ) : null}
      {save.isError ? (
        <p role="alert" className="mt-4 text-body-2-regular text-text-error-primary">
          {save.error.message}
        </p>
      ) : null}
    </AuthCard>
  );
}

export function RatePage({ token, picked }: { token: string; picked?: number }) {
  const { data, isPending, error } = useQuery({ ...customerRatingQuery(token), enabled: token !== "" });
  if (!token || (error instanceof ApiError && error.status === 404)) {
    return <Gone />;
  }
  if (error) {
    return <AuthStatus icon={RiLinkUnlinkM} tone="error" title="Something went wrong" description={error.message} />;
  }
  if (isPending || !data) {
    return (
      <AuthCard>
        <Spinner size="sm" aria-label="Loading" />
      </AuthCard>
    );
  }

  return <Form token={token} rating={data} picked={picked} />;
}
