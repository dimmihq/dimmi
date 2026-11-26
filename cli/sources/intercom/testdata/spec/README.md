Answers shaped by Intercom's own description of its REST API 2.18,
`descriptions/2.18/api.intercom.io.yaml` in
https://github.com/intercom/Intercom-OpenAPI (MIT licence).

- `NAME.example.json`: Intercom's example answer for the endpoint.
- `NAME.schema.json`: an answer built from the endpoint's response
  schema with every property set to a value of its type.

`spec_test.go` serves them and reads every kind from them.
