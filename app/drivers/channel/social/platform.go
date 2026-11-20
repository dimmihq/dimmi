/*
|--------------------------------------------------------------------------
| Platforms
|--------------------------------------------------------------------------
|
| Messenger and Instagram speak nearly the same messaging API, so one
| driver serves both and the platform says where they differ: how long a
| text may be, which files a reply may carry, and how a file is handed to
| Meta. A Facebook Page's file goes up as bytes; Instagram only takes an
| address it can fetch.
|
*/

package social

import (
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
Platform is one of Meta's direct-message surfaces.
*/
type Platform string

/*
The platforms. Each is also the type of its channel connections.
*/
const (
	Facebook  Platform = "facebook"
	Instagram Platform = "instagram"
)

/*
Limits Meta puts on a reply.
*/
const (
	facebookTextChars  = 2000
	instagramTextBytes = 1000
	maxImageBytes      = 8 << 20
	maxFileBytes       = 25 << 20
	maxFiles           = 10
)

/*
PlatformOf reads the platform a webhook delivery's object names.
*/
func PlatformOf(object string) (Platform, bool) {
	switch object {
	case "page":
		return Facebook, true
	case "instagram":
		return Instagram, true
	default:
		return "", false
	}
}

/*
Identity is the kind of contact identity the platform's customers have:
their id as the Page or Instagram account knows them.
*/
func (p Platform) Identity() enums.IdentityType {
	if p == Instagram {
		return enums.IdentityTypeInstagramUser
	}

	return enums.IdentityTypeFacebookUser
}

/*
Name is the platform as teammates read it.
*/
func (p Platform) Name() string {
	if p == Instagram {
		return "Instagram"
	}

	return "Messenger"
}

/*
fits reports whether one message may hold the text: 2,000 characters on
Messenger, 1,000 bytes on Instagram.
*/
func (p Platform) fits(text string) bool {
	if p == Instagram {
		return len(text) <= instagramTextBytes
	}

	return utf8.RuneCountInString(text) <= facebookTextChars
}

/*
kindOf is how the platform shows a file, and whether it takes it at
all: pictures, video and audio play in the thread, and anything else is
a file to download, which on Instagram must be a PDF.
*/
func (p Platform) kindOf(contentType string, size int64) (string, bool) {
	switch {
	case imageType(contentType):
		return "image", size <= maxImageBytes
	case strings.HasPrefix(contentType, "video/"):
		return "video", size <= maxFileBytes
	case strings.HasPrefix(contentType, "audio/"):
		return "audio", size <= maxFileBytes
	default:
		return "file", size <= maxFileBytes && (p == Facebook || contentType == "application/pdf")
	}
}

func imageType(contentType string) bool {
	return contentType == "image/jpeg" || contentType == "image/png" || contentType == "image/gif"
}
