/*
 * Four concrete team profiles with an honest pick (dimmi or rival), a reason,
 * and monthly bill numbers computed live from `prices.ts` and `rivals.ts`.
 */
import { cx } from "@/utils/cx";

import { Section, SplitHead } from "../components/container";
import { competitorBill, type Rival, type RivalTeamPick } from "../data/rivals";
import { bill, dollars } from "../pricing/prices";

export function TeamPicker({
  rival,
  tag = "Which team picks which",
  title,
  body = "",
  teams,
  rivalMonthlyForTeam,
}: {
  rival: Rival;
  tag?: string;
  title: string;
  body?: string;
  teams: RivalTeamPick[];
  rivalMonthlyForTeam?: (team: RivalTeamPick) => number;
}) {
  const computeRivalBill = (team: RivalTeamPick): number | null => {
    if (rivalMonthlyForTeam) {
      return rivalMonthlyForTeam(team);
    }
    if (!rival.competitor) {
      return null;
    }
    return competitorBill(rival.competitor, {
      conversations: team.conversations,
      aiShare: team.aiShare,
      operator: 0,
      ownModel: false,
      seats: team.seats,
    });
  };

  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead tag={tag} title={title} body={body} />
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
        {teams.map((team) => {
          const dimmiCost = bill({
            conversations: team.conversations,
            aiShare: team.aiShare,
            operator: 0,
            ownModel: false,
          }).total;
          const rivalCost = computeRivalBill(team);
          const pickDimmi = team.pick === "dimmi";

          return (
            <div
              key={team.profile}
              className={cx(
                "flex flex-col justify-between gap-4 rounded-2xl border p-6",
                pickDimmi
                  ? "border-accent-500/35 bg-accent-500/[0.03]"
                  : "border-border-button-default bg-background-full",
              )}
            >
              <div className="flex flex-col gap-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <h3 className="text-[17px] leading-6 font-medium text-text-primary">{team.profile}</h3>
                  <span
                    className={cx(
                      "rounded-full px-2.5 py-0.5 font-mono text-[11px] font-medium tracking-[0.06em] uppercase",
                      pickDimmi
                        ? "bg-accent-500/15 text-accent-700 dark:text-accent-300"
                        : "bg-background-secondary-default text-text-secondary",
                    )}
                  >
                    Pick: {pickDimmi ? "dimmi" : rival.name}
                  </span>
                </div>
                <p className="text-[14px] leading-6 text-text-secondary">{team.reason}</p>
              </div>

              <div className="flex flex-wrap items-center gap-x-5 gap-y-1 border-t border-separator-border pt-3 font-mono text-[12px] text-text-tertiary tabular-nums">
                <span>
                  dimmi Cloud:{" "}
                  <strong className="font-medium text-text-primary">{dollars(Math.round(dimmiCost))}/mo</strong>
                </span>
                {rivalCost != null && (
                  <span>
                    {team.rivalPlanLabel}:{" "}
                    <strong className="font-medium text-text-primary">{dollars(Math.round(rivalCost))}/mo</strong>
                  </span>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </Section>
  );
}
