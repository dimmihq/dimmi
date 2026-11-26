---
name: dimmi-migrate-intercom
description: Move a team's support history from Intercom into dimmi with the dimmi-migrate CLI, on the person's own computer. Use when someone asks to migrate, move, import or switch from Intercom to dimmi.
---

# Move from Intercom to dimmi

You run `dimmi-migrate` on the person's computer. It reads their Intercom workspace with a token from their own Intercom app and writes into their dimmi workspace. Customer data goes straight from Intercom to dimmi: it never passes through you.

Rules for the whole job:

- **Never ask for a key, token or password in the chat**, and never read one from a file or the keychain. `dimmi-migrate connect` opens a page in their browser where they paste it. If they paste a token into the chat anyway, tell them to revoke it in Intercom's Developer Hub and make a new one.
- Always pass `--json`. Each line is one JSON object: read `event`, `kind`, `counts`, `code` and `message`. The output holds ids and counts, never customer content.
- Never run `rollback` unless the person asks for it in their own words, and confirm first: it deletes everything the migration created in dimmi.
- Long runs are normal. A big workspace can take hours. Run commands in the background where you can, and report progress from the `progress` events.

## 1. Download the binary and check it

Pick the file for the OS and CPU (`uname -s`, `uname -m`; on Windows, `$env:PROCESSOR_ARCHITECTURE`):

| System | File |
| --- | --- |
| macOS, Apple silicon | `dimmi-migrate_darwin_arm64` |
| macOS, Intel | `dimmi-migrate_darwin_amd64` |
| Linux x86-64 | `dimmi-migrate_linux_amd64` |
| Linux ARM | `dimmi-migrate_linux_arm64` |
| Windows x64 | `dimmi-migrate_windows_amd64.exe` |
| Windows ARM | `dimmi-migrate_windows_arm64.exe` |

```bash
base=https://github.com/dimmihq/dimmi/releases/latest/download
file=dimmi-migrate_darwin_arm64
curl -fsSLO "$base/$file" && curl -fsSLO "$base/checksums.txt"
grep " $file\$" checksums.txt | shasum -a 256 -c -
chmod +x "$file" && mv "$file" ./dimmi-migrate
```

On Linux use `sha256sum -c -`; in PowerShell compare `(Get-FileHash $file -Algorithm SHA256).Hash` with the line in `checksums.txt`. **Stop if the checksum doesn't match** and tell the person; don't run the file.

## 2. Connect

```bash
./dimmi-migrate connect --json
```

Self-hosted dimmi: add `--dimmi-url https://support.example.com`.

Tell the person: "A page opened in your browser. Sign in to dimmi there, then follow its steps to make a read-only app in Intercom and paste its token. Tell me when the page says you're connected." The command prints a `connect_page` event with the page's address (open it for them if the browser didn't); it exits with a `connected` event once they finish. Keys go to the system keychain.

## 3. Plan, and ask what to skip

```bash
./dimmi-migrate plan --json
```

The `plan` event's `data` holds `kinds` (count per kind, with notes), `missing_permissions`, `not_imported` and `warnings`. Explain it in plain words, for example:

> Your Intercom workspace has 12 teammates, 4,310 contacts, 38,900 conversations, 210 articles in English and French, and 14 macros. Teammates arrive switched off: you choose who gets access in dimmi. Segments, outbound messages and reports stay behind.

Then ask:

1. If `missing_permissions` isn't empty: ask them to tick those in the Intercom app and run `plan` again.
2. Whether to leave anything out (often `contact_note` or old `ticket`s). Kinds: `teammate team tag attribute_definition company contact contact_note ticket_type help_center collection article snippet saved_reply schedule conversation ticket workflow`.
3. Whether to bring workflows. Intercom can't list them, so ask for the URL of each workflow they want; they arrive switched off, with the steps dimmi can't run listed.

## 4. Run

```bash
./dimmi-migrate run --json --skip contact_note --workflow https://app.intercom.com/a/apps/abc123/automation/workflows/987654
```

Leave out `--skip` and `--workflow` if they don't apply. Report each `kind_finished` event in a sentence. `record_failed` events name a record id and a reason; collect them and summarise at the end instead of reporting each.

If the run stops (network, laptop asleep, Ctrl-C), run the same command again: it resumes from its checkpoint and never duplicates.

Nothing imported sends an email or notification or starts a workflow.

## 5. Verify

```bash
./dimmi-migrate verify --json
```

The `verification` event says whether everything checked is in dimmi, kind by kind (`source` and `dimmi` counts, `missing` sampled ids). If something is missing, run `./dimmi-migrate sync --json` and verify again. Then ask the person to open dimmi and look at a few conversations, their help center and Settings → Teammates.

## 6. Keep it in step until the switch

While the team still works in Intercom, keep dimmi current:

```bash
./dimmi-migrate sync --watch 15m --json
```

It brings over what changed since the last run, every 15 minutes, until stopped. Run it in the background.

## 7. Cutover checklist

Walk the person through these, in order, and check each off with them:

1. **Teammates**: in dimmi, Settings → Teammates, enable the people who should get access. Imported teammates can't sign in until enabled.
2. **Messenger**: replace Intercom's snippet on their site and app with dimmi's (Settings → Messenger shows it). Signed-in users keep their identity: dimmi matches them by the user id they already send.
3. **Email**: point the support address's forwarding at the dimmi inbox address (Settings → Channels → Email), and add dimmi's sending domain records.
4. **Help center**: move the custom domain's DNS record to dimmi. Old article links redirect to the new ones.
5. **Last sync**: stop `sync --watch`, run `./dimmi-migrate sync --json` once more, then `verify`.
6. **Intercom**: once they're happy, delete the private app in Intercom's Developer Hub. The token stops working, and `./dimmi-migrate disconnect` forgets the saved keys.

## Undo

Only when the person asks, after confirming:

```bash
./dimmi-migrate rollback --yes --json
```

It removes everything this migration created in dimmi. Records that existed in dimmi before stay. The next `run` starts over.
