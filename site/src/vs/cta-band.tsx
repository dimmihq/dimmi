/*
 * Closing CTA wrapper for /vs pages forwarding custom title and links to FinalCta.
 */
import { FinalCta, type FinalCtaLink } from "../sections/final-cta";

export function CtaBand({ title, links }: { title: string; links?: FinalCtaLink[] }) {
  return <FinalCta title={title} links={links} />;
}
