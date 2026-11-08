# Help center sign-in: how competitors let readers see restricted articles

Research date: 2026-11-08. Public docs and community forums only; no accounts tested. Input for the help center sign-in in `specs/18-audiences`.

## Intercom

- The help center has no login of its own. It recognises a reader through the Messenger's session: with a custom domain on the same root domain as the product and "cookie forwarding", the session cookie the Messenger set on the product is readable on the help center ([public-faq], [control]).
- Requirements: custom domain, cookie forwarding, the Messenger installed for logged-in users on the product, a login link under Help Center privacy settings ([control], [public-faq]).
- The login link is only where a logged-out reader is sent to log in to the product. There is no token handed back. Users report the reader is not returned to the article afterwards ([cookie-thread], [restricted-thread]).
- A whole help center cannot be private. Audience is per article, not per collection. Fin has a separate audience setting ([control]).
- A third-party login (another provider's site) is "not directly supported" per a community answer, Feb 2025 ([third-party]).

## Zendesk

- The help center has its own end-user accounts. Readers self-register: name and email, then a welcome email to verify the address and set a password. Access can be open, closed (only users the team adds) or restricted to email domains ([zd-access]).
- Also: social sign-in (Google, Microsoft, Facebook, X) and SSO with JWT, SAML or OIDC, several configurations at once, with "let them choose" or "redirect to SSO" ([zd-sso]).
- Restrict the whole help center ("Require sign in") or articles by user segment built from tags, organizations, groups ([zd-restrict]).
- A help center sign-in carries into the messaging widget. A widget JWT does not sign the reader in to the help center ([zd-widget-hc]).

## Help Scout

- Restricted Docs works only through a custom callback, configured through the API only, on Plus or Pro ([hs-dev], [hs-help]).
- Flow: Docs redirects to the team's sign-in URL with `returnTo`; the team's endpoint authenticates the visitor and redirects to `/authcallback?token=<HS512 JWT>&returnTo=...`; Docs verifies and lets them in. Session length is the token's `exp` ([hs-dev]).
- No built-in login for visitors ([hs-dev]).

## GitBook

- Visitor authentication through Auth0, Azure AD, Okta, AWS Cognito, OIDC, or a custom backend signing a token with the site's private key ([gb]).

## Featurebase

- Private organization: access by company email domain or invitation, Microsoft Entra ID, or SSO (redirect to the team's login page). If SSO is on, other methods are off ([fb-auth], [fb-private]).
- Readers coming from the team's app via the SDK are signed in automatically, but the SDK route does not work for private organizations ([fb-auth]).
- Default reader login otherwise: Google, GitHub, Discord, or email and password ([fb-auth]).
- Per-article restrictions only; "no plans" for collections ([fb-privacy]).

## Where dimmi's first build (f10f58ab) stands

What shipped is the Help Scout / GitBook "custom backend" pattern: login URL with a return address, the team's product signs a JWT, the help center verifies it and sets a one-hour cookie. Differences: the token travels in the fragment, not the query; the messenger's identity secrets are reused; collections can be restricted.

Gaps against the market:

1. No path without engineering. Zendesk and Featurebase let readers sign in with an email (and password or social); Intercom needs only DNS and the Messenger it already has.
2. No reuse of the Messenger's identity, which is Intercom's whole approach. A reader identified in the messenger is anonymous on the help center.
3. No standard identity providers (OIDC, SAML) as GitBook and Zendesk offer.

Second build: email codes (on by default, the messenger's limits) close gap 1; a one-use pass from the messenger, carried in the fragment, closes gap 2 on any domain, where Intercom needs a custom domain and cookie forwarding. Gap 3, social sign-in and requiring sign-in for a whole help center remain.

[control]: https://www.intercom.com/help/en/articles/2982784-control-who-can-see-your-public-articles
[public-faq]: https://www.intercom.com/help/en/articles/8827044-public-articles-faqs
[cookie-thread]: https://community.intercom.com/settings-security-permissions-22/cookie-forwarding-10198
[restricted-thread]: https://community.intercom.com/knowledge-base-6/how-can-users-access-restricted-articles-in-help-center-421
[third-party]: https://community.intercom.com/article-help-center-6/using-a-third-party-login-for-restricted-content-in-fin-help-center-9111
[zd-access]: https://support.zendesk.com/hc/en-us/articles/4408887573274-Understanding-options-for-end-user-access-and-sign-in
[zd-sso]: https://support.zendesk.com/hc/en-us/articles/5380943678106-Giving-users-different-ways-to-sign-into-Zendesk
[zd-restrict]: https://support.zendesk.com/hc/en-us/articles/4408842656154
[zd-widget-hc]: https://developer.zendesk.com/documentation/conversations/messaging-platform/users/authenticating-users-help-center/
[hs-dev]: https://developer.helpscout.com/docs-api/restricted-docs/
[hs-help]: https://docs.helpscout.com/article/1497-restricted-docs
[gb]: https://gitbook.com/docs/publishing-documentation/visitor-authentication/enabling-visitor-authentication
[fb-auth]: https://help.featurebase.app/articles/8933417-user-authentication-methods
[fb-private]: https://help.featurebase.app/en/articles/6416894-public-vs-private-portal
[fb-privacy]: https://help.featurebase.app/articles/6624453-help-center-privacy-controls
