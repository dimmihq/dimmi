# dimmi for Python

The Python backend SDK for [dimmi](https://trydimmi.com): the REST API,
messenger identity tokens and webhook verification. Python 3.10+.

```sh
pip install dimmi-sdk
```

## REST API

```python
from dimmi import Dimmi

client = Dimmi(
    token="dmi_...",            # or DIMMI_API_TOKEN / DIMMI_TOKEN
    workspace="ws_...",         # or DIMMI_WORKSPACE
    base_url="https://app.trydimmi.com",  # or DIMMI_BASE_URL; your origin when self-hosting
    max_retries=3,
    timeout=30,
)

contact = client.post("/contacts", json={"email": "ada@example.com", "name": "Ada"})["data"]
conversation = client.post(
    "/conversations",
    json={"contact": {"id": contact["id"]}, "inbox_id": "inbox_...", "author": "contact",
          "body": "Hi, I need help with billing."},
    idempotency_key="order-1234-help",
)["data"]

for item in client.paginate("/conversations", {"status": "open"}):   # cursor lists
    print(item["id"])

for article in client.pages("/some-page-numbered-list"):            # page-number lists
    ...

client.request("PATCH", f"/contacts/{contact['id']}", json={"name": "Ada L."})
```

- Every request carries `Authorization: Bearer` and `Dimmi-Workspace`.
- 429 and 503 answers are retried `max_retries` times. The wait follows
  `Retry-After`, capped at 60 seconds; otherwise exponential backoff.
- Connection failures are retried too.
- Every POST gets an `Idempotency-Key` (a uuid4 unless you pass
  `idempotency_key=`), so a retried POST never acts twice.
- Responses of 400 and above raise typed errors.

### Every operation, generated

`client.api` is a client generated from the OpenAPI spec with
[openapi-python-client](https://github.com/openapi-generators/openapi-python-client).
It shares the same httpx client, so auth, retries and errors apply.

```python
from dimmi._generated.api.default import conversations_index, contacts_store
from dimmi._generated.models import ContactBody

page = conversations_index.sync(client=client.api)
```

Operations live in `dimmi._generated.api.default`, one module per
`operationId` (`contacts.store` becomes `contacts_store`). Each has
`sync`, `sync_detailed`, `asyncio` and `asyncio_detailed`.

### Async

```python
from dimmi import AsyncDimmi

async with AsyncDimmi() as client:
    async for conversation in client.paginate("/conversations"):
        ...
```

### Errors

```python
from dimmi import DimmiError, ValidationError, RateLimitError

try:
    client.post("/contacts", json={"email": "not-an-email"})
except ValidationError as error:
    print(error.status, error.code, error.message, error.fields)  # fields: {"email": ["..."]}
except DimmiError as error:
    ...
```

| Status | Error                                            |
| ------ | ------------------------------------------------ |
| 400    | `BadRequestError`                                |
| 401    | `AuthenticationError`                            |
| 403    | `PermissionDeniedError`                          |
| 404    | `NotFoundError`                                  |
| 409    | `ConflictError`                                  |
| 422    | `ValidationError`                                |
| 429    | `RateLimitError` (after retries; `.retry_after`) |
| 5xx    | `ServerError`                                    |

All subclass `DimmiError(status, code, message, fields)`.

## Identify users to the messenger

Sign a short-lived token for the signed-in user on your server. Never
send the secret to a browser.

```python
from dimmi import sign_identity_token, messenger_snippet

token = sign_identity_token(
    {
        "id": user.id,                      # required
        "email": user.email,
        "email_verified": user.email_confirmed,
        "name": user.name,
        "signed_up_at": user.created_at,    # datetime, date, epoch or ISO string
        "attributes": {"plan": "pro"},      # snake_case keys, flat values
        "company": {"id": org.id, "name": org.name},
    },
    # secret= or DIMMI_IDENTITY_SECRET (dmi_idsec_..., HS256)
    # private_key= or DIMMI_IDENTITY_PRIVATE_KEY (PKCS #8 PEM: ES256, RS256, EdDSA)
    # kid= or DIMMI_IDENTITY_KID (required with a private key)
    expires_in=600,
)

html = messenger_snippet("wgt_...", "https://app.trydimmi.com", token=token)
```

Fields: `id`, `email`, `email_verified`, `name`, `phone`,
`phone_verified`, `avatar_url`, `locale`, `timezone`, `signed_up_at`,
`company`, `attributes`. The camelCase names of the JavaScript SDK are
accepted too. Anything else raises `IdentityError` with a stable `code`
(`user_field_unknown`, `attribute_key_invalid`, `secret_missing`, ...).

### A token endpoint for long sessions

The messenger can fetch fresh tokens itself:

```python
from dimmi import sign_identity_token, token_response, messenger_snippet

@app.get("/dimmi/token")
def dimmi_token(request):
    user = current_user(request)
    body, headers = token_response(sign_identity_token({"id": user.id, "email": user.email}) if user else None)
    return Response(body, status_code=200, headers=headers)

html = messenger_snippet("wgt_...", "https://app.trydimmi.com", identity_url="/dimmi/token")
```

## Webhooks

```python
from dimmi import Webhook, WebhookVerificationError

webhook = Webhook(os.environ["DIMMI_WEBHOOK_SECRET"])  # whsec_...

@app.post("/dimmi/webhook")
async def receive(request: Request):
    try:
        event = webhook.verify(await request.body(), request.headers)
    except WebhookVerificationError:
        return Response(status_code=400)
    if event.type == "conversation.created":
        handle(event.data["object"])
    return Response(status_code=204)
```

`WebhookEvent` has `id`, `type`, `created_at`, `workspace_id`, `data`,
`actor`, `test` and `raw`. Verify the raw body bytes, before any JSON
parsing. Signatures follow [Standard Webhooks](https://www.standardwebhooks.com).

## Regenerating after API changes

`scripts/generate.sh` is the one entry point. It takes a spec file or
URL. With no argument it reads the committed snapshot
`tests/contracts/api_v1.openapi.json` at the repo root, or fetches
`http://localhost:8080/api/v1/openapi.json` when the snapshot is absent.

```sh
cd sdk/python
scripts/generate.sh                                              # repo snapshot
scripts/generate.sh ../../tests/contracts/api_v1.openapi.json    # explicit file
scripts/generate.sh http://localhost:8080/api/v1/openapi.json    # running server

python -m venv .venv && .venv/bin/pip install -e '.[test]'
.venv/bin/pytest -q
```

`src/dimmi/_generated` is never committed (.gitignore); run `make sdk-generate` after cloning and after the API changes. The wheel and sdist still ship it (`[tool.hatch.build] artifacts`), and the release generates it before building.
The script needs `uv` or `python3`.

- The generator is pinned to openapi-python-client 0.29.0. Later
  releases emit `httpx2` and `typing.Self`, which drop httpx and 3.10.
- ruff formats the output with Python 3.10 as the target.
- dimmi serves OpenAPI 3.1, which 0.29.0 reads without changes.
- A small preprocessing step pins `servers` to `/api/v1` so generated
  paths stay relative. Add spec fixes there if the generator ever chokes.
- Never break public names. When the spec drops or renames a generated
  model, add a shim to `scripts/compat/models/`. The script copies it
  back in and exports it from `dimmi._generated.models`.

## Development

```sh
python -m venv .venv && .venv/bin/pip install -e '.[test]'
.venv/bin/pytest
```
