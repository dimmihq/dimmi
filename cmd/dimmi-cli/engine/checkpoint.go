package engine

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/format"
)

/*
KindState is how far a kind got. Cursor resumes an unfinished read;
ReadFrom is when the last complete read began, which the next sync
reads from.
*/
type KindState struct {
	Done      bool      `json:"done"`
	Cursor    string    `json:"cursor,omitempty"`
	StartedAt time.Time `json:"started_at,omitzero"`
	ReadFrom  time.Time `json:"read_from,omitzero"`
	Sent      int       `json:"sent"`
	Skipped   int       `json:"skipped"`
	Failed    int       `json:"failed"`
}

/*
Failure is a record dimmi refused, kept so status can list it.
*/
type Failure struct {
	Kind     format.Kind `json:"kind"`
	SourceID string      `json:"source_id"`
	Code     string      `json:"code"`
	Message  string      `json:"message"`
}

/*
State is a migration's local checkpoint. It holds ids, cursors and
counts, never customer content.
*/
type State struct {
	MigrationID string                       `json:"migration_id"`
	Source      string                       `json:"source"`
	Account     Account                      `json:"account"`
	Kinds       map[format.Kind]*KindState   `json:"kinds"`
	Files       map[string]string            `json:"files,omitempty"`
	Failures    []Failure                    `json:"failures,omitempty"`
	Skip        []format.Kind                `json:"skip,omitempty"`
	UpdatedAt   time.Time                    `json:"updated_at"`
	SyncedAt    time.Time                    `json:"synced_at,omitzero"`
	Extra       map[string]map[string]string `json:"extra,omitempty"`
}

/*
Kind returns the kind's state, adding it when missing.
*/
func (s *State) Kind(kind format.Kind) *KindState {
	if s.Kinds == nil {
		s.Kinds = map[format.Kind]*KindState{}
	}

	if s.Kinds[kind] == nil {
		s.Kinds[kind] = &KindState{}
	}

	return s.Kinds[kind]
}

/*
Skips reports whether the plan skips the kind.
*/
func (s *State) Skips(kind format.Kind) bool {
	for _, k := range s.Skip {
		if k == kind {
			return true
		}
	}

	return false
}

/*
Store keeps a migration's checkpoint in a folder: state.json, rewritten
atomically, and hashes.log, an append-only list of what each record
looked like when dimmi last took it, so unchanged records aren't sent
again.
*/
type Store struct {
	Dir string

	mu     sync.Mutex
	hashes map[string]string
	log    *os.File
}

/*
OpenStore opens the checkpoint folder, creating it private to the user.
*/
func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	s := &Store{Dir: dir, hashes: map[string]string{}}

	if err := s.loadHashes(); err != nil {
		return nil, err
	}

	return s, nil
}

/*
Load reads the state, or returns nil when there is none.
*/
func (s *Store) Load() (*State, error) {
	raw, err := os.ReadFile(filepath.Join(s.Dir, "state.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var st State
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, fmt.Errorf("checkpoint %s: %w", s.Dir, err)
	}

	return &st, nil
}

/*
Save writes the state atomically.
*/
func (s *Store) Save(st *State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st.UpdatedAt = time.Now().UTC()

	raw, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}

	tmp := filepath.Join(s.Dir, "state.json.tmp")
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}

	if s.log != nil {
		if err := s.log.Sync(); err != nil {
			return err
		}
	}

	return os.Rename(tmp, filepath.Join(s.Dir, "state.json"))
}

/*
Reset forgets everything: the state and the hashes.
*/
func (s *Store) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.log != nil {
		_ = s.log.Close()
		s.log = nil
	}

	s.hashes = map[string]string{}

	for _, name := range []string{"state.json", "hashes.log"} {
		if err := os.Remove(filepath.Join(s.Dir, name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	return nil
}

/*
Close closes the hash log.
*/
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.log == nil {
		return nil
	}

	err := s.log.Close()
	s.log = nil

	return err
}

func hashKey(kind format.Kind, id string) string { return string(kind) + "\t" + id }

/*
Unchanged reports whether dimmi already holds the record as it is now.
*/
func (s *Store) Unchanged(kind format.Kind, id, hash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.hashes[hashKey(kind, id)] == hash
}

/*
Taken records that dimmi took the record as it is now.
*/
func (s *Store) Taken(kind format.Kind, id, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := hashKey(kind, id)
	if s.hashes[key] == hash {
		return nil
	}

	if s.log == nil {
		f, err := os.OpenFile(filepath.Join(s.Dir, "hashes.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return err
		}

		s.log = f
	}

	if _, err := fmt.Fprintf(s.log, "%s\t%s\n", key, hash); err != nil {
		return err
	}

	s.hashes[key] = hash

	return nil
}

/*
Seen lists the source ids dimmi took for the kind.
*/
func (s *Store) Seen(kind format.Kind) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefix := string(kind) + "\t"
	var out []string

	for key := range s.hashes {
		if id, ok := strings.CutPrefix(key, prefix); ok {
			out = append(out, id)
		}
	}

	return out
}

func (s *Store) loadHashes() error {
	f, err := os.Open(filepath.Join(s.Dir, "hashes.log"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)

	for sc.Scan() {
		parts := strings.Split(sc.Text(), "\t")
		if len(parts) == 3 {
			s.hashes[parts[0]+"\t"+parts[1]] = parts[2]
		}
	}

	return sc.Err()
}

/*
Hash fingerprints a record as it would be sent, so a record that didn't
change isn't sent again.
*/
func Hash(rec format.Record) (string, error) {
	raw, err := json.Marshal(rec.Data)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(raw)

	return hex.EncodeToString(sum[:16]), nil
}
