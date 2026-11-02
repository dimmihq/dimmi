/*
|--------------------------------------------------------------------------
| S3 Store
|--------------------------------------------------------------------------
|
| Objects are written through the SDK's transfer manager, which sends a
| small file in one request and a large one in parts. A failed upload
| never becomes an object: a single request is only sent once the whole
| body was read, and an unfinished multipart upload is aborted.
|
| Signed URLs are presigned requests. An upload link with a content type
| signs that header, so S3 itself refuses an upload of any other type.
|
*/

package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/dimmihq/dimmi/framework/filesystem"
)

/*
A store is the filesystem.Store of an s3 disk.
*/
type store struct {
	client   *s3.Client
	presign  *s3.PresignClient
	uploader *transfermanager.Client
	bucket   string
}

var _ filesystem.Store = (*store)(nil)

/*
Write uploads the object with its attributes.
*/
func (s *store) Write(ctx context.Context, path string, r io.Reader, opts filesystem.PutOptions) error {
	_, err := s.uploader.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:          &s.bucket,
		Key:             &path,
		Body:            r,
		ContentType:     optional(opts.ContentType),
		CacheControl:    optional(opts.CacheControl),
		ContentEncoding: optional(opts.ContentEncoding),
		Metadata:        opts.Metadata,
	})

	return mapError(err)
}

/*
Open starts reading the object.
*/
func (s *store) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &path})
	if err != nil {
		return nil, mapError(err)
	}

	return out.Body, nil
}

/*
Stat reads the object's headers.
*/
func (s *store) Stat(ctx context.Context, path string) (filesystem.Attributes, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &path})
	if err != nil {
		return filesystem.Attributes{}, mapError(err)
	}

	return filesystem.Attributes{
		Size:            aws.ToInt64(out.ContentLength),
		ContentType:     aws.ToString(out.ContentType),
		CacheControl:    aws.ToString(out.CacheControl),
		ContentEncoding: aws.ToString(out.ContentEncoding),
		LastModified:    aws.ToTime(out.LastModified),
		Metadata:        out.Metadata,
	}, nil
}

/*
Delete removes the object; S3 reports success for a missing one.
*/
func (s *store) Delete(ctx context.Context, path string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &path})

	return mapError(err)
}

/*
List pages through the objects under the prefix. S3 returns keys in
byte order, which is path order.
*/
func (s *store) List(ctx context.Context, prefix string) iter.Seq2[filesystem.Entry, error] {
	return func(yield func(filesystem.Entry, error) bool) {
		pages := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
			Bucket: &s.bucket,
			Prefix: optional(prefix),
		})

		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx)
			if err != nil {
				yield(filesystem.Entry{}, mapError(err))

				return
			}

			for _, object := range page.Contents {
				entry := filesystem.Entry{
					Path:         aws.ToString(object.Key),
					Size:         aws.ToInt64(object.Size),
					LastModified: aws.ToTime(object.LastModified),
				}
				if !yield(entry, nil) {
					return
				}
			}
		}
	}
}

/*
Sign presigns a GetObject or, for PUT, a PutObject request.
*/
func (s *store) Sign(ctx context.Context, path string, req filesystem.SignRequest) (string, error) {
	expires := s3.WithPresignExpires(req.TTL)

	if req.Method == http.MethodPut {
		signed, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
			Bucket:      &s.bucket,
			Key:         &path,
			ContentType: optional(req.ContentType),
		}, expires)
		if err != nil {
			return "", mapError(err)
		}

		return signed.URL, nil
	}

	signed, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &path}, expires)
	if err != nil {
		return "", mapError(err)
	}

	return signed.URL, nil
}

/*
mapError reports the SDK's missing key and missing object errors as
ErrNotFound.
*/
func mapError(err error) error {
	if err == nil {
		return nil
	}

	var noSuchKey *types.NoSuchKey
	var notFound *types.NotFound

	if errors.As(err, &noSuchKey) || errors.As(err, &notFound) {
		return fmt.Errorf("%w: %w", filesystem.ErrNotFound, err)
	}

	return err
}

func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
