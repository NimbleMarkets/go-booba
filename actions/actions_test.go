package actions

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestSharedFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/snapshots.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name     string
		Valid    bool
		Snapshot json.RawMessage
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := Decode(tc.Snapshot)
			if (err == nil) != tc.Valid {
				t.Fatalf("valid=%v error=%v", tc.Valid, err)
			}
		})
	}
}

func TestPublisherRetryDedupAndRefresh(t *testing.T) {
	var received []Snapshot
	fail := true
	p, err := NewPublisher("app", func(data []byte) error {
		if fail {
			return errors.New("disconnected")
		}
		s, e := Decode(data)
		if e != nil {
			return e
		}
		received = append(received, s)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s := Snapshot{Context: "list", Actions: []Action{}, Short: []string{}, Full: [][]string{}}
	if _, err = p.Publish(s); err == nil {
		t.Fatal("expected transport error")
	}
	fail = false
	if changed, err := p.Publish(s); err != nil || !changed {
		t.Fatal(changed, err)
	}
	if changed, err := p.Publish(s); err != nil || changed {
		t.Fatal(changed, err)
	}
	if err := p.Refresh(); err != nil {
		t.Fatal(err)
	}
	s.Context = "dialog"
	if _, err := p.Publish(s); err != nil {
		t.Fatal(err)
	}
	if len(received) != 3 || received[0].Revision != 1 || received[1].Revision != 1 || received[2].Revision != 2 {
		t.Fatalf("unexpected publications: %+v", received)
	}
}

func TestParseKeys(t *testing.T) {
	for _, s := range []string{"ctrl+c", "alt+enter", "shift+tab", "+", "ctrl++", " ", "pgup", "😀"} {
		if _, err := ParseKey(s); err != nil {
			t.Errorf("%q: %v", s, err)
		}
	}
	for _, s := range []string{"ctrl+ctrl+c", "cmd+x", "up/down", "hello", "\n"} {
		if _, err := ParseKey(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
