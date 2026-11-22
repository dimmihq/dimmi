package readable_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/app/support/readable"
)

/*
words is a reply with all of its white space taken out: what must
never change.
*/
func words(reply string) string {
	return strings.Join(strings.Fields(reply), "")
}

func TestFormat(t *testing.T) {
	t.Parallel()

	t.Run("breaks a paragraph that runs past three sentences into even ones", func(t *testing.T) {
		t.Parallel()
		in := "The void is pending. The refund was submitted. It returns to your balance. USPS takes 14 to 21 days."
		assert.Equal(t,
			"The void is pending. The refund was submitted.\n\nIt returns to your balance. USPS takes 14 to 21 days.",
			readable.Format(in))

		five := "One is here. Two is here. Three is here. Four is here. Five is here."
		assert.Equal(t,
			"One is here. Two is here. Three is here.\n\nFour is here. Five is here.", readable.Format(five))
	})

	t.Run("leaves a paragraph of three sentences or fewer alone", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{
			"Done.", "It is on its way. Expect it Friday.", "One is here. Two is here. Three is here.",
			"Your plan costs $49.00 a month, e.g. for 1,500 labels.",
		} {
			assert.Equal(t, in, readable.Format(in))
		}
	})

	t.Run("never adds, removes or reorders a word", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{
			"A one. B two. C three. D four. E five. F six. G seven.",
			"Press **Confirm. Then wait.** After that check the card. It is safe. Really it is. Promise.",
			"See [the guide. Part two](https://example.com/a.b) first. Then go on. Then stop. Then rest. Done.",
			"Intro here.\n\n- one. two. three. four. five.\n- six\n\nOutro one. Outro two. Outro three. Outro four.",
		} {
			out := readable.Format(in)
			assert.Equal(t, words(in), words(out), in)
			assert.Equal(t, out, readable.Format(out), "formatting again changes nothing")
		}
	})

	t.Run("does not break inside bold, code or a link", func(t *testing.T) {
		t.Parallel()
		out := readable.Format("Press **Confirm. Then wait.** After that check the card. It is safe. Really it is.")
		for paragraph := range strings.SplitSeq(out, "\n\n") {
			assert.Zero(t, strings.Count(paragraph, "**")%2, paragraph)
		}
	})

	t.Run("leaves lists, tables, headings, quotes, code and laid-out lines as written", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{
			"- One is here. Two is here. Three is here. Four is here.",
			"1. One is here. Two is here. Three is here. Four is here.",
			"## One is here. Two is here. Three is here. Four is here.",
			"> One is here. Two is here. Three is here. Four is here.",
			"| a. b. c. d. | e |",
			"One is here.\nTwo is here.\nThree is here.\nFour is here.",
			"```\n\na. b. c. d. e.\n\n```",
		} {
			assert.Equal(t, in, readable.Format(in))
		}
	})

	t.Run("finds sentences in any script", func(t *testing.T) {
		t.Parallel()
		arabic := "تم الإلغاء. تم إرسال الاسترداد. يعود المبلغ إلى رصيدك. هل الرصيد مناسب؟"
		assert.Equal(t, 2, len(strings.Split(readable.Format(arabic), "\n\n")))

		japanese := "ラベルは無効になりました。返金を申請しました。残高に戻ります。カードに戻しますか？"
		assert.Equal(t, 2, len(strings.Split(readable.Format(japanese), "\n\n")))

		russian := "Посылка в пути. Последнее сканирование было 12 ноября. " +
			"Доставка ожидалась 15 ноября. Я открыла розыск."
		assert.Equal(t, 2, len(strings.Split(readable.Format(russian), "\n\n")))
	})
}

func TestAbbreviations(t *testing.T) {
	t.Parallel()

	t.Run("never breaks after what looks like an abbreviation", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{
			"We ship with USPS. You asked about USPS vs. UPS rates today. UPS is faster. USPS is cheaper overall.",
			"Your parcel is delayed. Please contact Dr. Smith at the depot. He has the details. He replies fast.",
			"Посылка задержана. Она находится в г. Москва на складе. Доставка будет завтра. Мы сообщим отдельно.",
			"Das Paket ist unterwegs. Es kommt z. B. Montag oder Dienstag an. Wir melden uns dann. Danke sehr.",
			"El paquete salió ayer. Lo recibe el Sr. Gómez en la oficina. Llega mañana temprano. Gracias por esperar.",
			"Le colis est parti. Il est pour M. Dupont au bureau. Il arrive demain matin. Merci beaucoup.",
		} {
			out := readable.Format(in)
			assert.Equal(t, words(in), words(out), in)
			assert.Greater(t, len(strings.Split(out, "\n\n")), 1, "the paragraph is still broken up: "+in)
			for paragraph := range strings.SplitSeq(out, "\n\n") {
				for _, short := range []string{"vs.", "Dr.", "г.", "B.", "z.", "Sr.", "M."} {
					assert.False(t, strings.HasSuffix(paragraph, " "+short), "broken after "+short+": "+out)
				}
			}
		}
	})

	t.Run("breaks at the nearest safe place when the even one is not safe", func(t *testing.T) {
		t.Parallel()
		in := "One is here. Ask me. Three is here. Four is here."
		assert.Equal(t, "One is here.\n\nAsk me. Three is here.\n\nFour is here.", readable.Format(in))
	})
}
