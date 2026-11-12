/*
 * The card that asks a visitor to rate a conversation once it closed:
 * five faces, worst first, then a box for a comment once they picked
 * one, then thanks. The face they picked stays marked, and they can pick
 * another while the ask still takes changes, a week after it went out.
 */

export interface RatingFace {
  score: number;
  face: string;
  label: string;
}

export interface WidgetRating {
  id: string;
  score: number | null;
  comment: string | null;
  editable: boolean;
  faces: RatingFace[];
}

export interface RatingTexts {
  ask: string;
  comment: string;
  send: string;
  thanks: string;
}

export function RatingCard({
  rating,
  texts,
  commented,
}: {
  rating: WidgetRating;
  texts: RatingTexts;
  commented: boolean;
}) {
  const done = rating.score !== null && (commented || rating.comment !== null || !rating.editable);

  return (
    <div className="rating" data-key={`rating-${rating.id}`} data-rating={rating.id}>
      <p>{done ? texts.thanks : texts.ask}</p>
      <div className="faces">
        {rating.faces.map((face) => (
          <button
            key={face.score}
            type="button"
            aria-pressed={String(rating.score === face.score)}
            aria-label={face.label}
            title={face.label}
            data-score={face.score}
            disabled={!rating.editable}
          >
            {face.face}
          </button>
        ))}
      </div>
      {rating.score !== null && !done ? (
        <form className="rating-comment" data-rating-comment>
          <textarea name="comment" rows={2} maxLength={2000} placeholder={texts.comment} aria-label={texts.comment} />
          <button className="button primary" type="submit">
            {texts.send}
          </button>
        </form>
      ) : null}
    </div>
  );
}
