package intercom

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

/*
Source reads an Intercom workspace. Workflows lists the workflows to
export by their ruleset id, the number at the end of a workflow's URL,
since Intercom has no way to list them. Workers is how many records are
read in parallel within a page.
*/
type Source struct {
	Client    *Client
	Region    string
	Workflows []string
	Workers   int

	mu          sync.Mutex
	account     *engine.Account
	known       map[string]bool
	knownTeams  map[string]bool
	knownTags   map[string]bool
	former      map[string]Former
	centers     map[string]string
	defaultHC   string
	defaultLang string
}

/*
New returns a source over the client.
*/
func New(c *Client, region string, workflows []string) *Source {
	return &Source{Client: c, Region: region, Workflows: workflows, Workers: 8}
}

/*
Name is the source's name, intercom.
*/
func (s *Source) Name() string { return "intercom" }

/*
Kinds lists what is read, in import order.
*/
func (s *Source) Kinds() []format.Kind {
	kinds := []format.Kind{
		format.KindTeammate, format.KindTeam, format.KindTag, format.KindAttributeDefinition, format.KindCompany,
		format.KindContact, format.KindContactNote, format.KindTicketType, format.KindHelpCenter, format.KindCollection,
		format.KindArticle, format.KindSnippet, format.KindSavedReply, format.KindSchedule, format.KindConversation,
		format.KindTicket,
	}

	if len(s.Workflows) > 0 {
		kinds = append(kinds, format.KindWorkflow)
	}

	return kinds
}

/*
Account reads the workspace the token belongs to.
*/
func (s *Source) Account(ctx context.Context) (engine.Account, error) {
	s.mu.Lock()
	if s.account != nil {
		a := *s.account
		s.mu.Unlock()

		return a, nil
	}
	s.mu.Unlock()

	var m me
	if err := s.Client.get(ctx, "/me", nil, &m); err != nil {
		return engine.Account{}, err
	}

	a := engine.Account{ID: m.App.IDCode, Name: text(m.App.Name), Region: s.Region}
	if a.ID == "" {
		return a, errors.New("intercom didn't say which workspace this token belongs to")
	}

	s.mu.Lock()
	s.account = &a
	s.mu.Unlock()

	return a, nil
}

/*
Open downloads a file.
*/
func (s *Source) Open(ctx context.Context, f *format.File) (io.ReadCloser, error) {
	return s.Client.Open(ctx, f.Source)
}

/*
reader reads one kind page by page, from the cursor the options carry.
*/
type reader func(ctx context.Context, opts engine.ReadOptions, emit func(engine.Page) error) error

/*
readers maps each kind onto its reader.
*/
func (s *Source) readers() map[format.Kind]reader {
	return map[format.Kind]reader{
		format.KindTeammate:            s.readTeammates,
		format.KindTeam:                s.readTeams,
		format.KindTag:                 s.readTags,
		format.KindAttributeDefinition: s.readAttributes,
		format.KindCompany:             s.readCompanies,
		format.KindContact:             s.readContacts,
		format.KindContactNote:         s.readContactNotes,
		format.KindTicketType:          s.readTicketTypes,
		format.KindHelpCenter:          s.readHelpCenters,
		format.KindCollection:          s.readCollections,
		format.KindArticle:             s.readArticles,
		format.KindSnippet:             s.readSnippets,
		format.KindSavedReply:          s.readMacros,
		format.KindSchedule:            s.readSchedules,
		format.KindConversation:        s.readConversations,
		format.KindTicket:              s.readTickets,
		format.KindWorkflow:            s.readWorkflows,
	}
}

/*
Read reads one kind page by page.
*/
func (s *Source) Read(
	ctx context.Context, kind format.Kind, opts engine.ReadOptions, emit func(engine.Page) error,
) error {
	if read, ok := s.readers()[kind]; ok {
		return read(ctx, opts, func(p engine.Page) error {
			recs, err := s.guard(ctx, p.Records)
			if err != nil {
				return err
			}

			p.Records = recs

			return emit(p)
		})
	}

	return fmt.Errorf("intercom has no %s", kind)
}

func one(emit func(engine.Page) error, recs []format.Record) error {
	return emit(engine.Page{Records: recs})
}

/*
parallel runs fn over items with n at a time, keeping their order.
*/
func parallel[T any](
	ctx context.Context, n int, items []T, fn func(context.Context, T) ([]format.Record, error),
) ([]format.Record, error) {
	if n <= 0 {
		n = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([][]format.Record, len(items))
	errs := make([]error, len(items))
	sem := make(chan struct{}, n)

	var wg sync.WaitGroup

	for i, item := range items {
		wg.Add(1)
		sem <- struct{}{}

		go func(i int, item T) {
			defer wg.Done()
			defer func() { <-sem }()

			results[i], errs[i] = fn(ctx, item)
			if errs[i] != nil {
				cancel()
			}
		}(i, item)
	}

	wg.Wait()

	var out []format.Record

	for i := range items {
		if errs[i] != nil && !errors.Is(errs[i], context.Canceled) {
			return nil, errs[i]
		}

		out = append(out, results[i]...)
	}

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}
