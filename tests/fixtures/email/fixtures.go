/*
|--------------------------------------------------------------------------
| Email Fixtures
|--------------------------------------------------------------------------
|
| Raw emails as real clients and mail systems write them, for the email
| channel's tests: replies from Gmail, Outlook and Apple Mail, HTML-only
| and plain text mail, automatic replies, delivery and feedback reports,
| forwards, attachments, newsletters and provider notices. Placeholders
| in double braces are filled in by each test. The DKIM key pair signs
| mail the tests need proven; it protects nothing.
|
*/

package email

import "embed"

/*
Files holds the fixtures.
*/
//go:embed *.eml dkim_private.pem dkim_public.txt
var Files embed.FS
