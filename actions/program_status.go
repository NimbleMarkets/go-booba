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
		return "", fmt.Errorf("action status message exceeds %d decoded bytes", MaxStatusMessageBytes)
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
