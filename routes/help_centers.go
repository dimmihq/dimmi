/*
|--------------------------------------------------------------------------
| Help Center Routes
|--------------------------------------------------------------------------
|
| The API of a workspace's public docs and their changelog. Reading
| articles takes articles:read and writing them articles:write; the
| changelog takes changelog:read and changelog:write, feature requests
| feedback:read and feedback:write; changing a help
| center's address and look takes settings:write, and the actions check
| the member may manage help centers.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
imageUploadOverhead is room for the multipart framing around a picture
at the size limit, so a picture just under it is judged by its own bytes.
*/
const imageUploadOverhead = 16 << 10

/*
helpCenterRoutes declares the help center API.
*/
func helpCenterRoutes(workspace *routing.Router, a *app.App) {
	acts := a.Actions
	if acts.HelpCenters == nil {
		return
	}
	hc := &v1.HelpCenterController{
		HelpCenters: acts.HelpCenters, Collections: acts.Collections, Addresses: acts.HelpCenterAddresses, IDs: a.IDs,
	}
	art := &v1.ArticleController{Articles: acts.Articles, IDs: a.IDs}
	one, article := "/help-centers/{help_center}", "/articles/{article}"

	routing.Get(workspace, "/help-centers", "help-centers.index", hc.Index).Ability("articles:read")
	routing.Get(workspace, one, "help-centers.show", hc.Show).Ability("articles:read")
	routing.Patch(workspace, one, "help-centers.update", hc.Update).Ability("settings:write")
	routing.Get(workspace, one+"/collections", "help-centers.collections.index", hc.IndexCollections).
		Ability("articles:read")
	routing.Post(workspace, one+"/collections", "help-centers.collections.store", hc.StoreCollection).
		Ability("articles:write").Status(fhttp.StatusCreated)
	routing.Patch(workspace, "/collections/{collection}", "collections.update", hc.UpdateCollection).
		Ability("articles:write")
	routing.Delete(workspace, "/collections/{collection}", "collections.destroy", hc.DestroyCollection).
		Ability("articles:write").Status(fhttp.StatusNoContent)

	routing.Get(workspace, one+"/articles", "help-centers.articles.index", art.Index).Ability("articles:read")
	routing.Post(workspace, one+"/articles", "help-centers.articles.store", art.Store).
		Ability("articles:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, article, "articles.show", art.Show).Ability("articles:read")
	routing.Patch(workspace, article, "articles.update", art.Update).Ability("articles:write")
	routing.Delete(workspace, article, "articles.destroy", art.Destroy).
		Ability("articles:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, article+"/publications", "articles.publications.store", art.Publish).
		Ability("articles:write").Status(fhttp.StatusOK)
	routing.Get(workspace, article+"/versions", "articles.versions.index", art.Versions).Ability("articles:read")

	extras := &v1.ArticleExtrasController{Articles: acts.Articles, Images: acts.HelpImages, IDs: a.IDs}
	ratings := &v1.ArticleFeedbackController{Ratings: acts.HelpRatings, IDs: a.IDs}
	routing.Get(workspace, article+"/feedback", "articles.feedback.show", ratings.Show).Ability("articles:read")
	routing.Post(workspace, article+"/restorations", "articles.restorations.store", extras.Restore).
		Ability("articles:write").Status(fhttp.StatusOK)
	routing.Post(workspace, "/help-images", "help-images.store", extras.StoreImage).
		Ability("articles:write").Status(fhttp.StatusCreated).
		Use(middleware.BodyLimit(helpcenters.MaxImageBytes + imageUploadOverhead))

	changelogRoutes(workspace, a)
	feedbackRoutes(workspace, a)

	aud := &v1.HelpAudienceController{Articles: acts.Articles, Collections: acts.Collections, IDs: a.IDs}
	routing.Patch(workspace, article+"/audience", "articles.audience.update", aud.Article).Ability("articles:write")
	routing.Patch(workspace, "/collections/{collection}/audience", "collections.audience.update", aud.Collection).
		Ability("articles:write")
}

/*
changelogRoutes declares the changelog API: entries, labels and
subscribers of a help center's changelog.
*/
func changelogRoutes(workspace *routing.Router, a *app.App) {
	acts := a.Actions
	cl := &v1.ChangelogController{Entries: acts.ChangelogEntries, IDs: a.IDs}
	ex := &v1.ChangelogExtrasController{Entries: acts.ChangelogEntries, People: acts.ChangelogSubscribers, IDs: a.IDs}
	one, entry := "/help-centers/{help_center}", "/changelog-entries/{entry}"

	routing.Get(workspace, one+"/changelog-entries", "help-centers.changelog-entries.index", cl.Index).
		Ability("changelog:read").Query(numberedPageQuery())
	routing.Post(workspace, one+"/changelog-entries", "help-centers.changelog-entries.store", cl.Store).
		Ability("changelog:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, entry, "changelog-entries.show", cl.Show).Ability("changelog:read")
	routing.Patch(workspace, entry, "changelog-entries.update", cl.Update).Ability("changelog:write")
	routing.Delete(workspace, entry, "changelog-entries.destroy", cl.Destroy).
		Ability("changelog:write").Status(fhttp.StatusNoContent)
	routing.Post(workspace, entry+"/publications", "changelog-entries.publications.store", cl.Publish).
		Ability("changelog:write").Status(fhttp.StatusOK)
	routing.Delete(workspace, entry+"/publications", "changelog-entries.publications.destroy", cl.Unpublish).
		Ability("changelog:write").Status(fhttp.StatusOK)

	routing.Get(workspace, one+"/changelog-labels", "help-centers.changelog-labels.index", ex.Labels).
		Ability("changelog:read")
	routing.Post(workspace, one+"/changelog-labels", "help-centers.changelog-labels.store", ex.StoreLabel).
		Ability("changelog:write").Status(fhttp.StatusCreated)
	routing.Patch(workspace, "/changelog-labels/{label}", "changelog-labels.update", ex.UpdateLabel).
		Ability("changelog:write")
	routing.Delete(workspace, "/changelog-labels/{label}", "changelog-labels.destroy", ex.DestroyLabel).
		Ability("changelog:write").Status(fhttp.StatusNoContent)

	routing.Get(workspace, one+"/changelog-subscribers", "help-centers.changelog-subscribers.index", ex.Subscribers).
		Ability("changelog:read").Query(numberedPageQuery())
	routing.Delete(workspace, "/changelog-subscribers/{subscriber}", "changelog-subscribers.destroy",
		ex.DestroySubscriber).Ability("changelog:write").Status(fhttp.StatusNoContent)
}

/*
feedbackRoutes declares the feature requests API: a help center's
requests, their votes, comments and internal notes, and the order of
the statuses, which only members who change settings may save.
*/
func feedbackRoutes(workspace *routing.Router, a *app.App) {
	fb := &v1.FeedbackController{Posts: a.Actions.Feedback, IDs: a.IDs}
	ex := &v1.FeedbackExtrasController{Posts: a.Actions.Feedback, IDs: a.IDs}
	one, post := "/help-centers/{help_center}", "/feedback-posts/{post}"
	st := &v1.FeedbackStatusController{Posts: a.Actions.Feedback}

	routing.Get(workspace, "/feedback-statuses", "feedback-statuses.index", st.Index).Ability("feedback:read")
	routing.Patch(workspace, "/feedback-statuses", "feedback-statuses.update", st.Reorder).Ability("settings:write")

	routing.Get(workspace, one+"/feedback-posts", "help-centers.feedback-posts.index", fb.Index).
		Ability("feedback:read").Query(numberedPageQuery())
	routing.Post(workspace, one+"/feedback-posts", "help-centers.feedback-posts.store", fb.Store).
		Ability("feedback:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, post, "feedback-posts.show", fb.Show).Ability("feedback:read")
	routing.Patch(workspace, post, "feedback-posts.update", fb.Update).Ability("feedback:write")
	routing.Delete(workspace, post, "feedback-posts.destroy", fb.Destroy).
		Ability("feedback:write").Status(fhttp.StatusNoContent)

	routing.Get(workspace, post+"/votes", "feedback-posts.votes.index", ex.Voters).Ability("feedback:read")
	routing.Post(workspace, post+"/votes", "feedback-posts.votes.store", ex.Vote).
		Ability("feedback:write").Status(fhttp.StatusOK)
	routing.Delete(workspace, post+"/votes/{contact}", "feedback-posts.votes.destroy", ex.Unvote).
		Ability("feedback:write").Status(fhttp.StatusOK)
	routing.Get(workspace, post+"/comments", "feedback-posts.comments.index", ex.Comments).Ability("feedback:read")
	routing.Post(workspace, post+"/comments", "feedback-posts.comments.store", ex.StoreComment).
		Ability("feedback:write").Status(fhttp.StatusCreated)
	routing.Delete(workspace, "/feedback-comments/{comment}", "feedback-comments.destroy", ex.DestroyComment).
		Ability("feedback:write").Status(fhttp.StatusNoContent)
}
