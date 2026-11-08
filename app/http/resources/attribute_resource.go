/*
|--------------------------------------------------------------------------
| Attribute Definition Resources
|--------------------------------------------------------------------------
|
| How attribute definitions and dropped writes read over the API. A
| problem never carries the value that was dropped, only its shape.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/attributes"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Attribute is a definition.
*/
type Attribute struct {
	ID             string          `json:"id"`
	Object         string          `json:"object"`
	Of             string          `json:"of"`
	Key            string          `json:"key"`
	Label          string          `json:"label"`
	Description    *string         `json:"description"`
	DataType       string          `json:"data_type"`
	Options        json.RawMessage `json:"options"`
	Origin         string          `json:"origin"`
	Status         string          `json:"status"`
	ClientWritable bool            `json:"client_writable"`
	LastValueAt    *time.Time      `json:"last_value_at"`
	CreatedAt      time.Time       `json:"created_at"`
}

/*
NewAttribute shapes a definition. Of says whose attribute it is.
*/
func NewAttribute(d attributes.Definition) Attribute {
	return Attribute{
		ID: models.PublicID(models.PrefixAttribute, d.ID), Object: "attribute_definition",
		Of:  d.Object,
		Key: d.Key, Label: d.Label, Description: d.Description, DataType: d.DataType, Options: d.Options,
		Origin: d.Origin, Status: d.Status, ClientWritable: d.ClientWritable, LastValueAt: d.LastValueAt,
		CreatedAt: d.CreatedAt,
	}
}

/*
AttributeProblem is a group of dropped writes.
*/
type AttributeProblem struct {
	Object string    `json:"object"`
	Of     string    `json:"of"`
	Key    string    `json:"key"`
	Code   string    `json:"code"`
	Count  int64     `json:"count"`
	Sample *string   `json:"sample"`
	LastAt time.Time `json:"last_at"`
}

/*
NewAttributeProblem shapes a problem.
*/
func NewAttributeProblem(p models.AttributeProblem) AttributeProblem {
	return AttributeProblem{
		Object: "attribute_problem", Of: p.Object, Key: p.Key, Code: p.Code, Count: p.Count, Sample: p.Sample,
		LastAt: p.LastAt,
	}
}
