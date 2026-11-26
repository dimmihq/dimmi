<!--
Who reads this: the chat model filling conversation attributes, appended to decisions/extract when the
team asked for reasons (app/ai/aidecision/extract.go). Teammates read each reason in the inbox beside the
value, to see why the AI chose it.

Output contract: `_reasons` holds one key per field, a short sentence for a value given, null for a null.
-->
Beside the values, fill `_reasons` with one key per field. For each value you give, write one short sentence a teammate can check, pointing at what the customer said, such as "The customer asks for a refund of order A-1042." For a null value, give null.
