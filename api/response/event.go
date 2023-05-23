package response

// Events contain the latest events.
type Events struct {
	ChannelID int         `json:"channelid"`
	Events    []EventSpec `json:"events"`
}

type EventSpec struct {
	Data EventData `json:"data"`
}

// EventSpec is an individual event.
type EventData struct {
	Handler string      `json:"handler"`
	Object  EventObject `json:"object"`
}

// EventObject specifies the object of an event.
type EventObject struct {
	Attributes map[string]interface{}
	Reason     string `json:"reason"`
}

// Event is either an event or an error.
type Event struct {
	Event *EventData
	Error error
}
