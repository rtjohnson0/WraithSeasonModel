package bdl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestPaginationProgressCountsReceivedRows(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "private-test-key" {
			t.Error("missing authentication")
		}
		if r.URL.Query().Get("cursor") == "next" {
			fmt.Fprint(w, `{"data":[{"id":3}],"meta":{"next_cursor":null}}`)
		} else {
			fmt.Fprint(w, `{"data":[{"id":1},{"id":2}],"meta":{"next_cursor":"next"}}`)
		}
	}))
	defer server.Close()
	c := New("private-test-key")
	c.BaseURL, c.Sleep = server.URL, 0
	var messages []string
	c.Logf = func(format string, args ...any) { messages = append(messages, fmt.Sprintf(format, args...)) }
	rows := 0
	err := c.Paginate("/stats", url.Values{"secret": {"private-query"}}, func(json.RawMessage) error { rows++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	logs := strings.Join(messages, "\n")
	if rows != 3 || !strings.Contains(logs, "page 2 received, 1 rows on page, 3 rows total") {
		t.Fatalf("incorrect progress: rows=%d logs=%s", rows, logs)
	}
	if strings.Contains(logs, "private-test-key") || strings.Contains(logs, "private-query") {
		t.Fatal("progress exposed credentials or query values")
	}
}
