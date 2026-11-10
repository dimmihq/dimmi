# Slack Marketplace submission

The cloud edition ships one shared Slack app to every customer. Until Slack approves it for the Marketplace, a
commercially distributed app can call `conversations.history` and `conversations.replies` only once a minute,
15 messages at a time. Submit at the first staging deploy, with a working install: review takes weeks.

Self-hosted installs create their own app from the generated manifest. That is an internal app, exempt from the
throttle and from review.

## Before submitting

- [ ] Staging runs the cloud binary with `DIMMI_SLACK_CLIENT_ID`, `DIMMI_SLACK_CLIENT_SECRET`,
      `DIMMI_SLACK_SIGNING_SECRET` and `DIMMI_SLACK_APP_ID` set.
- [ ] The app's settings match `Manifest` in `app/drivers/channel/slack/manifest.go`: the same bot scopes, user
      scopes, events and "Track in dimmi" shortcut; event and interactivity request URLs on the staging host;
      redirect URL `<host>/integrations/slack/callback`; token rotation off; org-wide deploy off.
- [ ] Public distribution is turned on, and the install, deny, reconnect and uninstall flows work on staging.
- [ ] A reviewer workspace and a dimmi test account exist for Slack's review team, with sign-in steps.
- [ ] Privacy policy, terms, support page and an install landing page are live on the marketing site.

## Listing copy

- **Name:** dimmi
- **Short description:** Support your customers in Slack, with an AI agent that answers from your help center.
- **Long description:**
  - Turn shared and community channels into a support inbox: every customer question becomes a conversation.
  - Teammates reply from dimmi or straight from Slack, as themselves.
  - dimmi's AI agent answers from your help center and cites its sources. It hands off to a person when it
    cannot help.
  - Triage cards in an internal channel, so the team sees new requests and takes them in one click.
  - Ask Operator, dimmi's teammate assistant, about any conversation in a DM or an internal channel.
  - Get notifications and digests in channels and DMs.
- **Categories:** Customer Support, Communication.
- **Pricing:** paid plans, free trial.

## Why each scope is needed

| Scope | Why |
|---|---|
| `channels:history`, `groups:history`, `im:history` | Read customer messages in the channels and DMs the workspace connects |
| `channels:read`, `groups:read`, `im:read` | List channels to connect and follow renames, archives and Slack Connect sharing |
| `channels:join` | Join a channel an admin picks in dimmi, without a manual `/invite` |
| `chat:write`, `chat:write.public` | Post replies, triage cards and notifications |
| `chat:write.customize` | Show a teammate's name and avatar when they have not linked their own Slack user |
| `im:write` | Send notifications and Operator answers to a teammate's DM |
| `files:read`, `files:write` | Bring customer attachments into the conversation, and send teammates' attachments back |
| `users:read`, `users:read.email` | Know who wrote, and link Slack users to dimmi teammates and contacts by email |
| `reactions:read`, `reactions:write` | Emoji actions (close, take) and the 👀 and ✅ markers |
| `team:read` | Name the connected Slack workspace in dimmi |
| `app_mentions:read` | Answer `@dimmi` mentions in internal channels |
| `commands` | The "Track in dimmi" message shortcut |
| `assistant:write` | Operator in Slack's assistant panel |
| User `chat:write`, `reactions:write` | Asked only when a teammate links their own Slack user, so their replies post as them |

## Data handling statements

- **What is stored:** only Slack messages that become conversation messages, the channel's name and settings, and
  the authors' profile (name, avatar, email). The raw event Slack delivers is kept 7 days once processed, and 30
  days when it failed, then deleted.
- **Deletion:** a message deleted in Slack stays in the conversation, marked as deleted by the customer, as part of
  the support record. Erasing a contact removes their messages and stored events. Deleting a dimmi workspace
  revokes the Slack tokens and deletes all Slack data.
- **Uninstall:** uninstalling the app in Slack, or disconnecting it in dimmi, removes the tokens at once and stops
  all access.
- **AI and training:** Slack data is never used to train models. When the AI agent answers, the conversation is
  sent to the workspace's chosen model provider, which is bound by its API terms not to train on it.
- **Security:** tokens are encrypted per workspace at rest. Every Slack request is checked against the signing
  secret.
- **Hosting region:** match the cloud's data region statement in the privacy policy.

## After approval

- [ ] Note the approval date in `specs/23-integrations/README.md`.
- [ ] Check that the history throttle is lifted by fetching a thread whose root predates the bot joining.
