/*
|--------------------------------------------------------------------------
| Checking A Written Template
|--------------------------------------------------------------------------
|
| Meta reviews a template for hours; what it would refuse at once is
| caught here first, on the field to change: a name of lower-case letters,
| digits and underscores, a category dimmi sends, Meta's lengths, variables
| numbered from {{1}} in order with a sample for each, a body that neither
| starts nor ends with one, a footer without any, and buttons the inbox can
| send: quick replies, at most two fixed links and one phone number.
|
*/

package channels

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
Meta's limits on a template.
*/
const (
	templateNameMax   = 512
	templateHeaderMax = 60
	templateBodyMax   = 1024
	templateFooterMax = 60
	templateButtonMax = 25
	templateButtons   = 10
	templateLinks     = 2
	templatePhones    = 1
	templateURLMax    = 2000
	templatePhoneMax  = 20
	templateSampleMax = 200
	bindingKeyMax     = 64
)

var (
	templateName     = regexp.MustCompile(`^[a-z0-9_]+$`)
	templateLanguage = regexp.MustCompile(`^[a-z]{2,3}(_[A-Z]{2})?$`)
	templatePhone    = regexp.MustCompile(`^\+[1-9]\d{6,18}$`)
	anyPlaceholder   = regexp.MustCompile(`\{\{\s*([^}]*?)\s*\}\}`)
)

/*
checkNewDraft returns the first problem with a new template, its name
and language included, on its field, or nil.
*/
func checkNewDraft(d wadriver.Draft) error {
	if err := checkName(d); err != nil {
		return err
	}

	return checkDraft(d)
}

/*
checkDraft returns the first problem with the template's content, on
its field, or nil.
*/
func checkDraft(d wadriver.Draft) error {
	checks := []func(wadriver.Draft) error{checkCategory, checkHeader, checkBody, checkFooter, checkButtons}
	for _, check := range checks {
		if err := check(d); err != nil {
			return err
		}
	}

	return nil
}

func checkName(d wadriver.Draft) error {
	switch {
	case d.Name == "":
		return exceptions.WhatsAppTemplateInvalid("name", "Name the template.")
	case len(d.Name) > templateNameMax || !templateName.MatchString(d.Name):
		return exceptions.WhatsAppTemplateInvalid("name",
			"Use lower-case letters, digits and underscores only, such as order_shipped.")
	case !templateLanguage.MatchString(d.Language):
		return exceptions.WhatsAppTemplateInvalid("language", "Pick the template's language.")
	default:
		return nil
	}
}

func checkCategory(d wadriver.Draft) error {
	if d.Category != "utility" && d.Category != "marketing" {
		return exceptions.WhatsAppTemplateInvalid("category", "Pick utility or marketing.")
	}

	return nil
}

func checkHeader(d wadriver.Draft) error {
	if d.Header == "" {
		return nil
	}

	if utf8.RuneCountInString(d.Header) > templateHeaderMax {
		return exceptions.WhatsAppTemplateInvalid("header",
			fmt.Sprintf("Keep the header to %d characters.", templateHeaderMax))
	}

	if len(wadriver.Placeholders(d.Header)) > 1 {
		return exceptions.WhatsAppTemplateInvalid("header", "A header takes one variable at most.")
	}

	return checkVariables(d, "header", d.Header)
}

func checkBody(d wadriver.Draft) error {
	body := strings.TrimSpace(d.Body)

	switch {
	case body == "":
		return exceptions.WhatsAppTemplateInvalid("body", "Write the message.")
	case utf8.RuneCountInString(d.Body) > templateBodyMax:
		return exceptions.WhatsAppTemplateInvalid("body",
			fmt.Sprintf("Keep the message to %d characters.", templateBodyMax))
	case strings.HasPrefix(body, "{{") || strings.HasSuffix(body, "}}"):
		return exceptions.WhatsAppTemplateInvalid("body",
			"Meta refuses a message that starts or ends with a variable. Add words around it.")
	default:
		return checkVariables(d, "body", d.Body)
	}
}

func checkFooter(d wadriver.Draft) error {
	switch {
	case utf8.RuneCountInString(d.Footer) > templateFooterMax:
		return exceptions.WhatsAppTemplateInvalid("footer",
			fmt.Sprintf("Keep the footer to %d characters.", templateFooterMax))
	case anyPlaceholder.MatchString(d.Footer):
		return exceptions.WhatsAppTemplateInvalid("footer", "A footer cannot have variables.")
	default:
		return nil
	}
}

/*
checkVariables wants the part's variables numbered {{1}}, {{2}} and on in
the order they appear, each with a sample.
*/
func checkVariables(d wadriver.Draft, part, text string) error {
	for _, m := range anyPlaceholder.FindAllStringSubmatch(text, -1) {
		if _, err := strconv.Atoi(m[1]); err != nil {
			return exceptions.WhatsAppTemplateInvalid(part,
				fmt.Sprintf("Number variables {{1}}, {{2}} and on; {{%s}} is not a number.", m[1]))
		}
	}

	for i, key := range wadriver.Placeholders(text) {
		if key != strconv.Itoa(i+1) {
			return exceptions.WhatsAppTemplateInvalid(part,
				fmt.Sprintf("Number variables in order: {{%d}} comes before {{%s}}.", i+1, key))
		}

		sample := strings.TrimSpace(d.Samples[part+"."+key])
		if sample == "" || utf8.RuneCountInString(sample) > templateSampleMax {
			return exceptions.WhatsAppTemplateInvalid("samples."+part+"."+key,
				fmt.Sprintf("Give {{%s}} a sample value; Meta reviews the template with it.", key))
		}
	}

	return nil
}

func checkButtons(d wadriver.Draft) error {
	if len(d.Buttons) > templateButtons {
		return exceptions.WhatsAppTemplateInvalid("buttons",
			fmt.Sprintf("A template takes %d buttons at most.", templateButtons))
	}

	count := map[string]int{}

	for i, b := range d.Buttons {
		field := "buttons." + strconv.Itoa(i)
		count[b.Type]++

		if err := checkButton(b, field, count); err != nil {
			return err
		}
	}

	return nil
}

func checkButton(b wadriver.Action, field string, count map[string]int) error {
	text := strings.TrimSpace(b.Text)
	if text == "" || utf8.RuneCountInString(text) > templateButtonMax {
		return exceptions.WhatsAppTemplateInvalid(field+".text",
			fmt.Sprintf("Label the button in %d characters or fewer.", templateButtonMax))
	}

	switch b.Type {
	case wadriver.ButtonQuickReply:
		return nil
	case wadriver.ButtonURL:
		if count[b.Type] > templateLinks {
			return exceptions.WhatsAppTemplateInvalid(field, "A template takes two link buttons at most.")
		}

		u, err := url.Parse(b.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" || len(b.URL) > templateURLMax ||
			anyPlaceholder.MatchString(b.URL) {
			return exceptions.WhatsAppTemplateInvalid(field+".url", "Give a full https address without variables.")
		}

		return nil
	case wadriver.ButtonPhone:
		if count[b.Type] > templatePhones {
			return exceptions.WhatsAppTemplateInvalid(field, "A template takes one phone button at most.")
		}

		if len(b.PhoneNumber) > templatePhoneMax || !templatePhone.MatchString(b.PhoneNumber) {
			return exceptions.WhatsAppTemplateInvalid(field+".phone_number",
				"Give the number with its country code, such as +2348012345678.")
		}

		return nil
	default:
		return exceptions.WhatsAppTemplateInvalid(field+".type", "Pick a quick reply, link or phone button.")
	}
}
