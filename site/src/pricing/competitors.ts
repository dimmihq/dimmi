/*
 * Re-exports competitor pricing models and helpers from `site/src/data/rivals.ts`
 * so `/pricing` and every `/vs/*` page share one source of truth.
 */
export {
  CHECKED_ON,
  CHECKED_ON_ISO,
  COMPETITORS,
  type Competitor,
  competitorBill,
  fullBill,
  RESOLUTION_RATE,
  SCENARIOS,
} from "../data/rivals";
