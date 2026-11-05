/*
|--------------------------------------------------------------------------
| Upload Failures
|--------------------------------------------------------------------------
|
| What a client hears when a file it uploads is refused. The codes stay
| stable because messengers and apps show a notice for each one.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
retryStorageSeconds is how long a client waits before asking again while
the file store does not answer.
*/
const retryStorageSeconds = 5

/*
Failures of an upload.
*/
var (
	ErrAttachmentNotFound = fault.New(fault.NotFound, "attachment_not_found",
		"No upload of yours has this id.")
	ErrAttachmentTooLarge = fault.New(fault.TooLarge, "attachment_too_large",
		"This file is larger than this messenger accepts.")
	ErrAttachmentTypeBlocked = fault.New(fault.Invalid, "attachment_type_blocked",
		"Files of this type cannot be sent.")
	ErrAttachmentSizeMismatch = fault.New(fault.Invalid, "attachment_size_mismatch",
		"The uploaded file is not the size that was announced; upload it again.")
	ErrAttachmentImageTooLarge = fault.New(fault.Invalid, "attachment_image_too_large",
		"This picture has too many pixels; send a smaller one.")
	ErrStorageUnavailable = fault.New(fault.Unavailable, "storage_unavailable",
		"Files cannot be stored right now; try again shortly.").With("retry_after", retryStorageSeconds)
)
