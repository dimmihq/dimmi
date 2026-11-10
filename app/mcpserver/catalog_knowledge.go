/*
|--------------------------------------------------------------------------
| Tool Catalog: Knowledge
|--------------------------------------------------------------------------
|
| The entries for help centers, articles, collections, the changelog and feature requests.
|
*/

package mcpserver

import "maps"

/*
knowledgeDocs returns the entries for help centers, articles,
collections, the changelog and feature requests.
*/
func knowledgeDocs() map[string]Doc {
	out := knowledgeDocs2()
	for _, part := range []map[string]Doc{
		helpCenterDocs1(),
		helpCenterDocs2(),
		articleDocs1(),
		articleDocs2(),
		articleDocs3(),
	} {
		maps.Copy(out, part)
	}

	return out
}

/*
helpCenterDocs1 returns entries for help centers and collections, part 1.
*/
func helpCenterDocs1() map[string]Doc {
	return map[string]Doc{
		"help-centers.index": {
			Title: "List help centers",
			Use: "Lists the workspace's help centers, the public docs sites that also hold the changelog and " +
				"feature requests. Start here to find the help center id every article, collection, changelog " +
				"and feature request tool needs. The first call makes the default help center when there is none.",
			Returns: "every help center with its name, public url, status and settings, the default one marked.",
			Next: []string{
				"dimmi_help_centers_articles_list: see its articles",
				"dimmi_help_centers_collections_list: see how its articles are grouped",
			},
		},
		"help-centers.show": {
			Title:   "Get a help center",
			Use:     "Reads one help center: its address, language, status and look.",
			Returns: "the help center with its public url and settings.",
		},
		"help-centers.update": {
			Title: "Change a help center",
			Use: "Changes a help center's name, subdomain, language, status or look. Readers see every change at " +
				"once, and a new subdomain moves the public address. Only the fields sent change.",
			Returns: "the help center as it now is.",
			Params: map[string]string{
				"subdomain": "3 to 40 lowercase letters, digits and dashes, unique across the install. Changing " +
					"it moves the public address.",
				"status": "active serves the help center to readers; disabled takes it offline.",
				"settings": "Only the settings named change; an empty text clears one. feedback_board turns on the " +
					"feature request board, feedback_approval holds customers' requests until the team approves " +
					"them, primary_color is a hex color like #2563eb, appearance is auto, light or dark.",
			},
			Hint: "subdomain_taken or subdomain_reserved means that address is not free: pick another subdomain.",
		},
		"help-centers.collections.index": {
			Title:   "List collections",
			Use:     "Lists a help center's collections, the groups articles are filed under, up to three levels deep.",
			Returns: "every collection with its parent, slug, position and audience.",
			Next:    []string{"dimmi_help_centers_articles_list: list the articles of one collection"},
		},
	}
}

/*
helpCenterDocs2 returns entries for help centers and collections, part 2.
*/
func helpCenterDocs2() map[string]Doc {
	return map[string]Doc{
		"help-centers.collections.store": {
			Title: "Create a collection",
			Use: "Adds a collection to file articles under. Collections nest three levels deep at most. " +
				"The slug is made from the name when left out.",
			Returns: "the new collection.",
			Next:    []string{"dimmi_articles_update: move articles into it with collection_id"},
			Params: map[string]string{
				"parent_id": "Id of the parent collection (col_...); leave out for a top-level collection.",
				"slug":      "Lowercase letters, digits and single dashes, unique in the help center.",
				"position":  "Order among its siblings, lowest first.",
			},
		},
		"collections.update": {
			Title: "Change a collection",
			Use: "Renames, moves or reorders a collection. There is no draft: readers see the change at once. " +
				"A changed slug leaves a redirect from the old address.",
			Returns: "the collection as it now is.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"parent_id": "Id of the new parent collection (col_...); an empty text moves it to the top level.",
				"slug":      "Lowercase letters, digits and single dashes, unique in the help center.",
				"position":  "Order among its siblings, lowest first.",
			},
			Hint: "collection_too_deep means the move would nest it more than three levels: pick a higher parent.",
		},
		"collections.destroy": {
			Title: "Delete a collection",
			Use: "Moves a collection to the trash. Its articles stay, no longer filed under any collection. " +
				"A collection with sub-collections must be emptied first.",
			Hint: "collection_has_children means it still has sub-collections: move or delete them first.",
		},
		"collections.audience.update": {
			Title: "Limit a collection to an audience",
			Use: "Keeps a whole collection to readers in one saved audience, or opens it to everyone again. " +
				"Takes effect for readers at once.",
			Returns: "the collection with its audience.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"audience_id": "Id of the saved audience (audn_...) that may read it; null or empty opens it to " +
					"everyone.",
			},
			Hint: "audience_unfit means the audience uses conditions a reader's browser could claim: pick another.",
		},
	}
}

/*
articleDocs1 returns entries for help center articles, part 1.
*/
func articleDocs1() map[string]Doc {
	return map[string]Doc{
		"help-centers.articles.index": {
			Title: "List articles",
			Use: "Lists a help center's articles, up to 200, without their bodies. Use dimmi_articles_get for " +
				"an article's text.",
			Returns: "the articles with status, visibility, has_unpublished_changes and lock_version.",
			Next:    []string{"dimmi_articles_get: read one article's body"},
			Params: map[string]string{
				"status":        "draft, published or archived; leave out for all.",
				"collection_id": "Id of a collection (col_...) to list only its articles.",
				"query":         "Text to find in titles.",
			},
		},
		"help-centers.articles.store": {
			Title: "Create an article draft",
			Use: "Writes a new article as a draft. Readers see nothing until it is published with " +
				"dimmi_articles_publications_create.",
			Returns: "the draft with its id and lock_version.",
			Next:    []string{"dimmi_articles_publications_create: publish it once the person approves the text"},
			Params: map[string]string{
				"body_markdown": "The article in Markdown, at most 256 KB.",
				"collection_id": "Id of the collection (col_...) to file it under; leave out to leave it unfiled.",
				"slug":          "Lowercase letters, digits and single dashes; made from the title when left out.",
				"visibility": "public (default) can be shown to readers; agent_only is never shown publicly and " +
					"only teaches the AI agent.",
			},
		},
		"articles.show": {
			Title:   "Get an article",
			Use:     "Reads one article with the Markdown of its working copy, which may be ahead of what readers see.",
			Returns: "the article with body_markdown, status, has_unpublished_changes and lock_version.",
		},
	}
}

/*
articleDocs2 returns entries for help center articles, part 2.
*/
func articleDocs2() map[string]Doc {
	return map[string]Doc{
		"articles.update": {
			Title: "Edit an article",
			Use: "Saves the article's working copy; readers keep seeing the published version until it is " +
				"published again. Sending status instead unpublishes it to a draft or archives it, which takes it " +
				"off the help center at once and ignores the other fields.",
			Returns: "the article as it now is, with its new lock_version.",
			Next:    []string{"dimmi_articles_publications_create: make the saved changes live"},
			Params: map[string]string{
				"lock_version": "The lock_version the article had when you read it. Send it with every save.",
				"body_markdown": "The full new article in Markdown, at most 256 KB; it replaces the old text " +
					"entirely.",
				"collection_id": "Id of a collection (col_...); an empty text leaves the article unfiled.",
				"status":        "draft unpublishes a published article; archived archives it. Versions are kept.",
				"visibility":    "public or agent_only (never shown publicly, only teaches the AI agent).",
			},
			Hint: "article_changed means someone saved since you read it: read it again and reapply your change.",
		},
		"articles.destroy": {
			Title: "Delete an article",
			Use:   "Moves an article to the trash, taking it off the help center if it was published.",
		},
		"articles.publications.store": {
			Title: "Publish an article",
			Use: "Makes the article's saved working copy live on the help center and records it as a new " +
				"version. Show the person the final text first.",
			Returns: "the published article.",
			Risk:    new(RiskReach),
			Hint:    "nothing_to_publish means the published version already matches: save changes first.",
		},
		"articles.versions.index": {
			Title:   "List article versions",
			Use:     "Lists the kept versions of an article, newest first: each publish, restore and snapshot.",
			Returns: "the versions with their id, number, kind and title.",
			Next:    []string{"dimmi_articles_restorations_create: put an older version back"},
		},
		"articles.restorations.store": {
			Title: "Restore an article version",
			Use: "Puts a kept version's words, address and placing back into the working copy. Readers see " +
				"nothing until it is published, and the restore can itself be undone by restoring again.",
			Returns: "the article with the restored working copy.",
			Next:    []string{"dimmi_articles_publications_create: make the restored text live"},
			Params: map[string]string{
				"version_id": "Id of the version (artv_...) to put back. Find it with dimmi_articles_versions_list.",
			},
		},
	}
}

/*
articleDocs3 returns entries for help center articles, part 3.
*/
func articleDocs3() map[string]Doc {
	return map[string]Doc{
		"articles.feedback.show": {
			Title: "Read article ratings",
			Use: "Reads what readers said about an article: how many found it helpful or not, and the latest " +
				"notes left by those it did not help.",
			Returns: "the helpful and unhelpful counts and up to 100 recent comments, with the reader when known.",
		},
		"articles.audience.update": {
			Title: "Limit an article to an audience",
			Use: "Keeps an article to readers in one saved audience, or opens it to everyone again. Takes effect " +
				"for readers at once, without publishing.",
			Returns: "the article with its audience.",
			Risk:    new(RiskReach),
			Params: map[string]string{
				"audience_id": "Id of the saved audience (audn_...) that may read it; null or empty opens it to " +
					"everyone.",
			},
			Hint: "audience_unfit means the audience uses conditions a reader's browser could claim: pick another.",
		},
	}
}
