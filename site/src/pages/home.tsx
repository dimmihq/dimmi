import { Agent } from "../sections/agent";
import { Automate } from "../sections/automate";
import { FinalCta } from "../sections/final-cta";
import { Hero } from "../sections/hero";
import { Inbox } from "../sections/inbox";
import { Loop } from "../sections/loop";
import { Operator } from "../sections/operator";
import { Proof } from "../sections/proof";
import { SelfHost } from "../sections/self-host";
import { Trust } from "../sections/trust";

export function HomePage() {
  return (
    <>
      <Hero />
      <Proof />
      <Inbox />
      <Agent />
      <Operator />
      <Automate />
      <Loop />
      <SelfHost />
      <Trust />
      <FinalCta />
    </>
  );
}
