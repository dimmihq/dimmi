---
title: API rate limits
url: https://help.lovelace.test/api-rate-limits
---
# API rate limits

The Lovelace REST API allows 600 requests per minute per API token.

## When you hit the limit

Requests above the limit get the HTTP status 429 Too Many Requests with a Retry-After header saying how many seconds to wait. Spread requests out or batch them to stay under the limit.

## API tokens

Create API tokens from **Settings**, then **API**. Tokens act with the permissions of the member who created them.
