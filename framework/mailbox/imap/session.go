/*
|--------------------------------------------------------------------------
| IMAP Sessions
|--------------------------------------------------------------------------
|
| A session reads Sent before the folder of received mail, so a reply to
| something the mailbox just sent finds what it answers. Folders are
| examined, never selected for writing, and bodies are fetched with PEEK:
| a message the owner has not read stays unread.
|
*/

package imap

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
sentNames are the folder names taken for Sent when no folder carries the
special-use flag, in order of preference.
*/
func sentNames() []string {
	return []string{"Sent", "Sent Items", "Sent Messages", "Sent Mail"}
}

/*
Message is one message read. ID is stable while the folder keeps its
UIDVALIDITY. Truncated says Raw holds the header only, because the
message is larger than the client reads whole; Size is its real size.
*/
type Message struct {
	ID           string
	Folder       mailbox.Folder
	RFCMessageID string
	Raw          []byte
	Size         int64
	Truncated    bool
}

/*
Batch is one call's messages, sent mail first, the cursor to ask with
next, and whether more messages wait right now.
*/
type Batch struct {
	Messages []Message
	Cursor   []byte
	More     bool
}

/*
Session is one signed-in connection.
*/
type Session struct {
	client   *imapclient.Client
	cancel   context.CancelFunc
	opts     Options
	received string
	sent     string
}

func (s *Session) start(creds Credentials) error {
	if err := s.client.Login(creds.Username, creds.Password).Wait(); err != nil {
		return commandError(err)
	}

	options := &imap.ListOptions{ReturnSpecialUse: s.client.Caps().Has(imap.CapSpecialUse)}

	folders, err := s.client.List("", "*", options).Collect()
	if err != nil {
		return fmt.Errorf("%w: list folders: %w", ErrUnreachable, err)
	}

	s.sent = sentFolder(folders)

	return nil
}

/*
sentFolder picks the folder flagged as Sent, or else the first folder
with a usual name, or none.
*/
func sentFolder(folders []*imap.ListData) string {
	for _, folder := range folders {
		if slices.Contains(folder.Attrs, imap.MailboxAttrSent) {
			return folder.Mailbox
		}
	}

	for _, name := range sentNames() {
		for _, folder := range folders {
			if strings.EqualFold(folder.Mailbox, name) {
				return folder.Mailbox
			}
		}
	}

	return ""
}

/*
SentFolder returns the folder sent mail is read from and filed into,
empty when the mailbox has none.
*/
func (s *Session) SentFolder() string {
	return s.sent
}

/*
folders returns the folders read, Sent first when there is one.
*/
func (s *Session) folders() []folder {
	if s.sent == "" {
		return []folder{{s.received, mailbox.FolderReceived}}
	}

	return []folder{{s.sent, mailbox.FolderSent}, {s.received, mailbox.FolderReceived}}
}

/*
folder is a folder of the mailbox and what it holds.
*/
type folder struct {
	name string
	kind mailbox.Folder
}

/*
Append files a copy of a sent message into Sent, marked read. It is the
only change a session ever makes.

Errors: ErrNoSentFolder, or ErrUnreachable when the server refuses or
the connection breaks.
*/
func (s *Session) Append(ctx context.Context, raw []byte) error {
	if s.sent == "" {
		return ErrNoSentFolder
	}

	defer context.AfterFunc(ctx, func() { _ = s.client.Close() })()

	cmd := s.client.Append(s.sent, int64(len(raw)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen}, Time: s.opts.Clock.Now(),
	})

	if _, err := cmd.Write(raw); err != nil {
		return fmt.Errorf("%w: append: %w", ErrUnreachable, err)
	}

	if err := cmd.Close(); err != nil {
		return fmt.Errorf("%w: append: %w", ErrUnreachable, err)
	}

	if _, err := cmd.Wait(); err != nil {
		return fmt.Errorf("%w: append: %w", ErrUnreachable, err)
	}

	return nil
}

/*
Close signs out and closes the connection. Closing a session that is
already closed, or whose connection broke, does nothing more.
*/
func (s *Session) Close() error {
	defer s.cancel()

	select {
	case <-s.client.Closed():
		return nil
	default:
	}

	_ = s.client.Logout().Wait()

	if err := s.client.Close(); err != nil {
		return fmt.Errorf("close imap session: %w", err)
	}

	return nil
}
