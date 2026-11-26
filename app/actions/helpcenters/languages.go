/*
|--------------------------------------------------------------------------
| Help Center Languages
|--------------------------------------------------------------------------
|
| A help center has one default language, the one its articles and
| collections are written in, and any number of other languages readers
| may pick. The default's addresses carry no prefix, so every address a
| help center ever had keeps working; another language's pages live
| under /<language>/. Switching a language off hides its translations
| but keeps them. The default may move to another language only while
| nothing is translated into it, since the source would then say the
| same thing twice.
|
*/

package helpcenters

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxLocales is how many languages one help center may offer.
*/
const MaxLocales = 30

/*
Failures of a help center's languages.
*/
var (
	ErrLocaleNotEnabled = fault.New(fault.Invalid, "locale_not_enabled",
		"Turn this language on for the help center first.").OnField("locale")
	ErrLocaleIsDefault = fault.New(fault.Invalid, "locale_is_default",
		"This is the help center's default language; edit the article itself.").OnField("locale")
	ErrTooManyLocales = fault.New(fault.Invalid, "too_many_locales",
		"A help center offers at most 30 languages.").OnField("locales")
	ErrDefaultLocaleTranslated = fault.New(fault.Conflict, "default_locale_translated",
		"Articles are already translated into this language. Delete those translations before making it the default.").
		OnField("locale")
)

/*
Language is a language a help center may offer: its tag, its name in
English and its name in itself.
*/
type Language struct {
	Code   string
	Name   string
	Native string
}

/*
languages are the languages the editor offers, in the order it lists
them. Any well-formed tag is accepted; these are the ones with names.
*/
var languages = []Language{
	{"ar", "Arabic", "العربية"}, {"bg", "Bulgarian", "Български"}, {"bn", "Bengali", "বাংলা"},
	{"ca", "Catalan", "Català"}, {"cs", "Czech", "Čeština"}, {"da", "Danish", "Dansk"},
	{"de", "German", "Deutsch"}, {"el", "Greek", "Ελληνικά"}, {"en", "English", "English"},
	{"en-GB", "English (UK)", "English (UK)"}, {"es", "Spanish", "Español"},
	{"es-MX", "Spanish (Mexico)", "Español (México)"}, {"et", "Estonian", "Eesti"},
	{"fa", "Persian", "فارسی"}, {"fi", "Finnish", "Suomi"}, {"fr", "French", "Français"},
	{"fr-CA", "French (Canada)", "Français (Canada)"}, {"he", "Hebrew", "עברית"}, {"hi", "Hindi", "हिन्दी"},
	{"hr", "Croatian", "Hrvatski"}, {"hu", "Hungarian", "Magyar"}, {"id", "Indonesian", "Bahasa Indonesia"},
	{"it", "Italian", "Italiano"}, {"ja", "Japanese", "日本語"}, {"ko", "Korean", "한국어"},
	{"lt", "Lithuanian", "Lietuvių"}, {"lv", "Latvian", "Latviešu"}, {"ms", "Malay", "Bahasa Melayu"},
	{"nb", "Norwegian", "Norsk bokmål"}, {"nl", "Dutch", "Nederlands"}, {"pl", "Polish", "Polski"},
	{"pt", "Portuguese", "Português"}, {"pt-BR", "Portuguese (Brazil)", "Português (Brasil)"},
	{"ro", "Romanian", "Română"}, {"ru", "Russian", "Русский"}, {"sk", "Slovak", "Slovenčina"},
	{"sl", "Slovenian", "Slovenščina"}, {"sr", "Serbian", "Српски"}, {"sv", "Swedish", "Svenska"},
	{"sw", "Swahili", "Kiswahili"}, {"th", "Thai", "ไทย"}, {"tr", "Turkish", "Türkçe"},
	{"uk", "Ukrainian", "Українська"}, {"vi", "Vietnamese", "Tiếng Việt"},
	{"zh-CN", "Chinese (Simplified)", "简体中文"}, {"zh-TW", "Chinese (Traditional)", "繁體中文"},
}

/*
Languages returns the languages the editor offers.
*/
func Languages() []Language { return slices.Clone(languages) }

/*
LanguageOf names a language tag, falling back to the tag itself.
*/
func LanguageOf(code string) Language {
	for _, l := range languages {
		if strings.EqualFold(l.Code, code) {
			return l
		}
	}
	return Language{Code: code, Name: code, Native: code}
}

/*
Listed reports whether the editor offers the language. Only these may
be added beside a default, since each becomes the first part of an
address and must never be mistaken for a page.
*/
func Listed(code string) bool {
	return slices.ContainsFunc(languages, func(l Language) bool { return l.Code == code })
}

/*
LocaleOfSegment reads the first part of a help center address as a
language the editor offers, written as it is stored.
*/
func LocaleOfSegment(seg string) (string, bool) {
	for _, l := range languages {
		if strings.EqualFold(l.Code, seg) {
			return l.Code, true
		}
	}
	return "", false
}

/*
NormalizeLocale writes a language tag the one way it is stored: the
language lowercase and a region uppercase, en, pt-BR, zh-TW. It
returns false for anything that is not a tag.
*/
func NormalizeLocale(raw string) (string, bool) {
	tag := strings.ReplaceAll(strings.TrimSpace(raw), "_", "-")
	lang, region, hasRegion := strings.Cut(tag, "-")
	lang = strings.ToLower(lang)
	if hasRegion {
		if len(region) == 2 {
			region = strings.ToUpper(region)
		} else if len(region) == 4 {
			region = strings.ToUpper(region[:1]) + strings.ToLower(region[1:])
		}
		tag = lang + "-" + region
	} else {
		tag = lang
	}
	if !localePattern.MatchString(tag) {
		return "", false
	}
	return tag, true
}

/*
LocalesOf returns every language a help center offers, its default
first. A help center saved before it had languages offers its default
alone.
*/
func LocalesOf(h models.HelpCenter) []string {
	out := []string{h.Locale}
	for _, l := range h.Locales {
		if !slices.Contains(out, l) {
			out = append(out, l)
		}
	}
	return out
}

/*
OffersLocale reports whether readers of the help center may pick the
language, the default among them.
*/
func OffersLocale(h models.HelpCenter, locale string) bool {
	return slices.Contains(LocalesOf(h), locale)
}

/*
checkTranslationLocale refuses a language the help center does not
offer, and its default, which the source itself is written in.
*/
func checkTranslationLocale(h models.HelpCenter, raw string) (string, error) {
	locale, ok := NormalizeLocale(raw)
	if !ok {
		return "", ErrLocaleInvalid
	}
	if locale == h.Locale {
		return "", ErrLocaleIsDefault
	}
	if !OffersLocale(h, locale) {
		return "", ErrLocaleNotEnabled
	}
	return locale, nil
}

/*
LanguagesChange is a change to a help center's languages; nil leaves a
part as it is.
*/
type LanguagesChange struct {
	Membership authz.Membership
	ID         ids.UUID
	Default    *string
	Locales    *[]string
}

/*
SetLanguages changes a help center's default language and the languages
it offers.
*/
func (h *HelpCenters) SetLanguages(ctx context.Context, in LanguagesChange) (models.HelpCenter, error) {
	if !in.Membership.Can("help_centers:manage") {
		return models.HelpCenter{}, exceptions.ErrForbidden
	}
	var out models.HelpCenter
	err := h.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		current, err := findHelpCenter(ctx, q, in.Membership.Workspace.ID, in.ID)
		if err != nil {
			return err
		}
		out, err = setLanguages(ctx, q, current, in, h.Clock.Now)
		return err
	})
	return out, err
}

/*
setLanguages checks and writes a change to the help center's languages.
A new default given alone takes the old default's place, as a help
center with one language always did.
*/
func setLanguages(ctx context.Context, q *models.Queries, c models.HelpCenter, in LanguagesChange, now nowFunc,
) (models.HelpCenter, error) {
	def := c.Locale
	if in.Default != nil {
		var ok bool
		if def, ok = NormalizeLocale(*in.Default); !ok {
			return c, ErrLocaleInvalid
		}
	}
	wanted := slices.DeleteFunc(LocalesOf(c), func(l string) bool { return l == c.Locale })
	if in.Locales != nil {
		wanted = nil
		for _, raw := range *in.Locales {
			l, ok := NormalizeLocale(raw)
			if !ok || (l != def && !Listed(l)) {
				return c, fault.New(fault.Invalid, "locale_invalid",
					"Pick languages from the list, such as fr or pt-BR.").OnField("locales")
			}
			wanted = append(wanted, l)
		}
	}
	locales := []string{def}
	for _, l := range wanted {
		if !slices.Contains(locales, l) {
			locales = append(locales, l)
		}
	}
	if len(locales) > MaxLocales {
		return c, ErrTooManyLocales
	}
	if def != c.Locale {
		n, err := q.CountTranslationsInLocale(ctx, models.CountTranslationsInLocaleParams{
			WorkspaceID: c.WorkspaceID, HelpCenterID: c.ID, Locale: def,
		})
		if err != nil {
			return c, fmt.Errorf("count translations: %w", err)
		}
		if n > 0 {
			return c, ErrDefaultLocaleTranslated
		}
	}
	if def == c.Locale && slices.Equal(locales, LocalesOf(c)) && len(c.Locales) > 0 {
		return c, nil
	}
	out, err := q.SetHelpCenterLocales(ctx, models.SetHelpCenterLocalesParams{
		Locale: def, Locales: locales, Now: now(), WorkspaceID: c.WorkspaceID, ID: c.ID,
	})
	if err != nil {
		return c, fmt.Errorf("set help center languages: %w", err)
	}
	return out, nil
}

/*
PickLocale picks from the offered languages the first one an
Accept-Language header asks for, best weight first, matching a region
to its language and a language to its first region. The first offered
language, the default, answers when nothing matches.
*/
func PickLocale(header string, offered []string) string {
	type ask struct {
		tag    string
		weight float64
	}
	var asks []ask
	for i, part := range strings.Split(header, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if tag == "" || tag == "*" {
			continue
		}
		weight := 1.0 - float64(i)*0.0001
		if q, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			weight = parseWeight(q) - float64(i)*0.0001
		}
		asks = append(asks, ask{tag: strings.ToLower(tag), weight: weight})
	}
	best, bestWeight := offered[0], 0.0
	for _, a := range asks {
		if a.weight <= bestWeight {
			continue
		}
		if match := matchLocale(a.tag, offered); match != "" {
			best, bestWeight = match, a.weight
		}
	}
	return best
}

/*
matchLocale finds the offered language a tag asks for: the same tag, or
the same language.
*/
func matchLocale(tag string, offered []string) string {
	for _, o := range offered {
		if strings.EqualFold(o, tag) {
			return o
		}
	}
	lang, _, _ := strings.Cut(tag, "-")
	for _, o := range offered {
		if l, _, _ := strings.Cut(strings.ToLower(o), "-"); l == lang {
			return o
		}
	}
	return ""
}

func parseWeight(s string) float64 {
	var whole, frac, scale float64 = 0, 0, 1
	seenDot := false
	for _, r := range s {
		switch {
		case r == '.':
			seenDot = true
		case r >= '0' && r <= '9' && !seenDot:
			whole = whole*10 + float64(r-'0')
		case r >= '0' && r <= '9':
			scale /= 10
			frac += float64(r-'0') * scale
		default:
			return 0
		}
	}
	return min(whole+frac, 1)
}
