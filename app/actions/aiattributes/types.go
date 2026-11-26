/*
|--------------------------------------------------------------------------
| Conversation Attribute Types
|--------------------------------------------------------------------------
|
| A conversation attribute holds one of a dozen kinds of value, checked
| by the same rules ticket fields use. The AI fills only the kinds it
| can read off a conversation: text, numbers, a yes or no, a date and
| one of a list. Files, several choices, a moment and links to contacts
| and companies are a teammate's to fill.
|
*/

package aiattributes

import (
	"slices"

	"github.com/dimmihq/dimmi/app/ai/aidecision"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/fieldvalues"
)

/*
The data types of a conversation attribute.
*/
const (
	TypeText           = "text"
	TypeLongText       = "long_text"
	TypeNumber         = "number"
	TypeDecimal        = "decimal"
	TypeBoolean        = "boolean"
	TypeDate           = "date"
	TypeDateTime       = "datetime"
	TypeChoice         = "choice"
	TypeMultipleChoice = "multiple_choice"
	TypeFile           = "file"
	TypeContact        = "contact"
	TypeCompany        = "company"
)

/*
Who fills an attribute: the AI, teammates, or both.
*/
const (
	FilledByAI        = "ai"
	FilledByTeammates = "teammates"
	FilledByBoth      = "both"
)

/*
maxText is the longest single-line text value.
*/
const maxText = 500

/*
DataTypes returns every data type in the order the settings offer them.
*/
func DataTypes() []string {
	return []string{
		TypeText, TypeLongText, TypeNumber, TypeDecimal, TypeChoice, TypeMultipleChoice, TypeBoolean, TypeDate,
		TypeDateTime, TypeFile, TypeContact, TypeCompany,
	}
}

/*
AIFillable reports whether the AI can fill a type.
*/
func AIFillable(dataType string) bool {
	return slices.Contains(aidecision.Types(), aiType(dataType))
}

/*
aiType is the type the decision step reads a data type as.
*/
func aiType(dataType string) string {
	switch dataType {
	case TypeLongText:
		return aidecision.TypeText
	case TypeDecimal:
		return aidecision.TypeNumber
	}

	return dataType
}

/*
choosable reports whether a type takes options.
*/
func choosable(dataType string) bool {
	return dataType == TypeChoice || dataType == TypeMultipleChoice
}

/*
byAI reports whether the AI fills an attribute.
*/
func byAI(a models.AiConversationAttribute) bool {
	return a.FilledBy != FilledByTeammates && AIFillable(a.DataType)
}

/*
fit makes a value the attribute's type, or says why it does not fit.
*/
func fit(a models.AiConversationAttribute, value any) (any, string) {
	switch a.DataType {
	case TypeText:
		return fieldvalues.Text(value, maxText)
	case TypeLongText:
		return fieldvalues.Text(value, fieldvalues.MaxLongText)
	case TypeChoice:
		return fieldvalues.Fit(fieldvalues.TypeSingleChoice, a.Options, value)
	}

	return fieldvalues.Fit(a.DataType, a.Options, value)
}
