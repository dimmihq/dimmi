package intercom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

/*
strictDimmi holds what it takes like dimmi's migrations API, and
refuses what dimmi would put off or reject: a record naming one it
doesn't hold yet, or a file that wasn't uploaded.
*/
type strictDimmi struct {
	mu       sync.Mutex
	t        *testing.T
	held     map[format.Kind]map[string]json.RawMessage
	files    map[string]string
	waiting  map[string]waitingRecord
	deferred int
	upserts  atomic.Int32
	crashAt  int32
	cancel   context.CancelFunc
}

/*
waitingRecord is a record dimmi put off until what it names arrives.
*/
type waitingRecord struct {
	kind format.Kind
	id   string
	raw  json.RawMessage
}

func newStrictDimmi(t *testing.T) *strictDimmi {
	t.Helper()

	return &strictDimmi{
		t: t, held: make(map[format.Kind]map[string]json.RawMessage), files: map[string]string{},
		waiting: map[string]waitingRecord{},
	}
}

func (d *strictDimmi) Start(context.Context, engine.Start) (engine.Migration, error) {
	return engine.Migration{ID: "mig_sim"}, nil
}

func (d *strictDimmi) Get(context.Context, string) (engine.Migration, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	m := engine.Migration{ID: "mig_sim", Counts: make(map[format.Kind]engine.Counts)}
	for k, held := range d.held {
		m.Counts[k] = engine.Counts{Created: len(held)}
	}

	return m, nil
}

func (d *strictDimmi) Rollback(context.Context, string) (engine.Migration, error) {
	return engine.Migration{}, errors.New("not in the simulation")
}

func (d *strictDimmi) Lookup(_ context.Context, _ string, kind format.Kind, ids []string) (map[string]bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := map[string]bool{}
	for _, id := range ids {
		_, out[id] = d.held[kind][id]
	}

	return out, nil
}

func (d *strictDimmi) Upload(_ context.Context, _ string, u engine.Upload) (string, error) {
	raw, err := io.ReadAll(u.Body)
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(string(raw), simFile) || int64(len(raw)) != u.Size {
		return "", fmt.Errorf("file %s arrived as %q", u.SourceID, raw)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	id := fmt.Sprintf("att_%d", len(d.files)+1)
	d.files[id] = u.SourceID

	return id, nil
}

/*
refKind gives, for a field that names another record, the record's
kind.
*/
func refKind(key string) (format.Kind, bool) {
	switch key {
	case "contact_ref", "participant_refs":
		return format.KindContact, true
	case "company_ref":
		return format.KindCompany, true
	case "tag_refs", "added_tag_refs", "removed_tag_refs":
		return format.KindTag, true
	case "assignee_ref", "teammate_ref", "owner_ref", "member_refs":
		return format.KindTeammate, true
	case "team_ref", "team_refs":
		return format.KindTeam, true
	case "conversation_ref":
		return format.KindConversation, true
	case "ticket_type_ref":
		return format.KindTicketType, true
	case "help_center_ref":
		return format.KindHelpCenter, true
	case "collection_ref":
		return format.KindCollection, true
	default:
		return "", false
	}
}

/*
missing lists the records a record names that dimmi doesn't hold, and
attachments that weren't uploaded.
*/
func (d *strictDimmi) missing(v any, key string, out *[]string) {
	switch x := v.(type) {
	case map[string]any:
		if key == "author" {
			d.missingAuthor(x, out)
		}

		for k, child := range x {
			d.missing(child, k, out)
		}
	case []any:
		for _, child := range x {
			d.missing(child, key, out)
		}
	case string:
		d.missingRef(x, key, out)
	}
}

/*
missingAuthor lists an author dimmi doesn't hold.
*/
func (d *strictDimmi) missingAuthor(a map[string]any, out *[]string) {
	t, ok := a["type"].(string)
	if !ok {
		return
	}

	ref, _ := a["ref"].(string)

	switch {
	case t == format.AuthorTeammate && d.held[format.KindTeammate][ref] == nil:
		*out = append(*out, "teammate "+ref)
	case t == format.AuthorContact && d.held[format.KindContact][ref] == nil:
		*out = append(*out, "contact "+ref)
	}
}

/*
missingRef lists the record or the file an id names that dimmi doesn't
hold.
*/
func (d *strictDimmi) missingRef(id, key string, out *[]string) {
	if kind, ok := refKind(key); ok && id != "" && d.held[kind][id] == nil {
		*out = append(*out, string(kind)+" "+id)
	}

	if key == "attachments" && d.files[id] == "" {
		*out = append(*out, "file "+id)
	}
}

func (d *strictDimmi) Upsert(
	_ context.Context, _ string, kind format.Kind, recs []format.Record,
) ([]engine.Result, error) {
	if n := d.upserts.Add(1); d.crashAt > 0 && n == d.crashAt {
		d.cancel()

		return nil, context.Canceled
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	out := make([]engine.Result, 0, len(recs))

	for _, r := range recs {
		if r.Kind != kind {
			d.t.Errorf("a %s record sent in a %s batch", r.Kind, kind)
		}

		raw, err := json.Marshal(r.Data)
		if err != nil {
			return nil, err
		}

		out = append(out, d.apply(kind, r.SourceID, raw))
	}

	d.resolve()

	return out, nil
}

/*
apply takes a record, or puts it off with what it waits for, as dimmi
does.
*/
func (d *strictDimmi) apply(kind format.Kind, id string, raw json.RawMessage) engine.Result {
	var v any
	_ = json.Unmarshal(raw, &v)

	var gaps []string

	d.missing(v, "", &gaps)

	if kind == format.KindArticle || kind == format.KindCollection {
		gaps = slices.DeleteFunc(gaps, func(g string) bool { return strings.HasPrefix(g, "collection ") })
	}

	key := string(kind) + " " + id

	if len(gaps) > 0 {
		d.deferred++
		d.waiting[key] = waitingRecord{kind: kind, id: id, raw: raw}

		var refs []engine.Ref

		for _, g := range gaps {
			k, ref, _ := strings.Cut(g, " ")
			refs = append(refs, engine.Ref{Kind: format.Kind(k), SourceID: ref})
		}

		return engine.Result{SourceID: id, Status: engine.StatusDeferred, WaitingFor: refs}
	}

	delete(d.waiting, key)

	if d.held[kind] == nil {
		d.held[kind] = map[string]json.RawMessage{}
	}

	status := engine.StatusCreated
	if _, ok := d.held[kind][id]; ok {
		status = engine.StatusUpdated
	}

	d.held[kind][id] = raw

	return engine.Result{SourceID: id, Status: status, ID: "rec_" + id}
}

/*
resolve applies the waiting records whose names have all arrived.
*/
func (d *strictDimmi) resolve() {
	for range 5 {
		progressed := false

		for _, w := range maps.Clone(d.waiting) {
			if d.apply(w.kind, w.id, w.raw).Status != engine.StatusDeferred {
				progressed = true
			} else {
				d.deferred--
			}
		}

		if !progressed {
			return
		}
	}
}
