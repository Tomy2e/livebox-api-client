package livebox

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Tomy2e/livebox-api-client/api/request"
	"github.com/Tomy2e/livebox-api-client/api/response"
	"github.com/Tomy2e/livebox-api-client/internal/client"
)

type events struct {
	ChannelID int      `json:"channelid"`
	Events    []string `json:"events"`
}

// Events watches the specified events until context is canceled.
func (c *Client) Events(ctx context.Context, events []string) <-chan *response.Event {
	el := &eventListener{
		client:  c,
		events:  events,
		channel: make(chan *response.Event, 128),
	}
	go el.Run(ctx)

	c.startEventSessionKeepAlive()

	return el.channel
}

func (c *Client) requestEvent(ctx context.Context, req *events) (*response.Events, error) {
	for {
		var events response.Events

		if err := c.client.Request(ctx, client.ContentTypeEvent, req, &events); err != nil {
			if response.IsChannelDoesNotExistError(err) || response.IsFunctionExecutionFailedError(err) {
				req.ChannelID = 0
				continue
			}

			return nil, err
		}

		return &events, nil
	}
}

// startEventSessionKeepAlive starts a goroutine that continuously sends a normal
// livebox request to reauthenticate automatically if needed.
func (c *Client) startEventSessionKeepAlive() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventsCtr++
	if c.eventsCtr == 1 {
		c.log.Debug("Starting event session keepalive goroutine")
		ch := make(chan struct{})
		c.eventsStopCh = ch

		go func() {
			defer close(ch)

			for {
				out := json.RawMessage{}
				if err := c.client.Request(
					context.TODO(),
					client.ContentTypeWS,
					request.New("IoTService", "getStatus", nil),
					&out,
				); err != nil {
					c.log.Debug("Failed to send session keepalive request", slog.Any("error", err))
				}

				select {
				case <-ch:
					c.log.Debug("Stopped event session keepalive goroutine")
					return
				case <-time.After(30 * time.Second):
				}
			}
		}()
	}
}

func (c *Client) stopEventSessionKeepAlive() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.eventsCtr--

	if c.eventsCtr == 0 {
		c.eventsStopCh <- struct{}{}
	}
}

type eventListener struct {
	client    *Client
	channelID int
	events    []string
	channel   chan *response.Event
}

func (el *eventListener) Run(ctx context.Context) {
	defer el.client.stopEventSessionKeepAlive()
	defer close(el.channel)

	for {
		events, err := el.client.requestEvent(ctx, &events{ChannelID: el.channelID, Events: el.events})
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}

			el.channel <- &response.Event{Error: err}
			el.channelID = 0
			time.Sleep(1 * time.Second) // TODO: retry with backoff?
			continue
		}

		el.channelID = events.ChannelID

		for _, event := range events.Events {
			event := event
			select {
			case <-ctx.Done():
				return
			case el.channel <- &response.Event{Event: &event.Data}:
			}
		}
	}
}
