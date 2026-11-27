/*
 * Proof band for real dimmi product captures (ProductShot, ShotStill, ShotVideo,
 * or custom verified evidence blocks) inside the Section rails.
 */
import type { ReactNode } from "react";

import { Section, SplitHead } from "../components/container";

export function ProofBand({
  tag = "Product proof",
  title,
  body = "",
  caption,
  children,
}: {
  tag?: string;
  title: string;
  body?: string;
  caption?: string;
  children: ReactNode;
}) {
  return (
    <Section frameClassName="flex flex-col gap-10">
      <SplitHead tag={tag} title={title} body={body} />
      <div className="flex flex-col gap-3">
        {children}
        {caption ? <p className="text-[13px] leading-5 text-text-tertiary">{caption}</p> : null}
      </div>
    </Section>
  );
}
