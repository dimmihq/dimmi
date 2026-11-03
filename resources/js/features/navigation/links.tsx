/*
 * BoardUI's link and button links, able to move inside the app. BoardUI
 * renders them as ordinary anchors, which the browser follows with a full
 * page load. Each component here picks its element from its props: given
 * `to`, it renders the router's Link, which changes page inside the app,
 * keeps the app running and plays the page transition; given `href`, it
 * renders BoardUI's plain anchor, for addresses the app does not draw,
 * such as the social sign-in redirects. Either way the styling is
 * BoardUI's.
 */
import { createLink } from "@tanstack/react-router";
import type { ComponentProps } from "react";

import {
  ButtonLink as AnchorButtonLink,
  type ButtonLinkProps as AnchorButtonLinkProps,
} from "@/components/base/buttons/button";
import {
  LinkButton as AnchorLinkButton,
  type LinkButtonProps as AnchorLinkButtonProps,
  type LinkButtonAnchorProps,
} from "@/components/base/buttons/link-button";

function LinkButtonAnchor({ href = "", ...props }: Omit<LinkButtonAnchorProps, "href"> & { href?: string }) {
  return <AnchorLinkButton href={href} {...props} />;
}

function ButtonLinkAnchor(props: AnchorButtonLinkProps) {
  return <AnchorButtonLink {...props} />;
}

const RouterLinkButton = createLink(LinkButtonAnchor);
const RouterButtonLink = createLink(ButtonLinkAnchor);

type RouterLinkButtonProps = ComponentProps<typeof RouterLinkButton>;
type RouterButtonLinkProps = ComponentProps<typeof RouterButtonLink>;

/*
 * LinkButton is BoardUI's inline text link. With `to` it opens a page of
 * the app through the router; with `href` it is a plain anchor; with
 * neither it is a button.
 */
export function LinkButton(props: RouterLinkButtonProps | AnchorLinkButtonProps) {
  if ("to" in props && props.to !== undefined) {
    return <RouterLinkButton {...props} />;
  }

  return <AnchorLinkButton {...(props as AnchorLinkButtonProps)} />;
}

/*
 * ButtonLink is BoardUI's link drawn as a button. With `to` it opens a
 * page of the app through the router; with `href` it is a plain anchor.
 */
export function ButtonLink(props: RouterButtonLinkProps | AnchorButtonLinkProps) {
  if ("to" in props && props.to !== undefined) {
    return <RouterButtonLink {...props} />;
  }

  return <AnchorButtonLink {...(props as AnchorButtonLinkProps)} />;
}
