/*
 * How available the signed-in person is for new conversations. Three
 * states reach the screen: active, away, and away while handing what
 * customers answer back to the team. The server keeps the truth; the
 * menu only shows what the caller query last read.
 */
import { request } from "./client";
import type { Caller } from "./workspace";

export type AvailabilityState = "active" | "away" | "away_reassign";

/*
 * availabilityOf reads the three-way state from the caller.
 */
export function availabilityOf(member: Caller["member"]): AvailabilityState {
  if (!member || member.availability === "online") {
    return "active";
  }
  return member.away_reassign ? "away_reassign" : "away";
}

/*
 * setAvailability changes the caller's own availability in the
 * workspace.
 */
export function setAvailability(workspace: string, state: AvailabilityState): Promise<unknown> {
  const body =
    state === "active"
      ? { availability: "online" }
      : { availability: "away", away_reassign: state === "away_reassign" };
  return request<unknown>("PATCH", "/members/me", body, { workspace });
}
