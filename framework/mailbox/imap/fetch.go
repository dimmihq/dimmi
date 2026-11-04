/*
|--------------------------------------------------------------------------
| Fetching Messages
|--------------------------------------------------------------------------
|
| Sizes come first, so a message too large to read whole is read as its
| header only and the application can still show who wrote and what about.
| Every body section is fetched with PEEK, which leaves the read flag of
| the owner's mail untouched.
|
*/

package imap

import (
	"fmt"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/dimmihq/dimmi/framework/mailbox"
)

/*
fetch reads the messages, in the order of the UIDs.
*/
func (s *Session) fetch(kind mailbox.Folder, validity uint32, uids []imap.UID) ([]Message, error) {
	if len(uids) == 0 {
		return nil, nil
	}

	heads, err := s.collect(uids, &imap.FetchOptions{UID: true, RFC822Size: true, Envelope: true})
	if err != nil {
		return nil, err
	}

	whole, large := s.split(uids, heads)

	bodies, err := s.sections(whole, &imap.FetchItemBodySection{Peek: true})
	if err != nil {
		return nil, err
	}

	headers, err := s.sections(large, &imap.FetchItemBodySection{Specifier: imap.PartSpecifierHeader, Peek: true})
	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(uids))

	for _, uid := range uids {
		head, ok := heads[uid]
		if !ok {
			continue
		}

		message := Message{
			ID: fmt.Sprintf("%s:%d:%d", kind, validity, uid), Folder: kind, Size: head.RFC822Size,
			RFCMessageID: messageID(head.Envelope), Raw: bodies[uid],
		}
		if raw, truncated := headers[uid]; truncated {
			message.Raw, message.Truncated = raw, true
		}

		messages = append(messages, message)
	}

	return messages, nil
}

/*
split tells the messages read whole from those too large, read as their
header only.
*/
func (s *Session) split(uids []imap.UID, heads map[imap.UID]*imapclient.FetchMessageBuffer) ([]imap.UID, []imap.UID) {
	var whole, large []imap.UID

	for _, uid := range uids {
		if head, ok := heads[uid]; ok && s.opts.MaxMessageBytes > 0 && head.RFC822Size > s.opts.MaxMessageBytes {
			large = append(large, uid)

			continue
		}

		whole = append(whole, uid)
	}

	return whole, large
}

func messageID(envelope *imap.Envelope) string {
	if envelope == nil {
		return ""
	}

	return strings.Trim(envelope.MessageID, "<> ")
}

/*
sections reads one body section of each message.
*/
func (s *Session) sections(uids []imap.UID, section *imap.FetchItemBodySection) (map[imap.UID][]byte, error) {
	out := map[imap.UID][]byte{}
	if len(uids) == 0 {
		return out, nil
	}

	found, err := s.collect(uids, &imap.FetchOptions{UID: true, BodySection: []*imap.FetchItemBodySection{section}})
	if err != nil {
		return nil, err
	}

	for uid, message := range found {
		out[uid] = message.FindBodySection(section)
	}

	return out, nil
}

func (s *Session) collect(
	uids []imap.UID, options *imap.FetchOptions,
) (map[imap.UID]*imapclient.FetchMessageBuffer, error) {
	found, err := s.client.Fetch(imap.UIDSetNum(uids...), options).Collect()
	if err != nil {
		return nil, fmt.Errorf("%w: fetch: %w", ErrUnreachable, err)
	}

	out := make(map[imap.UID]*imapclient.FetchMessageBuffer, len(found))
	for _, message := range found {
		out[message.UID] = message
	}

	return out, nil
}
