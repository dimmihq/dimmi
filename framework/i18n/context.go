/*
|--------------------------------------------------------------------------
| Locale Context
|--------------------------------------------------------------------------
|
| The locale a person reads in travels with the work done for them: the
| request sets it, and a queued mail or notification restores the
| recipient's before rendering. The translator travels beside it, so
| views, mailables and error responses render text without being handed
| a translator of their own.
|
*/

package i18n

import "context"

/*
localeKey is the context key of the locale.
*/
type localeKey struct{}

/*
translatorKey is the context key of the translator.
*/
type translatorKey struct{}

/*
WithLocale returns a context whose messages are in the locale.
*/
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKey{}, locale)
}

/*
Locale returns the locale set on the context, or "" when none is.
*/
func Locale(ctx context.Context) string {
	locale, _ := ctx.Value(localeKey{}).(string)

	return locale
}

/*
WithTranslator returns a context carrying the translator.
*/
func WithTranslator(ctx context.Context, t *Translator) context.Context {
	return context.WithValue(ctx, translatorKey{}, t)
}

/*
From returns the translator the context carries, or nil when it carries
none.
*/
func From(ctx context.Context) *Translator {
	t, _ := ctx.Value(translatorKey{}).(*Translator)

	return t
}

/*
T returns the message for the key in the locale of the context, using
the translator the context carries. Without one, it returns the key,
just as a translator does for a message no locale has.
*/
func T(ctx context.Context, key string, args Args) string {
	t := From(ctx)
	if t == nil {
		return key
	}

	return t.T(ctx, key, args)
}
