/*
|--------------------------------------------------------------------------
| Widget Home Links
|--------------------------------------------------------------------------
|
| The links a customer lists on their messenger's home screen, such as
| booking a call or reading the docs. A visitor follows one straight from
| the messenger, so each address must be one a browser opens safely: a
| secure web page or an email address, never a script. The list keeps the
| order it was given and is replaced whole on every change.
|
*/

package widget

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxLinkTitle and maxLinkURL are the longest a link's title and address may
be.
*/
const (
	maxLinkTitle = 40
	maxLinkURL   = 2048
)

/*
linkIcons are the icons a link may show.
*/
func linkIcons() []string {
	return []string{"link", "calendar", "book", "mail", "phone", "chat", "video", "help"}
}

/*
HomeLinks returns the links the widget lists on its home screen, none when
what is stored cannot be read.
*/
func HomeLinks(w models.WidgetSetting) []ConfigHomeLink {
	links := []ConfigHomeLink{}
	if len(w.HomeLinks) == 0 {
		return links
	}

	if err := json.Unmarshal(w.HomeLinks, &links); err != nil {
		return []ConfigHomeLink{}
	}

	return links
}

/*
applyLinks checks and stores the links when the change sends them.
*/
func applyLinks(p *models.SaveWidgetSettingsParams, c SettingsChange) error {
	if !c.HomeLinksSent {
		return nil
	}

	links, err := CheckHomeLinks(c.HomeLinks)
	if err != nil {
		return err
	}

	out, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("encode widget links: %w", err)
	}

	p.HomeLinks = out

	return nil
}

/*
CheckHomeLinks refuses more links than a home screen lists, a title that is
empty or too long, an address that is not a secure web page or an email
address, and an unknown icon. It returns the links cleaned, with a missing
icon as the plain link icon.
*/
func CheckHomeLinks(links []ConfigHomeLink) ([]ConfigHomeLink, error) {
	if len(links) > maxHomeLinks {
		return nil, exceptions.WidgetField("home_links", "List up to "+strconv.Itoa(maxHomeLinks)+" links.")
	}

	out := make([]ConfigHomeLink, 0, len(links))

	for i, link := range links {
		cleaned, err := checkLink(link, "home_links."+strconv.Itoa(i))
		if err != nil {
			return nil, err
		}

		out = append(out, cleaned)
	}

	return out, nil
}

/*
checkLink cleans and checks one link, naming its fields under the prefix.
*/
func checkLink(link ConfigHomeLink, prefix string) (ConfigHomeLink, error) {
	title := str.Clean(link.Title)
	if title == "" || len([]rune(title)) > maxLinkTitle {
		return link, exceptions.WidgetField(prefix+".title", "Use 1 to 40 characters.")
	}

	address := strings.TrimSpace(link.URL)
	if !safeLinkURL(address) {
		return link, exceptions.WidgetField(prefix+".url", "Use an https:// address or a mailto: address.")
	}

	icon := link.Icon
	if icon == "" {
		icon = "link"
	}

	if problem := oneOf(linkIcons()...)(icon); problem != "" {
		return link, exceptions.WidgetField(prefix+".icon", problem)
	}

	return ConfigHomeLink{Title: title, URL: address, Icon: icon}, nil
}

/*
safeLinkURL reports whether the address is a secure web page with a host or
an email address.
*/
func safeLinkURL(address string) bool {
	if address == "" || len(address) > maxLinkURL {
		return false
	}

	parsed, err := url.Parse(address)
	if err != nil {
		return false
	}

	switch parsed.Scheme {
	case "https":
		return parsed.Host != ""
	case "mailto":
		return strings.Contains(parsed.Opaque, "@")
	default:
		return false
	}
}
