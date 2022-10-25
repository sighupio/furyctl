package analytics

type Event interface {
	Send(ch chan Event) error
	Properties() map[string]interface{}
	Name() string
}

func NewCommandEvent(name, errorMessage string, exitStatus int, details *DistroDetails) Event {
	props := map[string]interface{}{
		"exitStatus":   exitStatus,
		"errorMessage": errorMessage,
		"details":      details,
	}

	return CommandEvent{
		name:       name,
		properties: props,
	}
}

func (c CommandEvent) Send(ch chan Event) error {
	ch <- c

	return nil
}

func (c CommandEvent) Properties() map[string]interface{} {
	return c.properties
}

func (c CommandEvent) Name() string {
	return c.name
}

type CommandEvent struct {
	name string
	properties
}

type properties map[string]interface{}

type DistroDetails struct {
	Phase    string
	Provider string
	Version  string
}
