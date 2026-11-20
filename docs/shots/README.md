# Docs screenshots

The pictures in the docs are captured from a running dimmi, never drawn or mocked up. `capture.ts` signs in to the Lumen demo workspace, opens each screen listed in `shots.ts` and saves it at twice the pixel density in light and dark mode to `docs/public/shots/<name>-light.webp` and `-dark.webp`. A page shows one with:

```mdx
<Screenshot name="api-tokens" alt="What the picture shows, in one sentence." />
```

## Capture

1. Build the app and its assets, then a binary with the demo commands:

   ```bash
   npx vite build && pnpm widget:build && go build -tags dev -o tmp/dimmi-shots ./cmd/dimmi
   ```

2. Serve it on its own database with the `dimmi-shots` launch configuration (port 8085). Seed the demo workspace once, on that database only:

   ```bash
   ./tmp/dimmi-shots marketing:demo --owner-email maya@lumen.test
   ```

3. Create the extra data the docs' shots show (below), once.

4. Capture every shot, or the ones whose names start with a prefix:

   ```bash
   SHOTS_URL=http://localhost:8085 node docs/shots/capture.ts --only webhook
   ```

5. Look at every new picture before you use it.

## Extra data

Run with the `dimmi-shots` database settings (`DIMMI_DATABASE__URL`, `DIMMI_REDIS__URL`) in the environment:

```bash
TOKEN=$(./tmp/dimmi-shots token:create --email maya@lumen.test --workspace-slug lumen --name Zapier --abilities "*" --json -n | grep '"token"' | jq -r .token)
./tmp/dimmi-shots token:create --email maya@lumen.test --workspace-slug lumen --name "Data warehouse sync" --abilities conversations:read,contacts:read --expires 365d -n
API=http://localhost:8085/api/v1
curl -s -X POST $API/webhook-endpoints -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"url":"https://hooks.acme.example/dimmi","description":"Order system","event_types":["conversation.*","ticket.*"]}'
curl -s -X POST $API/identity-secrets -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"algorithm":"HS256","name":"Production"}'
curl -s -X POST $API/email-domains -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"domain":"lumen.example"}'
```

Then send a test delivery to the endpoint and rotate its secret (`POST $API/webhook-endpoints/<id>/tests` with `{}`, and `POST $API/webhook-endpoints/<id>/secret-rotations` with `{"overlap_seconds":86400}`).

## Rules

- Show what the reader sees in dimmi Cloud: the capture rewrites the local server's address to `https://trydimmi.com` on screen.
- Crop to what the page talks about: a settings section (`crop: { section: "Heading" }`) or a dialog (`crop: { dialog: true }`).
- Never capture a secret, a real person or a real customer. The demo data is fictional.
- A shot that types into the app clears what it typed (`cleanup`), so drafts don't leak into other captures.
- Pages mark a picture still to capture with `{/* SHOT: name | path | what to show | alt */}`.
