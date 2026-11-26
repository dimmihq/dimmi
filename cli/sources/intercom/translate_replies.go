package intercom

import (
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/cli/format"
)

/*
placeholder finds an Intercom placeholder: {{ name }} or
{{ name | fallback: "x" }}.
*/
var placeholder = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*(?:\|\s*fallback\s*:\s*("[^"]*"|'[^']*'))?\s*\}\}`)

/*
liquidName maps an Intercom placeholder name, in lower case, onto
dimmi's Liquid.
*/
func liquidName(name string) (string, bool) {
	switch name {
	case "first_name", "user.first_name", "contact.first_name":
		return "contact.first_name", true
	case "name", "user.name", "full_name", "user.full_name", "contact.name":
		return "contact.name", true
	case "email", "user.email", "contact.email":
		return "contact.email", true
	case "company.name", "user.company.name", "company_name":
		return "company.name", true
	case "admin.first_name", "teammate.first_name", "current_admin.first_name",
		"conversation.admin_assignee.first_name":
		return "sender.first_name", true
	case "admin.name", "teammate.name", "current_admin.name", "conversation.admin_assignee.name":
		return "sender.name", true
	}

	return "", false
}

/*
rewritePlaceholders rewrites Intercom placeholders into dimmi's Liquid,
fallbacks becoming defaults. Names dimmi has no value for stay as they
were and are listed.
*/
func rewritePlaceholders(body string) (string, []string) {
	var unknown []string

	out := placeholder.ReplaceAllStringFunc(body, func(m string) string {
		parts := placeholder.FindStringSubmatch(m)
		name, fallback := parts[1], parts[2]

		liquid, ok := liquidName(strings.ToLower(name))
		if !ok {
			if !slices.Contains(unknown, name) {
				unknown = append(unknown, name)
			}

			return m
		}

		if fallback != "" {
			return fmt.Sprintf("{{ %s | default: %s }}", liquid, fallback)
		}

		return "{{ " + liquid + " }}"
	})

	return out, unknown
}

/*
macroBody converts a macro's HTML and rewrites its placeholders. They
are set aside while the HTML becomes Markdown, which would otherwise
escape their underscores.
*/
func macroBody(body string) (string, []string) {
	var kept []string

	held := placeholder.ReplaceAllStringFunc(body, func(m string) string {
		kept = append(kept, m)

		return fmt.Sprintf("DIMMIPLACEHOLDER%dX", len(kept)-1)
	})

	md := toMarkdown(held)

	for i, m := range kept {
		md = strings.Replace(md, fmt.Sprintf("DIMMIPLACEHOLDER%dX", i), html.UnescapeString(m), 1)
	}

	return rewritePlaceholders(md)
}

func translateMacro(m macro) format.Record {
	body, unknown := macroBody(firstOf(m.Body, m.BodyText))

	shared := m.VisibleTo != "specific_teams"
	r := &format.SavedReply{
		Base:  format.Base{SourceID: string(m.ID), SourceUpdatedAt: parseTime(m.UpdatedAt)},
		Title: text(m.Name), Body: body, Shared: &shared, Unknown: unknown,
	}

	if !shared {
		r.TeamRefs = slices.Clone(m.TeamIDs)
	}

	return format.NewRecord(format.KindSavedReply, r.SourceID, r)
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return format.At(t)
		}
	}

	return nil
}

/*
weekday names a day of the week, Monday first.
*/
func weekday(day int) string {
	return [...]string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}[day]
}

func clock(minute int) string {
	if minute >= 1440 {
		return "24:00"
	}

	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}

/*
weekly splits intervals measured from Monday 00:00 into stretches of
each day; one that runs past midnight is split at it.
*/
func weekly(intervals []interval) []format.Hours {
	var out []format.Hours

	for _, iv := range intervals {
		start, end := iv.StartMinute, iv.EndMinute
		if end == 10079 {
			end = 10080
		}

		for start < end {
			day := start / 1440
			if day > 6 {
				break
			}

			dayEnd := min(end, (day+1)*1440)
			out = append(out, format.Hours{
				Day: weekday(day), Start: clock(start - day*1440), End: clock(dayEnd - day*1440),
			})
			start = dayEnd
		}
	}

	return out
}

func translateSchedule(s schedule, exceptions []scheduleException, first bool) format.Record {
	sch := &format.Schedule{
		Base: base(s.ID, s.UpdatedAt), Name: text(s.Name), Timezone: s.TimeZone, Weekly: weekly(s.Intervals),
		Default: first,
	}

	if s.AlwaysOpen && len(sch.Weekly) == 0 {
		sch.Weekly = weekly([]interval{{StartMinute: 0, EndMinute: 10080}})
	}

	for _, e := range exceptions {
		if e.Type == "closed" {
			sch.Holidays = append(sch.Holidays, format.Holiday{
				Date: e.Date, Name: text(e.Name), Yearly: e.RecurringAnnually,
			})

			continue
		}

		for _, iv := range e.Intervals {
			sch.Exceptions = append(sch.Exceptions, format.Exception{
				Date: e.Date, Name: text(e.Name), Start: clock(iv.StartMinute), End: clock(iv.EndMinute),
				Yearly: e.RecurringAnnually,
			})
		}
	}

	return format.NewRecord(format.KindSchedule, sch.SourceID, sch)
}
