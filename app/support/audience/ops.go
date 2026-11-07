/*
|--------------------------------------------------------------------------
| Operators
|--------------------------------------------------------------------------
|
| One table says which operators each field type takes and what value
| each operator expects. A negative operator also matches people with
| no value at all, so "plan is not pro" includes everyone without a
| plan; the builder says so beside every negative condition.
|
*/

package audience

import "slices"

/*
The operators.
*/
const (
	OpEq                    = "eq"
	OpNeq                   = "neq"
	OpIn                    = "in"
	OpNotIn                 = "not_in"
	OpContains              = "contains"
	OpNotContains           = "not_contains"
	OpStartsWith            = "starts_with"
	OpEndsWith              = "ends_with"
	OpContainsAny           = "contains_any"
	OpNotContainsAny        = "not_contains_any"
	OpExists                = "exists"
	OpNotExists             = "not_exists"
	OpGt                    = "gt"
	OpGte                   = "gte"
	OpLt                    = "lt"
	OpLte                   = "lte"
	OpBetween               = "between"
	OpIsTrue                = "is_true"
	OpIsFalse               = "is_false"
	OpBefore                = "before"
	OpAfter                 = "after"
	OpOn                    = "on"
	OpWithinLastDays        = "within_last_days"
	OpMoreThanDaysAgo       = "more_than_days_ago"
	OpDaysAgo               = "days_ago"
	OpWithinNextDays        = "within_next_days"
	OpMoreThanDaysFromNow   = "more_than_days_from_now"
	OpHas                   = "has"
	OpHasAny                = "has_any"
	OpHasNone               = "has_none"
	OpMatches               = "matches"
	valueNone               = "none"
	valueText               = "text"
	valueTexts              = "texts"
	valueNumber             = "number"
	valueRange              = "range"
	valueDays               = "days"
	valueDate               = "date"
	maxKeywordsPerCondition = 50
)

/*
opsFor returns the operators a field type takes.
*/
func opsFor(t FieldType) []string {
	presence := []string{OpExists, OpNotExists}
	switch t {
	case TypeString:
		return append([]string{
			OpEq, OpNeq, OpIn, OpNotIn, OpContains, OpNotContains, OpStartsWith, OpEndsWith,
			OpContainsAny, OpNotContainsAny,
		}, presence...)
	case TypeNumber:
		return append([]string{OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte, OpBetween}, presence...)
	case TypeBool:
		return append([]string{OpIsTrue, OpIsFalse}, presence...)
	case TypeDate:
		return append([]string{
			OpBefore, OpAfter, OpOn, OpWithinLastDays, OpMoreThanDaysAgo, OpDaysAgo,
			OpWithinNextDays, OpMoreThanDaysFromNow,
		}, presence...)
	case TypeList, TypeID:
		return append([]string{OpEq, OpNeq, OpIn, OpNotIn}, presence...)
	case TypeSet:
		return []string{OpHas, OpHasAny, OpHasNone}
	case TypeURL:
		return []string{OpEq, OpContains, OpNotContains, OpStartsWith, OpMatches}
	case TypeSaved:
		return []string{OpIn, OpNotIn}
	case TypeAuto:
		return autoOps()
	}

	return nil
}

/*
autoOps is every operator of the value types a field of unknown type
may hold.
*/
func autoOps() []string {
	var all []string
	for _, t := range []FieldType{TypeString, TypeNumber, TypeBool, TypeDate} {
		for _, op := range opsFor(t) {
			if !slices.Contains(all, op) {
				all = append(all, op)
			}
		}
	}

	return all
}

/*
allowedOps returns the operators a registered field takes.
*/
func allowedOps(f Field) []string {
	if len(f.Ops) > 0 {
		return f.Ops
	}

	return opsFor(f.Type)
}

/*
valueShape says what value an operator expects.
*/
func valueShape(op string) string {
	switch op {
	case OpExists, OpNotExists, OpIsTrue, OpIsFalse:
		return valueNone
	case OpIn, OpNotIn, OpContainsAny, OpNotContainsAny, OpHasAny, OpHasNone:
		return valueTexts
	case OpGt, OpGte, OpLt, OpLte:
		return valueNumber
	case OpBetween:
		return valueRange
	case OpWithinLastDays, OpMoreThanDaysAgo, OpDaysAgo, OpWithinNextDays, OpMoreThanDaysFromNow:
		return valueDays
	case OpBefore, OpAfter, OpOn:
		return valueDate
	}

	return valueText
}

/*
negative reports whether an operator is true when there is no value.
*/
func negative(op string) bool {
	switch op {
	case OpNotExists, OpNeq, OpNotIn, OpNotContains, OpNotContainsAny, OpHasNone:
		return true
	}

	return false
}
