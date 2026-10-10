package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
)

// Publisher serializes changed JSON snapshots through a host-supplied transport.
// It does not write OSC sequences or print behind Bubble Tea's renderer.
// Send must not call back into this Publisher.
type Publisher struct {
	mu              sync.Mutex
	instance        string
	send            func([]byte) error
	content, latest []byte
	revision        uint64
}

func NewPublisher(instance string, send func([]byte) error) (*Publisher, error) {
	if !identifier.MatchString(instance) || send == nil {
		return nil, fmt.Errorf("invalid publisher configuration")
	}
	return &Publisher{instance: instance, send: send}, nil
}

// Publish overrides the supplied version, instance, and revision. A failed send
// does not consume a revision; the application can retry the same update.
func (p *Publisher) Publish(s Snapshot) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s.Version = Version
	s.Instance = p.instance
	s.Revision = 0
	if err := s.Validate(); err != nil {
		return false, err
	}
	content, err := json.Marshal(s)
	if err != nil {
		return false, err
	}
	if bytes.Equal(content, p.content) {
		return false, nil
	}
	if p.revision == MaxRevision {
		return false, fmt.Errorf("action revision exhausted")
	}
	s.Revision = p.revision + 1
	if err := s.Validate(); err != nil {
		return false, err
	}
	data, err := json.Marshal(s)
	if err != nil {
		return false, err
	}
	if err := p.send(append([]byte(nil), data...)); err != nil {
		return false, err
	}
	p.revision++
	p.content = content
	p.latest = data
	return true, nil
}

// Refresh republishes the latest snapshot for a fresh receiver after reconnect.
// The host must arrange the request; this package defines no wire query.
func (p *Publisher) Refresh() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.latest == nil {
		return nil
	}
	return p.send(append([]byte(nil), p.latest...))
}
