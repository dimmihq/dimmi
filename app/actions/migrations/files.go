/*
|--------------------------------------------------------------------------
| Migration Files
|--------------------------------------------------------------------------
|
| A client downloads each attachment and picture from the other tool and
| uploads it here first, then names it by the file id it got back:
| attachments in a conversation entry, pictures in a body as
| dimmi-file:<file id>. A file sent with its source id is kept in the
| ledger under it, so a rerun that sends it again gets the first copy
| back. A file waits, unattached, until a record uses it, and is never
| swept away as an abandoned upload.
|
*/

package migrations

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/attachments"
	"github.com/dimmihq/dimmi/app/actions/messages"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxMigrationFileBytes is the largest file a migration uploads.
*/
const MaxMigrationFileBytes = 100 << 20

/*
FileInput is one file uploaded to a migration: its source id, empty
for none, name, type, size and bytes.
*/
type FileInput struct {
	Membership  authz.Membership
	MigrationID ids.UUID
	SourceID    string
	Filename    string
	ContentType string
	SizeBytes   int64
	Body        io.Reader
}

/*
Upload keeps the file for the migration's records, and reports whether
it was uploaded before under the same source id, in which case the first
copy is returned and the bytes sent are dropped.

Rules:
  - The caller needs imports:manage.
  - The migration is not being undone.
  - The file is at most MaxMigrationFileBytes and of an allowed type.

Events: none.
*/
func (s *Migrations) Upload(ctx context.Context, in FileInput) (models.Attachment, bool, error) {
	if err := CanMigrate(in.Membership); err != nil {
		return models.Attachment{}, false, err
	}

	ws := in.Membership.Workspace.ID
	ctx = tenancy.With(ctx, ws.String())

	row, err := s.find(ctx, ws, in.MigrationID)
	if err != nil {
		return models.Attachment{}, false, err
	}

	if row.Status == "rolling_back" || row.Status == "rolled_back" {
		return models.Attachment{}, false, ErrClosed
	}

	sourceID := cut(strings.TrimSpace(in.SourceID), 255)
	if sourceID != "" {
		if found, ok, err := s.uploaded(ctx, row, sourceID); err != nil || ok {
			return found, ok, err
		}
	}

	file, err := s.Kit.Upload.Handle(ctx, attachments.FileUpload{
		WorkspaceID: ws, Uploader: attachments.Uploader{Type: messages.UploaderMember, ID: in.Membership.Member.ID},
		Filename: in.Filename, ContentType: in.ContentType, SizeBytes: in.SizeBytes, MaxBytes: MaxMigrationFileBytes,
		Body: in.Body,
	})
	if err != nil {
		return file, false, err
	}

	if sourceID == "" {
		sourceID = "upload:" + file.ID.String()
	}

	r := &runner{svc: s, row: row, m: in.Membership}

	err = s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		st := &step{runner: r, now: s.Clock.Now()}

		return st.keepRow(ctx, models.New(tx), kept{kind: kindFile, sourceID: sourceID, id: file.ID, created: true})
	})

	return file, false, err
}

/*
uploaded returns the file uploaded before under the source id, and
reports whether there is one.
*/
func (s *Migrations) uploaded(ctx context.Context, row models.Migration, sourceID string,
) (models.Attachment, bool, error) {
	var (
		out   models.Attachment
		found bool
	)

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rec, err := q.FindMigrationRecord(ctx, models.FindMigrationRecordParams{
			MigrationID: row.ID, Kind: kindFile, SourceID: sourceID,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && rec.RecordID == nil) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find migration file: %w", err)
		}

		out, err = q.FindAttachment(ctx, models.FindAttachmentParams{WorkspaceID: row.WorkspaceID, ID: *rec.RecordID})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		found = err == nil

		return wrap("find migration file", err)
	})

	return out, found, err
}

/*
file returns the uploaded file the public file id names, refusing one
this migration did not upload.
*/
func (s *step) file(ctx context.Context, public string) (models.Attachment, error) {
	missing := invalid("attachments", "This file was not uploaded to this migration: "+public)

	id, err := s.runner.svc.IDs.Decode(string(models.PrefixAttachment), public)
	if err != nil {
		return models.Attachment{}, missing
	}

	rec, err := s.q.MigrationFileByAttachment(ctx, models.MigrationFileByAttachmentParams{
		WorkspaceID: s.ws(), RecordID: &id,
	})
	if err != nil || rec.MigrationID != s.runner.row.ID {
		return models.Attachment{}, missing
	}

	row, err := s.q.FindAttachment(ctx, models.FindAttachmentParams{WorkspaceID: s.ws(), ID: id})
	if err != nil || row.UploadedAt == nil {
		return models.Attachment{}, missing
	}

	return row, nil
}
