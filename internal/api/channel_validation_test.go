package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWritesRejectInvalidChannelsBeforeStore(t *testing.T) {
	router := NewRouter(nil)
	for _, endpoint := range []string{"add", "update"} {
		for _, body := range []string{`{"channel":[]}`, `{"channel":[""]}`, `{"channel":["unknown"]}`, `{"channel":["system",""]}`, `{}`} {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/"+endpoint, strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("%s %s: status %d, body %s", endpoint, body, recorder.Code, recorder.Body.String())
			}
		}
	}
}
