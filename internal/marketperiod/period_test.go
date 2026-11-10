package marketperiod

import (
	"encoding/json"
	"testing"
)

func TestMetadata(t *testing.T) {
	for _, raw := range []string{`{}`, `{"period":null}`, `{"period":"game"}`, `{"period":0}`} {
		if !Metadata(json.RawMessage(raw)) {
			t.Errorf("rejected full: %s", raw)
		}
	}
	for _, raw := range []string{`{"period":"1q"}`, `{"period":1}`, `{"period_id":"1h"}`, `{"periodID":"unknown"}`, `{"period":"game","period_id":"1q"}`} {
		if Metadata(json.RawMessage(raw)) {
			t.Errorf("accepted partial/unknown: %s", raw)
		}
	}
}
