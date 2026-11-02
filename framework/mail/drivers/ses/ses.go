/*
|--------------------------------------------------------------------------
| SES Mailer
|--------------------------------------------------------------------------
|
| Messages go to Amazon SES as whole MIME documents. A configuration set
| routes a stream's delivery events and keeps its reputation apart, and a
| tenant does the same for one customer of the application inside it;
| tags come back on every event so it can be matched to the message.
|
| SES replaces the Message-ID header with its own, built from the id it
| returns, so the result reports the header actually sent: replies will
| quote that one. The SDK does not retry: the caller's job queue does,
| with its own backoff, after reading the error kind.
|
*/

package ses

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/aws/smithy-go"

	"github.com/dimmihq/dimmi/framework/aws"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/config"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mail/internal/provider"
	"github.com/dimmihq/dimmi/framework/mail/mime"
	"github.com/dimmihq/dimmi/framework/manager"
)

/*
Options configure an ses mailer beyond the AWS account: the
configuration set used when a message names none.
*/
type Options struct {
	ConfigurationSet string `koanf:"configuration_set"`
}

/*
Mailer sends through Amazon SES.
*/
type Mailer struct {
	client  *sesv2.Client
	builder *mime.Builder
	region  string
	opts    Options
}

/*
Factory builds ses mailers for the mail manager. Each mailer's section
holds the AWS account options beside its own. The context bounds
loading credentials; pass the application's root context.
*/
func Factory(ctx context.Context, client *httpclient.Client, clk clock.Clock) manager.Factory[mail.Mailer] {
	return func(_ string, section config.Section) (mail.Mailer, error) {
		cfg, err := aws.Config(ctx, section, client)
		if err != nil {
			return nil, err
		}

		var opts Options
		if err := section.Decode(&opts); err != nil {
			return nil, err
		}

		return New(cfg, clk, opts), nil
	}
}

/*
New returns a mailer on the AWS configuration, dating messages by the
clock.
*/
func New(cfg awssdk.Config, clk clock.Clock, opts Options) *Mailer {
	client := sesv2.NewFromConfig(cfg, func(o *sesv2.Options) {
		o.Retryer = awssdk.NopRetryer{}
	})

	return &Mailer{client: client, builder: mime.NewBuilder(clk), region: cfg.Region, opts: opts}
}

/*
Send sends the message as a raw document.
*/
func (m *Mailer) Send(ctx context.Context, msg mail.Message) (mail.SendResult, error) {
	raw, err := m.builder.Build(msg)
	if err != nil {
		return mail.SendResult{}, err
	}

	input := &sesv2.SendEmailInput{
		Content:     &types.EmailContent{Raw: &types.RawMessage{Data: raw}},
		Destination: destination(msg),
		EmailTags:   tags(msg.Tags),
	}

	if set := cmp.Or(msg.ConfigurationSet, m.opts.ConfigurationSet); set != "" {
		input.ConfigurationSetName = awssdk.String(set)
	}

	if msg.TenantName != "" {
		input.TenantName = awssdk.String(msg.TenantName)
	}

	out, err := m.client.SendEmail(ctx, input)
	if err != nil {
		return mail.SendResult{}, classify(err)
	}

	id := awssdk.ToString(out.MessageId)

	return mail.SendResult{ProviderMessageID: id, MessageID: id + "@" + m.messageIDDomain()}, nil
}

/*
messageIDDomain is the domain of the Message-ID SES writes: its oldest
region keeps the original name.
*/
func (m *Mailer) messageIDDomain() string {
	if m.region == "us-east-1" {
		return "email.amazonses.com"
	}

	return m.region + ".amazonses.com"
}

/*
destination lists the envelope recipients, needed for Bcc recipients
the document does not name. A raw message without recipient fields
leaves SES to read its headers.
*/
func destination(msg mail.Message) *types.Destination {
	if len(msg.To)+len(msg.Cc)+len(msg.Bcc) == 0 {
		return nil
	}

	return &types.Destination{
		ToAddresses:  provider.Emails(msg.To),
		CcAddresses:  provider.Emails(msg.Cc),
		BccAddresses: provider.Emails(msg.Bcc),
	}
}

func tags(in map[string]string) []types.MessageTag {
	out := make([]types.MessageTag, 0, len(in))
	for _, name := range slices.Sorted(maps.Keys(in)) {
		out = append(out, types.MessageTag{Name: awssdk.String(name), Value: awssdk.String(in[name])})
	}

	return out
}

/*
classify maps SES error codes to mail error kinds. Anything SES did not
name, such as a lost connection, may pass on a later try.
*/
func classify(err error) error {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return fmt.Errorf("ses send: %w: %w", mail.ErrTemporary, err)
	}

	switch apiErr.ErrorCode() {
	case "TooManyRequestsException", "LimitExceededException", "Throttling", "ThrottlingException":
		return fmt.Errorf("ses send: %w: %w", &mail.RateLimitedError{}, err)
	case "MessageRejected", "MailFromDomainNotVerifiedException", "AccountSuspendedException",
		"SendingPausedException", "BadRequestException", "NotFoundException":
		return fmt.Errorf("ses send: %w: %w", mail.ErrPermanent, err)
	default:
		return fmt.Errorf("ses send: %w: %w", mail.ErrTemporary, err)
	}
}
