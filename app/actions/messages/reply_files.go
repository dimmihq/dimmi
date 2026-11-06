/*
|--------------------------------------------------------------------------
| Reply Files
|--------------------------------------------------------------------------
|
| A teammate's reply may carry files they uploaded first. Only their own
| finished uploads that no message took yet can go, only on a channel
| that carries files, and only as many as it takes; anything else
| refuses the whole reply, so nothing is sent without the files the
| teammate meant to send.
|
*/

package messages

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UploaderMember marks a file a teammate uploaded.
*/
const UploaderMember = "member"

/*
checkFiles refuses files the reply's channel cannot carry.
*/
func checkFiles(route conversations.Route, files []ids.UUID) error {
	if len(files) == 0 {
		return nil
	}

	caps := route.Capabilities
	if !caps.Attachments {
		return exceptions.ErrAttachmentsDisabled
	}

	if caps.MaxAttachments > 0 && len(files) > caps.MaxAttachments {
		return exceptions.ErrAttachmentTooLarge
	}

	return nil
}

/*
attachFiles links the teammate's finished uploads to the reply.
*/
func attachFiles(ctx context.Context, s *session, files []ids.UUID, now time.Time) error {
	if len(files) == 0 {
		return nil
	}

	member := s.m.Member.ID

	uploads, err := s.w.Q.ListFinishedUploads(ctx, models.ListFinishedUploadsParams{
		WorkspaceID: s.msg.WorkspaceID, Ids: files, UploadedByType: UploaderMember, UploadedByID: &member,
	})
	if err != nil {
		return fmt.Errorf("list reply uploads: %w", err)
	}

	if len(uploads) != len(files) {
		return exceptions.ErrAttachmentNotUploaded
	}

	linked, err := s.w.Q.LinkAttachments(ctx, models.LinkAttachmentsParams{
		MessageID: &s.msg.ID, Now: now, WorkspaceID: s.msg.WorkspaceID, Ids: files,
	})
	if err != nil {
		return fmt.Errorf("link reply uploads: %w", err)
	}

	if int(linked) != len(files) {
		return exceptions.ErrAttachmentNotUploaded
	}

	return nil
}
