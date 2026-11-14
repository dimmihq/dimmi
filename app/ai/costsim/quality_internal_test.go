package costsim

import "testing"

func TestHitMatchesTheArticleBySlug(t *testing.T) {
	if !hit([]string{"https://help.example.com/articles/adding-a-store"}, []string{"adding-a-store"}) {
		t.Fatal("the expected article was kept")
	}
	if hit([]string{"https://help.example.com/articles/adding-a-store-page"}, []string{"adding-a-store"}) {
		t.Fatal("a longer slug is another article")
	}
}

func TestQualitySummary(t *testing.T) {
	q := &QualityReport{Items: []QualityResult{
		{Hit: true, Graded: true, Grade: Grade{Score: 5, Correct: true}, CostMicros: 100},
		{Graded: true, Grade: Grade{Score: 2, Contradicts: true}, CostMicros: 300},
		{Failed: true, Graded: true, Grade: Grade{Score: 1}, CostMicros: 900},
	}}
	q.summarize()
	if q.HitRate != 0.5 || q.AvgScore != 3.5 || q.CorrectShare != 0.5 || q.Contradicting != 1 ||
		q.AnswerMicros != 200 || q.Failed != 1 {
		t.Fatalf("summary = %+v", q)
	}
}
