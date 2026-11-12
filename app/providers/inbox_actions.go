/*
|--------------------------------------------------------------------------
| Inbox Actions
|--------------------------------------------------------------------------
|
| The shared inbox's actions and its channel drivers. The core ships the
| api channel; other parts of the product register their own drivers on
| the same manager. Only the testing environment may add the fake
| driver, which records what it would have sent.
|
*/

package providers

import (
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/attachments"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	conversationviews "github.com/dimmihq/dimmi/app/actions/conversation_views"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/drafts"
	"github.com/dimmihq/dimmi/app/actions/inboxes"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/channels"
	"github.com/dimmihq/dimmi/app/drivers/channel/api"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
bootInbox registers the core channel drivers and builds the inbox's
actions.
*/
func bootInbox(a *app.App) error {
	a.Channels = channels.NewManager(a.Application.Config.App.Env == foundation.EnvTesting)
	if err := a.Channels.Register(api.Driver{}); err != nil {
		return err
	}

	delivery := conversations.Delivery{Channels: a.Channels, Queue: a.Queue}
	a.Actions.Inbox = &app.InboxActions{
		ListInboxes: &inboxes.ListInboxes{DB: a.DB}, ShowInbox: &inboxes.ShowInbox{DB: a.DB},
		CreateInbox:    &inboxes.CreateInbox{DB: a.DB, Events: a.Events, Clock: a.Clock},
		UpdateInbox:    &inboxes.UpdateInbox{DB: a.DB, Events: a.Events, Clock: a.Clock},
		SetInboxAccess: &inboxes.SetInboxAccess{DB: a.DB, Events: a.Events, Clock: a.Clock},
		DeleteInbox:    &inboxes.DeleteInbox{DB: a.DB, Events: a.Events, Clock: a.Clock},
		ListContacts:   &contacts.ListContacts{DB: a.DB, Clock: a.Clock},
		ShowContact:    &contacts.ShowContact{DB: a.DB},
		CreateContact:  &contacts.CreateContact{DB: a.DB, Events: a.Events, Clock: a.Clock},
		UpdateContact:  &contacts.UpdateContact{DB: a.DB, Events: a.Events, Clock: a.Clock},

		ListConversations: &conversations.ListConversations{DB: a.DB, IDs: a.IDs, Clock: a.Clock},
		ShowConversation:  &conversations.ShowConversation{DB: a.DB},
		CreateConversation: &conversations.CreateConversation{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Delivery: delivery,
		},
		ApplyChanges:        &conversations.ApplyConversationChanges{DB: a.DB, Events: a.Events, Clock: a.Clock},
		TrashConversation:   &conversations.TrashConversation{DB: a.DB, Events: a.Events, Clock: a.Clock},
		RestoreConversation: &conversations.RestoreConversation{DB: a.DB, Events: a.Events, Clock: a.Clock},
		ShowNeighbors:       &conversations.ShowNeighbors{DB: a.DB},
		CountConversations:  &conversations.CountConversations{DB: a.DB, IDs: a.IDs, Clock: a.Clock, Cache: a.Cache},
		ShowReadState:       &conversations.ShowReadState{DB: a.DB},
		UpdateReadState:     &conversations.UpdateReadState{DB: a.DB, Events: a.Events, Clock: a.Clock},
		Search:              &conversations.Search{DB: a.DB, IDs: a.IDs},

		ListMessages:      &messages.ListMessages{DB: a.DB, Disks: a.Storage},
		AddNote:           &messages.AddNote{DB: a.DB, Events: a.Events, Clock: a.Clock, IDs: a.IDs},
		AddContactMessage: &messages.AddContactMessage{DB: a.DB, Events: a.Events, Clock: a.Clock},
		DeleteMessage:     &messages.DeleteMessage{DB: a.DB, Events: a.Events, Clock: a.Clock, Queue: a.Queue},
	}

	bootRecords(a)
	bootDelivery(a, delivery)

	return errors.Join(
		bootWorkbench(a),
		bootErasures(a),
		queue.Register(a.Queue, &jobs.SendChannelMessage{
			DB: a.DB, Clock: a.Clock, Events: a.Events, Channels: a.Channels, Disks: a.Storage, Queue: a.Queue,
			GIFs: a.Config.Inbox.GIF.GiphyAPIKey != "",
		}),
		queue.Register(a.Queue, &jobs.EraseMessageCopies{DB: a.DB, Storage: a.Storage, Erasers: a.MessageErasers}),
		queue.Register(a.Queue, &jobs.PurgeStaleDrafts{
			DB: a.DB, Clock: a.Clock, RetentionDays: a.Config.Inbox.Drafts.RetentionDays,
		}),
	)
}

/*
bootDelivery builds the actions around a message's delivery: replying,
reading a received email whole and sending a failed reply again.
*/
func bootDelivery(a *app.App, delivery conversations.Delivery) {
	acts := a.Actions.Inbox
	acts.Reply = &messages.ReplyToConversation{DB: a.DB, Events: a.Events, Clock: a.Clock, Delivery: delivery}
	acts.ShowOriginal = &messages.ShowOriginal{DB: a.DB}
	acts.DownloadOriginal = &messages.DownloadOriginal{DB: a.DB, Storage: a.Storage}
	acts.RetryDelivery = &messages.RetryDelivery{DB: a.DB, Events: a.Events, Clock: a.Clock, Delivery: delivery}
}

/*
bootWorkbench builds the actions of the inbox's workbench: preferences,
saved views, snooze lists, drafts and a new workspace's practice
conversations.
*/
func bootWorkbench(a *app.App) error {
	disk, err := a.Storage.Default()
	if err != nil {
		return fmt.Errorf("practice conversations disk: %w", err)
	}

	acts := a.Actions.Inbox
	saver := conversationviews.Saver{DB: a.DB, IDs: a.IDs, Events: a.Events, Clock: a.Clock}

	acts.ShowPreferences = &conversations.ShowPreferences{DB: a.DB, IDs: a.IDs}
	acts.UpdatePreferences = &conversations.UpdatePreferences{DB: a.DB, IDs: a.IDs, Events: a.Events, Clock: a.Clock}
	acts.ListViews = &conversationviews.ListViews{DB: a.DB, IDs: a.IDs}
	acts.ShowView = &conversationviews.ShowView{DB: a.DB, IDs: a.IDs}
	acts.CreateView = &conversationviews.CreateView{Saver: saver}
	acts.UpdateView = &conversationviews.UpdateView{Saver: saver}
	acts.DeleteView = &conversationviews.DeleteView{Saver: saver}
	acts.ReorderViews = &conversationviews.ReorderViews{Saver: saver}
	acts.ListSnoozeLists = &conversations.ListSnoozeLists{DB: a.DB}
	acts.UpdateSnoozeList = &conversations.UpdateSnoozeList{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.ShowSnoozeOptions = &conversations.ShowSnoozeOptions{DB: a.DB, Clock: a.Clock}
	acts.ListMentionables = &conversations.ListMentionables{DB: a.DB}
	acts.ReactToNote = &messages.ReactToNote{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.EditNote = &messages.EditNote{DB: a.DB, Events: a.Events, Clock: a.Clock, IDs: a.IDs}

	drafter := drafts.Saver{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.ListDrafts = &drafts.ListDrafts{DB: a.DB}
	acts.SaveDraft = &drafts.SaveDraft{Saver: drafter}
	acts.UpdateDraft = &drafts.UpdateDraft{Saver: drafter}
	acts.DeleteDraft = &drafts.DeleteDraft{Saver: drafter}
	diskName := a.Application.Config.Filesystems.Default
	acts.UploadFile = &attachments.UploadFile{
		Create:   &attachments.CreatePendingUpload{DB: a.DB, Clock: a.Clock, Disk: disk, DiskName: diskName},
		Put:      &attachments.PutUploadContent{DB: a.DB, Disk: disk},
		Complete: &attachments.CompleteUpload{DB: a.DB, Clock: a.Clock, Disk: disk, Queue: a.Queue},
	}
	acts.CreateTestConversations = &conversations.CreateTestConversations{
		DB: a.DB, Clock: a.Clock, Create: acts.CreateConversation,
		Receiver: &conversations.ReceiveInboundMessage{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Disk: disk, DiskName: diskName, Queue: a.Queue,
			ReopenWindow: a.Config.Channels.Email.Inbound.ReopenWindow,
		},
	}

	return nil
}
