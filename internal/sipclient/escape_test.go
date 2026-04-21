package sipclient

import "testing"

func TestSOLTracker(t *testing.T) {
	tr := NewSOLTracker()
	if !tr.AtStart() {
		t.Fatalf("initial state should be AtStart=true")
	}
	tr.Observe([]byte("hello"))
	if tr.AtStart() {
		t.Errorf("after 'hello', AtStart should be false")
	}
	tr.Observe([]byte("\r"))
	if !tr.AtStart() {
		t.Errorf("after '\\r', AtStart should be true")
	}
	tr.Observe([]byte("x"))
	if tr.AtStart() {
		t.Errorf("after 'x', AtStart should be false")
	}
	tr.Observe([]byte("\n"))
	if !tr.AtStart() {
		t.Errorf("after '\\n', AtStart should be true")
	}
	tr.Observe([]byte("ab\r\ncd"))
	if tr.AtStart() {
		t.Errorf("after 'ab\\r\\ncd', AtStart should be false (cd terminates the line)")
	}
	tr.Observe([]byte{})
	if tr.AtStart() {
		t.Errorf("empty Observe should not change state")
	}
}
