import { RiArrowRightLine, RiGithubFill } from "@remixicon/react";

import { ButtonLink } from "@/components/base/buttons/button";

import { Section } from "../components/container";
import { APP_URL, GITHUB_URL } from "../links";
import { track } from "../track";

/* Rays fanning up from the bottom edge, each ending in a small node. */
const rays = Array.from({ length: 34 }, (_, i) => {
  const angle = Math.PI * (0.06 + (0.88 * i) / 33);
  const reach = 150 + ((i * 97) % 170);
  return {
    id: `ray-${i}`,
    x: 600 - Math.cos(angle) * reach * 2.4,
    y: 300 - Math.sin(angle) * reach,
    bright: i % 5 === 2,
  };
});

export function FinalCta() {
  return (
    <Section tone="brand" flush frameClassName="overflow-hidden">
      <div className="relative flex flex-col items-center gap-8 px-5 pt-24 pb-56 text-center sm:px-12 md:pt-28">
        <h2 className="max-w-xl text-[40px] leading-[1.12] font-medium tracking-[-0.025em] text-balance text-text-primary max-sm:text-[30px]">
          Try dimmi free on our cloud, or self-host it and pay us $0.
        </h2>
        <div className="flex flex-wrap justify-center gap-3">
          <ButtonLink
            href={`${APP_URL}/signup`}
            trailingIcon={RiArrowRightLine}
            onClick={() => track("cta_start_free", { location: "final_cta" })}
          >
            Start free, no card
          </ButtonLink>
          <ButtonLink
            href={GITHUB_URL}
            variant="secondary"
            leadingIcon={RiGithubFill}
            onClick={() => track("github_click", { location: "final_cta" })}
          >
            Self-host on GitHub
          </ButtonLink>
        </div>
      </div>
      <svg
        viewBox="0 0 1200 300"
        preserveAspectRatio="xMidYMax slice"
        className="pointer-events-none absolute inset-x-0 bottom-0 h-56 w-full"
        aria-hidden
      >
        {rays.map((ray) => (
          <g key={ray.id}>
            <line
              x1="600"
              y1="420"
              x2={ray.x}
              y2={ray.y}
              className={ray.bright ? "stroke-accent-400" : "stroke-white"}
              strokeOpacity={ray.bright ? 0.55 : 0.14}
              strokeWidth="1"
            />
            <circle
              cx={ray.x}
              cy={ray.y}
              r={ray.bright ? 2.4 : 1.6}
              className={ray.bright ? "fill-accent-400" : "fill-white"}
              fillOpacity={ray.bright ? 1 : 0.5}
            />
          </g>
        ))}
      </svg>
    </Section>
  );
}
