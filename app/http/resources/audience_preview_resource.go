/*
|--------------------------------------------------------------------------
| Audience Preview Resources
|--------------------------------------------------------------------------
|
| How previews, tests and the field list read over the API. A preview
| lists matching people only by name and email; a test shows each
| condition with the value the person has, so whoever reads it may
| already read contacts.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/audiences"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/audience"
)

/*
AudiencePerson is a person listed by a preview or a test.
*/
type AudiencePerson struct {
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	Email *string `json:"email"`
	Kind  string  `json:"kind"`
}

/*
AudiencePreview is how many people a rule matches.
*/
type AudiencePreview struct {
	Object        string             `json:"object"`
	Count         int64              `json:"count"`
	Pages         int64              `json:"pages"`
	TotalContacts int64              `json:"total_contacts"`
	Approximate   bool               `json:"approximate"`
	Sample        []AudiencePerson   `json:"sample"`
	Warnings      []audience.Warning `json:"warnings"`
}

/*
NewAudiencePreview shapes a preview.
*/
func NewAudiencePreview(p audiences.Preview) AudiencePreview {
	out := AudiencePreview{
		Object: "audience_preview", Count: p.Count, Pages: p.Pages, TotalContacts: p.Total,
		Approximate: p.Approximate, Sample: make([]AudiencePerson, 0, len(p.Sample)), Warnings: p.Warnings,
	}

	for _, c := range p.Sample {
		out.Sample = append(out.Sample, newAudiencePerson(c))
	}

	return out
}

/*
AudienceEvaluation is a rule tested on one person.
*/
type AudienceEvaluation struct {
	Object  string                `json:"object"`
	Result  string                `json:"result"`
	Contact AudiencePerson        `json:"contact"`
	Leaves  []audience.LeafResult `json:"leaves"`
}

/*
NewAudienceEvaluation shapes a test. The result reads match, no_match,
or depends_on_page when page conditions decide it.
*/
func NewAudienceEvaluation(e audiences.Evaluation) AudienceEvaluation {
	result := map[string]string{"true": "match", "false": "no_match"}[e.Result]
	if result == "" {
		result = "depends_on_page"
	}

	return AudienceEvaluation{
		Object: "audience_evaluation", Result: result, Contact: newAudiencePerson(e.Contact), Leaves: e.Leaves,
	}
}

/*
AudienceField is a field a rule may read.
*/
type AudienceField struct {
	Key     string   `json:"key"`
	Type    string   `json:"type"`
	Where   string   `json:"where"`
	Trust   string   `json:"trust"`
	Options []string `json:"options,omitempty"`
	Ops     []string `json:"ops"`
	Custom  bool     `json:"custom"`
}

/*
NewAudienceField shapes a field.
*/
func NewAudienceField(f audiences.Field) AudienceField {
	return AudienceField{
		Key: f.Key, Type: string(f.Type), Where: string(f.Where), Trust: string(f.Trust), Options: f.Options,
		Ops: f.Ops, Custom: f.Custom,
	}
}

func newAudiencePerson(c models.Contact) AudiencePerson {
	return AudiencePerson{ID: models.PublicID(models.PrefixContact, c.ID), Name: c.Name, Email: c.Email, Kind: c.Kind}
}
