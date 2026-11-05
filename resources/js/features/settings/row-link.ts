/*
 * The class of a list item's title that opens the item's own page: the
 * link's hit area stretches over the whole row, so the row answers the
 * click while the controls beside it keep their own.
 */

/*
 * rowLink is the class of such a title.
 */
export const rowLink =
  "outline-none after:absolute after:inset-y-0 after:-start-3 after:end-0 after:content-[''] hover:underline focus-visible:after:ring-2 focus-visible:after:ring-border-focus-ring";
