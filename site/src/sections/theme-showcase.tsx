/*
 * Help center themes: a real help center wearing the Retro 95 theme
 * (site/shots/themes/retro). The shot is 3840 pixels wide, so it stays
 * sharp on any screen.
 */
import { Intro, Panel, Section } from "../components/container";

export function ThemeShowcase() {
  return (
    <Section frameClassName="flex flex-col gap-14">
      <Intro
        title="Make your help center look nothing like ours."
        body="Redesign every page, from articles to feature requests and your changelog. We turned one into a 1995 desktop to show how far you can take it."
      />
      <Panel className="p-3 sm:p-5">
        <img
          src="/shots/theme-retro-home.webp"
          alt="A dimmi help center redesigned as a Windows 95 desktop, with a teal background, grey windows, folders for each collection and a scrolling banner"
          width={3840}
          height={2400}
          loading="lazy"
          decoding="async"
          className="block aspect-[8/5] w-full rounded-md border border-separator-border object-cover object-top"
        />
      </Panel>
    </Section>
  );
}
