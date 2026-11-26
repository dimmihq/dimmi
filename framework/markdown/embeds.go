/*
|--------------------------------------------------------------------------
| Embeds
|--------------------------------------------------------------------------
|
| Videos and interactive demos are embedded only from an allow-list of
| providers. An embed names the page a person would share, such as
| "https://youtu.be/dQw4w9WgXcQ"; the renderer reads the video or demo
| id out of it and builds the player address itself from a fixed prefix,
| so nothing from the source is ever used as a frame address as it is.
| An address from any other host, or one whose id does not have the
| provider's shape, is no embed and renders as a plain link.
|
*/

package markdown

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

/*
Embed is a resolved embed: the provider's name, its player address and
the address a person can open instead.
*/
type Embed struct {
	Provider string
	Player   string
	URL      string
}

/*
provider is one allowed embed source. Hosts are the hosts its share
addresses use, Player the fixed start of its player addresses, and id
reads the id out of a share address's path and query.
*/
type provider struct {
	name   string
	hosts  []string
	player string
	id     func(u *url.URL) (string, bool)
	suffix func(u *url.URL) string
}

var (
	youtubeID = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	digitsID  = regexp.MustCompile(`^\d{1,15}$`)
	hexID     = regexp.MustCompile(`^[a-f0-9]{32}$`)
	wistiaID  = regexp.MustCompile(`^[a-z0-9]{10}$`)
	slugID    = regexp.MustCompile(`^[A-Za-z0-9_-]{4,64}$`)
	vimeoHash = regexp.MustCompile(`^[a-f0-9]{6,20}$`)
)

/*
providers is the allow-list, in the order the documentation lists it:
video hosts first, then interactive demo tools.
*/
func providers() []provider {
	return append(videoProviders(), demoProviders()...)
}

/*
videoProviders are the allowed video hosts.
*/
func videoProviders() []provider {
	return []provider{
		{
			name:   "youtube",
			hosts:  []string{"youtube.com", "www.youtube.com", "m.youtube.com", "youtu.be", "www.youtube-nocookie.com"},
			player: "https://www.youtube-nocookie.com/embed/",
			id:     youtubeVideo,
			suffix: youtubeStart,
		},
		{
			name:   "vimeo",
			hosts:  []string{"vimeo.com", "www.vimeo.com", "player.vimeo.com"},
			player: "https://player.vimeo.com/video/",
			id:     vimeoVideo,
			suffix: vimeoPrivacy,
		},
		{
			name:   "loom",
			hosts:  []string{"loom.com", "www.loom.com"},
			player: "https://www.loom.com/embed/",
			id:     pathID(hexID, "share", "embed"),
		},
		{
			name:   "wistia",
			hosts:  []string{"fast.wistia.net", "fast.wistia.com", "*.wistia.com"},
			player: "https://fast.wistia.net/embed/iframe/",
			id:     wistiaVideo,
		},
		{
			name:   "vidyard",
			hosts:  []string{"share.vidyard.com", "play.vidyard.com", "video.vidyard.com"},
			player: "https://play.vidyard.com/",
			id:     vidyardVideo,
		},
	}
}

/*
demoProviders are the allowed interactive demo tools.
*/
func demoProviders() []provider {
	return []provider{
		{
			name:   "arcade",
			hosts:  []string{"app.arcade.software", "demo.arcade.software"},
			player: "https://demo.arcade.software/",
			id:     arcadeDemo,
			suffix: func(*url.URL) string { return "?embed" },
		},
		{
			name:   "supademo",
			hosts:  []string{"app.supademo.com"},
			player: "https://app.supademo.com/embed/",
			id:     pathID(slugID, "demo", "embed"),
		},
		{
			name:   "storylane",
			hosts:  []string{"app.storylane.io"},
			player: "https://app.storylane.io/demo/",
			id:     pathID(slugID, "share", "demo"),
		},
		{
			name:   "tella",
			hosts:  []string{"tella.tv", "www.tella.tv"},
			player: "https://www.tella.tv/video/",
			id:     pathID(slugID, "video"),
			suffix: func(*url.URL) string { return "/embed" },
		},
		{
			name:   "navattic",
			hosts:  []string{"capture.navattic.com"},
			player: "https://capture.navattic.com/",
			id:     firstSegment(slugID),
		},
	}
}

/*
EmbedProviders names every provider an embed may come from.
*/
func EmbedProviders() []string {
	var names []string
	for _, p := range providers() {
		names = append(names, p.name)
	}
	return names
}

/*
EmbedOrigins returns the origins every player address starts with, for
a content security policy's frame-src.
*/
func EmbedOrigins() []string {
	var out []string
	for _, p := range providers() {
		u, _ := url.Parse(p.player)
		origin := u.Scheme + "://" + u.Host
		if !slices.Contains(out, origin) {
			out = append(out, origin)
		}
	}
	return out
}

/*
playerPattern matches the player addresses the renderer builds, for the
sanitize policy.
*/
func playerPattern() *regexp.Regexp {
	var prefixes []string
	for _, p := range providers() {
		prefixes = append(prefixes, regexp.QuoteMeta(p.player))
	}
	suffix := `(?:/embed|\?embed|\?start=\d+|\?h=[a-f0-9]+)?`
	return regexp.MustCompile(`^(?:` + strings.Join(prefixes, "|") + `)[A-Za-z0-9_-]+` + suffix + `$`)
}

/*
ResolveEmbed reads an embed address and returns its player, or false
when the address is not from an allowed provider or has no valid id.
*/
func ResolveEmbed(raw string) (Embed, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Port() != "" {
		return Embed{}, false
	}
	host := strings.ToLower(u.Hostname())
	for _, p := range providers() {
		if !matchesHost(p.hosts, host) {
			continue
		}
		id, ok := p.id(u)
		if !ok {
			return Embed{}, false
		}
		player := p.player + id
		if p.suffix != nil {
			player += p.suffix(u)
		}
		return Embed{Provider: p.name, Player: player, URL: u.String()}, true
	}
	return Embed{}, false
}

/*
matchesHost reports whether a host is one of the hosts, where "*." at
the start of one allows any single subdomain.
*/
func matchesHost(hosts []string, host string) bool {
	for _, h := range hosts {
		if rest, ok := strings.CutPrefix(h, "*."); ok {
			sub, found := strings.CutSuffix(host, "."+rest)
			if found && sub != "" && !strings.Contains(sub, ".") {
				return true
			}
			continue
		}
		if host == h {
			return true
		}
	}
	return false
}

/*
segments splits a path into its non-empty parts.
*/
func segments(u *url.URL) []string {
	var out []string
	for s := range strings.SplitSeq(u.Path, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

/*
pathID reads an id that follows one of the prefixes, as in /share/ID.
*/
func pathID(shape *regexp.Regexp, prefixes ...string) func(*url.URL) (string, bool) {
	return func(u *url.URL) (string, bool) {
		s := segments(u)
		if len(s) < 2 || !slices.Contains(prefixes, s[0]) {
			return "", false
		}
		return s[1], shape.MatchString(s[1])
	}
}

/*
firstSegment reads an id that is the whole first part of the path.
*/
func firstSegment(shape *regexp.Regexp) func(*url.URL) (string, bool) {
	return func(u *url.URL) (string, bool) {
		s := segments(u)
		if len(s) != 1 {
			return "", false
		}
		return s[0], shape.MatchString(s[0])
	}
}

/*
youtubeVideo reads a YouTube id from watch, short, live, embed and
youtu.be addresses.
*/
func youtubeVideo(u *url.URL) (string, bool) {
	s := segments(u)
	var id string
	switch {
	case strings.EqualFold(u.Hostname(), "youtu.be") && len(s) == 1:
		id = s[0]
	case len(s) == 1 && s[0] == "watch":
		id = u.Query().Get("v")
	case len(s) == 2 && (s[0] == "embed" || s[0] == "shorts" || s[0] == "live"):
		id = s[1]
	}
	return id, youtubeID.MatchString(id)
}

/*
youtubeStart keeps a start time given in whole seconds.
*/
func youtubeStart(u *url.URL) string {
	q := u.Query()
	t := strings.TrimSuffix(q.Get("t"), "s")
	if t == "" {
		t = q.Get("start")
	}
	if n, err := strconv.Atoi(t); err == nil && n > 0 {
		return "?start=" + strconv.Itoa(n)
	}
	return ""
}

/*
vimeoVideo reads a Vimeo id from vimeo.com/ID, vimeo.com/ID/HASH and
player.vimeo.com/video/ID.
*/
func vimeoVideo(u *url.URL) (string, bool) {
	s := segments(u)
	if strings.EqualFold(u.Hostname(), "player.vimeo.com") {
		if len(s) == 2 && s[0] == "video" {
			return s[1], digitsID.MatchString(s[1])
		}
		return "", false
	}
	if len(s) == 1 || len(s) == 2 {
		return s[0], digitsID.MatchString(s[0])
	}
	return "", false
}

/*
vimeoPrivacy keeps the hash an unlisted Vimeo video needs.
*/
func vimeoPrivacy(u *url.URL) string {
	hash := u.Query().Get("h")
	if s := segments(u); hash == "" && len(s) == 2 && s[0] != "video" {
		hash = s[1]
	}
	if vimeoHash.MatchString(hash) {
		return "?h=" + hash
	}
	return ""
}

/*
wistiaVideo reads a Wistia id from /medias/ID and /embed/iframe/ID.
*/
func wistiaVideo(u *url.URL) (string, bool) {
	s := segments(u)
	switch {
	case len(s) == 2 && s[0] == "medias":
		return s[1], wistiaID.MatchString(s[1])
	case len(s) == 3 && s[0] == "embed" && (s[1] == "iframe" || s[1] == "medias"):
		return s[2], wistiaID.MatchString(s[2])
	}
	return "", false
}

/*
vidyardVideo reads a Vidyard id from /watch/ID and play.vidyard.com/ID.
*/
func vidyardVideo(u *url.URL) (string, bool) {
	s := segments(u)
	switch {
	case len(s) == 2 && s[0] == "watch":
		return s[1], slugID.MatchString(s[1])
	case len(s) == 1:
		id := strings.TrimSuffix(s[0], ".html")
		return id, slugID.MatchString(id)
	}
	return "", false
}

/*
arcadeDemo reads an Arcade id from app.arcade.software/share/ID and
demo.arcade.software/ID.
*/
func arcadeDemo(u *url.URL) (string, bool) {
	s := segments(u)
	switch {
	case len(s) == 2 && s[0] == "share":
		return s[1], slugID.MatchString(s[1])
	case len(s) == 1 && strings.EqualFold(u.Hostname(), "demo.arcade.software"):
		return s[0], slugID.MatchString(s[0])
	}
	return "", false
}
