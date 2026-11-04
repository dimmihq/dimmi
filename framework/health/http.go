/*
|--------------------------------------------------------------------------
| Health Endpoints
|--------------------------------------------------------------------------
|
| Two questions get two endpoints. Liveness asks whether the process is
| running at all; it answers without touching any dependency, so a slow
| database never gets a healthy process restarted. Readiness asks whether
| the process can serve traffic; it runs the checks, and a failure takes
| the instance out of the load balancer until it recovers.
|
| The readiness body names each check and its status but never the
| detail, since the endpoint is reachable from outside and details carry
| addresses and error text.
|
*/

package health

import (
	"encoding/json"
	"net/http"
)

/*
Up returns the liveness handler. It answers 200 for as long as the
process can serve a request, naming the release that runs when one is
given, so a deploy can check which build answers.
*/
func Up(release string) http.Handler {
	body := map[string]string{"status": "up"}
	if release != "" {
		body["release"] = release
	}

	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, body)
	})
}

/*
Ready returns the readiness handler. It runs the registry's checks and
answers 200 when none failed, else 503. The body holds the overall
status and each check's status by name.
*/
func Ready(registry *Registry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		report := registry.Run(r.Context())

		checks := make(map[string]Status, len(report.Outcomes))
		for _, outcome := range report.Outcomes {
			checks[outcome.Name] = outcome.Result.Status
		}

		code := http.StatusOK
		if report.Status == Fail {
			code = http.StatusServiceUnavailable
		}

		writeJSON(w, code, readyBody{Status: report.Status, Checks: checks})
	})
}

type readyBody struct {
	Status Status            `json:"status"`
	Checks map[string]Status `json:"checks"`
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
