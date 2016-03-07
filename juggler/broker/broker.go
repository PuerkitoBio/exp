package broker

import (
	"time"

	"github.com/PuerkitoBio/exp/juggler/msg"
	"github.com/pborman/uuid"
)

type CallerBroker interface {
	Results(uuid.UUID) (ResultsConn, error)
	Call(cp *msg.CallPayload, timeout time.Duration) error
}

// CalleeBroker defines the methods for a broker in the callee role.
type CalleeBroker interface {
	// Calls returns a CallsConn that can be used to process call requests
	// for the specified URIs.
	Calls(uris ...string) (CallsConn, error)

	// Result registers a call result in the broker.
	Result(rp *msg.ResPayload, timeout time.Duration) error
}

type PubSubBroker interface {
	PubSub() (PubSubConn, error)
	Publish(channel string, pp *msg.PubPayload) error
}

type ResultsConn interface {
	Results() <-chan *msg.ResPayload
	ResultsErr() error

	// Close closes the connection.
	Close() error
}

// CallsConn defines the methods to list the call requests using this
// connection.
type CallsConn interface {
	// Calls returns a stream of call requests for the URI used to
	// create the CallConn. The returned channel is closed when the
	// connection is closed, or when an error occurs. Callers can call
	// CallsErr to check the error that caused the channel to be closed.
	//
	// Only the first call to Calls starts the goroutine that checks
	// for requests. Subsequent calls return the same channel, so that many
	// consumers can process calls.
	Calls() <-chan *msg.CallPayload

	// CallsErr returns the error that caused the channel returned from
	// Calls to be closed. Is only non-nil once the channel is closed.
	CallsErr() error

	// Close closes the connection.
	Close() error
}

// PubSubConn defines the methods to manage subscriptions to events
// for a connection.
type PubSubConn interface {
	// Subscribe subscribes the connection to channel, which is treated
	// as a pattern if pattern is true.
	Subscribe(channel string, pattern bool) error

	// Unsubscribe unsubscribes the connection from the channel, which
	// is treated as a pattern if pattern is true.
	Unsubscribe(channel string, pattern bool) error

	// Events returns a stream of event payloads from events published
	// on channels that the connection is subscribed to.
	// The returned channel is closed when the connection is closed,
	// or when an error is received. Callers can call EventsErr on the
	// PubSubConn to check the error that caused the channel to be closed.
	//
	// Only the first call to Events starts the goroutine that listens to
	// events. Subsequent calls return the same channel, so that many
	// consumers can process events.
	Events() <-chan *msg.EvntPayload

	// EventsErr returns the error that caused the channel returned from
	// Events to be closed. Is only non-nil once the channel is closed.
	EventsErr() error

	// Close closes the connection.
	Close() error
}