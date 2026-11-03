/*
 * The team's faces in an overlapping row, initials until members have
 * avatars.
 */
import type { Member } from "../content";
import { initials } from "../text";

export function Faces({ team, className = "" }: { team: Member[]; className?: string }) {
  return (
    <span className={`faces ${className}`.trim()}>
      {team.map((member) => (
        <span key={member.name}>{initials(member.name)}</span>
      ))}
    </span>
  );
}
