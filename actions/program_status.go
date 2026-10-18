package actions

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// These names identify an opt-in, private convention within ordinary OSC 7501
// reports. They are not part of the Program Status Protocol specification.
const StatusNamespace = "booba-actions"
const MessageMarker = " | booba-actions/v1 "

// MaxStatusMessageBytes is OSC 7501's decoded msg budget, including summary,
// MessageMarker and serialized JSON (UTF-8 bytes, before base64 encoding).
const MaxStatusMessageBytes = 2048
const QueryProgramStatus = "\x1b]7501;?\x1b\\"

// ProgramStatus supplies the human-facing part of an action-bearing report.
// State is idle, working, or blocked; Kind is meaningful only for blocked.
type ProgramStatus struct {
	State   string
	Kind    string
	App     string
	Summary string
}

// StatusID keeps the model's instance separate from the IDs of individual
// actions, splitting long instances to respect OSC 7501's 32-byte segments.
func StatusID(instance string) (string, error) {
	if !identifier.MatchString(instance) {
		return "", fmt.Errorf("invalid action instance")
	}
	if len(instance) > 32 {
		return StatusNamespace + "/" + instance[:32] + "/" + instance[32:], nil
	}
	return StatusNamespace + "/" + instance, nil
}

// EncodeProgramStatus carries a snapshot after readable text in the msg field.
// Other hosts may display the entire message; truncation is not guaranteed.
func EncodeProgramStatus(s Snapshot, status ProgramStatus) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	if status.State == "" {
		status.State = "idle"
	}
	if status.State != "idle" && status.State != "working" && status.State != "blocked" {
		return "", fmt.Errorf("invalid action status state")
	}
	if status.Kind != "" && (status.State != "blocked" || (status.Kind != "permission" && status.Kind != "question" && status.Kind != "auth")) {
		return "", fmt.Errorf("invalid action status kind")
	}
	if status.App != "" && (!identifier.MatchString(status.App) || len(status.App) > 32) {
		return "", fmt.Errorf("invalid status app")
	}
	if status.Summary == "" || !validText(status.Summary, 256) || strings.Contains(status.Summary, MessageMarker) {
		return "", fmt.Errorf("invalid status summary")
	}
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	message := status.Summary + MessageMarker + string(data)
	if len(message) > MaxStatusMessageBytes {
		return "", fmt.Errorf("action status message exceeds %d decoded bytes (got %d)", MaxStatusMessageBytes, len(message))
	}
	id, _ := StatusID(s.Instance)
	body := "state=" + status.State + ":id=" + id
	if status.App != "" {
		body += ":app=" + status.App
	}
	if status.Kind != "" {
		body += ":kind=" + status.Kind
	}
	return "\x1b]7501;" + body + ":msg=" + base64.StdEncoding.EncodeToString([]byte(message)) + "\x1b\\", nil
}

func ClearProgramStatus(instance string) (string, error) {
	id, err := StatusID(instance)
	if err != nil {
		return "", err
	}
	return "\x1b]7501;state=clear:id=" + id + "\x1b\\", nil
}

// Report is one decoded Program Status Protocol record: a report written by
// EncodeProgramStatus or Bubble Tea's View.ProgramStatus, or a clear sequence.
// Snapshot is set when the message carries the booba-actions convention.
type Report struct {
	Status  ProgramStatus
	ID      string
	Clear   bool
	Message string // Complete decoded msg field, including any JSON suffix.
	// Snapshot holds the actions carried after MessageMarker, if any.
	Snapshot *Snapshot
}

// IsProgramStatusQuery reports whether sequence is the OSC 7501 capability
// query, terminated by either BEL or ST.
func IsProgramStatusQuery(sequence string) bool {
	payload, ok := programStatusPayload(sequence)
	return ok && payload == "?"
}

func programStatusPayload(sequence string) (string, bool) {
	const prefix = "\x1b]7501;"
	if !strings.HasPrefix(sequence, prefix) {
		return "", false
	}
	payload := sequence[len(prefix):]
	switch {
	case strings.HasSuffix(payload, "\x1b\\"):
		return payload[:len(payload)-2], true
	case strings.HasSuffix(payload, "\a"):
		return payload[:len(payload)-1], true
	}
	return "", false
}

// DecodeProgramStatus parses one OSC 7501 report. Hosts use it to interpret
// program output. Unknown fields are ignored; a msg without MessageMarker
// leaves Snapshot nil and becomes the summary. Invalid action JSON is an error
// so hosts can drop stale controls, as the browser does.
func DecodeProgramStatus(sequence string) (Report, error) {
	payload, ok := programStatusPayload(sequence)
	if !ok {
		return Report{}, fmt.Errorf("not an OSC 7501 sequence")
	}
	if payload == "?" {
		return Report{}, fmt.Errorf("OSC 7501 capability query is not a report")
	}
	var r Report
	for _, field := range strings.Split(payload, ":") {
		key, value, _ := strings.Cut(field, "=")
		switch key {
		case "state":
			r.Status.State = value
		case "id":
			r.ID = value
		case "app":
			r.Status.App = value
		case "kind":
			r.Status.Kind = value
		case "msg":
			decoded, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				return Report{}, fmt.Errorf("invalid OSC 7501 msg: %w", err)
			}
			if len(decoded) > MaxStatusMessageBytes {
				return Report{}, fmt.Errorf("status message exceeds %d decoded bytes (got %d)", MaxStatusMessageBytes, len(decoded))
			}
			r.Message = string(decoded)
		}
	}
	if r.Status.State == "" {
		return Report{}, fmt.Errorf("OSC 7501 report has no state")
	}
	r.Clear = r.Status.State == "clear"
	r.Status.Summary = r.Message
	if summary, data, ok := strings.Cut(r.Message, MessageMarker); ok {
		s, err := Decode([]byte(data))
		if err != nil {
			return Report{}, fmt.Errorf("invalid booba-actions payload: %w", err)
		}
		r.Status.Summary = summary
		r.Snapshot = &s
	}
	return r, nil
}
