/*
|--------------------------------------------------------------------------
| Engagement Occurrences
|--------------------------------------------------------------------------
|
| The occurrences of what the team publishes and sends: changelog
| entries, feature requests, broadcasts and series. Each names the
| object its payload shows, the REST collection that serves it, and the
| few facts a snapshot cannot tell, such as a request's old status.
|
*/

package webhooks

import (
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Subjects of the engagement events.
*/
const (
	subjectChangelogEntry   = "changelog_entry"
	subjectFeedbackPost     = "feedback_post"
	subjectBroadcast        = "broadcast"
	subjectSeries           = "series"
	subjectSeriesEnrollment = "series_enrollment"
)

/*
engagementSubject is the public id prefix of a subject and the REST
collection that serves it, empty when none does.
*/
type engagementSubject struct {
	prefix models.Prefix
	path   string
}

/*
engagementSubjectOf returns the prefix and collection of an engagement
subject, or false for any other kind.
*/
func engagementSubjectOf(kind string) (engagementSubject, bool) {
	switch kind {
	case subjectChangelogEntry:
		return engagementSubject{prefix: models.PrefixChangelogEntry, path: "changelog-entries"}, true
	case subjectFeedbackPost:
		return engagementSubject{prefix: models.PrefixFeedbackPost, path: "feedback-posts"}, true
	case subjectBroadcast:
		return engagementSubject{prefix: models.PrefixBroadcast, path: "broadcasts"}, true
	case subjectSeries:
		return engagementSubject{prefix: models.PrefixSeries, path: "series"}, true
	case subjectSeriesEnrollment:
		return engagementSubject{prefix: models.PrefixSeriesEnrollment}, true
	}

	return engagementSubject{}, false
}

/*
engagementOccurrence names the object of an engagement event, or
reports false for any other event.
*/
func engagementOccurrence(e any, args jobs.FanOutWebhookEventArgs) (jobs.FanOutWebhookEventArgs, bool) {
	if args.ActorType == "" {
		args.ActorType = "system"
	}

	switch ev := e.(type) {
	case appevents.ChangelogEntryPublished:
		args.Subject, args.SubjectID = subjectChangelogEntry, ev.EntryID
	case appevents.ChangelogEntryUpdated:
		args.Subject, args.SubjectID = subjectChangelogEntry, ev.EntryID
	case appevents.ChangelogEntryUnpublished:
		args.Subject, args.SubjectID = subjectChangelogEntry, ev.EntryID
	default:
		return feedbackOccurrence(e, args)
	}

	return args, true
}

/*
feedbackOccurrence names the request of a feedback event, and goes on
to broadcasts and series for any other.
*/
func feedbackOccurrence(e any, args jobs.FanOutWebhookEventArgs) (jobs.FanOutWebhookEventArgs, bool) {
	args.Subject = subjectFeedbackPost

	switch ev := e.(type) {
	case appevents.FeedbackPostCreated:
		args.SubjectID = ev.PostID
	case appevents.FeedbackPostStatusChanged:
		args.SubjectID = ev.PostID
	case appevents.FeedbackPostVoted:
		args.SubjectID = ev.PostID
	case appevents.FeedbackPostMerged:
		args.SubjectID = ev.PostID
	case appevents.FeedbackPostLinked:
		args.SubjectID = ev.PostID
	default:
		return sendingOccurrence(e, args)
	}

	return args, true
}

/*
sendingOccurrence names the broadcast or series of a sending event.
*/
func sendingOccurrence(e any, args jobs.FanOutWebhookEventArgs) (jobs.FanOutWebhookEventArgs, bool) {
	switch ev := e.(type) {
	case appevents.BroadcastSent:
		args.Subject, args.SubjectID = subjectBroadcast, ev.BroadcastID
	case appevents.BroadcastPaused:
		args.Subject, args.SubjectID = subjectBroadcast, ev.BroadcastID
	case appevents.SeriesPublished:
		args.Subject, args.SubjectID = subjectSeries, ev.SeriesID
	case appevents.SeriesJourneyCompleted:
		args.Subject, args.SubjectID = subjectSeriesEnrollment, ev.EnrollmentID
	default:
		return args, false
	}

	return args, true
}

/*
engagementExtras returns the facts of an engagement event, and of a
contact merge, that its object cannot tell.
*/
func engagementExtras(e any) map[string]any {
	switch ev := e.(type) {
	case appevents.ContactMerged:
		return map[string]any{
			"merged_id": models.PublicID(models.PrefixContact, ev.SourceID),
			"moved":     map[string]any{"conversations": ev.Conversations, "identities": ev.Identities},
		}
	case appevents.FeedbackPostStatusChanged:
		return changes("status", ev.From, ev.To)
	case appevents.FeedbackPostVoted:
		return map[string]any{"vote": map[string]any{
			"contact_id": models.PublicID(models.PrefixContact, ev.ContactID),
			"member_id":  publicID(models.PrefixMember, ev.MemberID), "priority": ev.Priority,
		}}
	case appevents.FeedbackPostMerged:
		return map[string]any{"merged_id": models.PublicID(models.PrefixFeedbackPost, ev.SourceID)}
	case appevents.FeedbackPostLinked:
		return map[string]any{"conversation_id": models.PublicID(models.PrefixConversation, ev.ConversationID)}
	case appevents.BroadcastPaused:
		return reason(ev.Reason)
	case appevents.SeriesPublished:
		return map[string]any{"version_id": models.PublicID(models.PrefixSeriesVersion, ev.VersionID)}
	case appevents.SeriesJourneyCompleted:
		return map[string]any{"reason": ev.Reason, "goal_reached": ev.Goal}
	}

	return nil
}
