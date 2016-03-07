package redisbroker

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/PuerkitoBio/exp/juggler/broker"
	"github.com/PuerkitoBio/exp/juggler/msg"
	"github.com/garyburd/redigo/redis"
)

// Pool defines the methods required for a redis pool that provides
// a method to get a connection and to release the pool's resources.
type Pool interface {
	// Get returns a redis connection.
	Get() redis.Conn

	// Close releases the resources used by the pool.
	Close() error
}

// Broker is a broker that provides the methods to
// interact with Redis using the juggler protocol.
type Broker struct {
	// Pool is the redis pool to use to get connections.
	Pool Pool

	// BlockingTimeout is the time to wait for a value on calls to
	// BRPOP.
	BlockingTimeout time.Duration

	// LogFunc is the logging function to use. If nil, log.Printf
	// is used. It can be set to juggler.DiscardLog to disable logging.
	LogFunc func(string, ...interface{})

	// CallCap is the capacity of the CALL queue. If it is exceeded,
	// Broker.Call calls fail with an error.
	CallCap int

	// ResultCap is the capacity of the RES queue. If it is exceeded,
	// Broker.Result calls fail with an error.
	ResultCap int
}

const (
	// if no Broker.BlockingTimeout is provided.
	defaultBlockingTimeout = 5 * time.Second

	callScript = `
		redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[1])
		local res = redis.call("LPUSH", KEYS[2], ARGV[2])
		if res > ARGV[3] and ARGV[3] > 0 then
			redis.call("LTRIM", KEYS[2], 1, ARGV[3] + 1)
			return redis.error_reply("list capacity exceeded")
		end
		return res
	`
	callKey            = "juggler:calls:{%s}"            // 1: URI
	callTimeoutKey     = "juggler:calls:timeout:{%s}:%s" // 1: URI, 2: mUUID
	defaultCallTimeout = time.Minute

	// RES: callee stores the result of the call in resKey (LPUSH) and
	// sets resTimeoutKey with an expiration of callTimeoutKey PTTL minus
	// the time of the call invocation.
	//
	// Caller BRPOPs on resKey. On a new payload, it checks if resTimeoutKey
	// is still valid. If it is, it sends the result on the connection,
	// otherwise it drops it. resTimeoutKey is deleted.
	resKey        = "juggler:results:{%s}"            // 1: cUUID
	resTimeoutKey = "juggler:results:timeout:{%s}:%s" // 1: cUUID, 2: mUUID
)

// Call registers a call request in the broker.
func (b *Broker) Call(cp *msg.CallPayload, timeout time.Duration) error {
	p, err := json.Marshal(cp)
	if err != nil {
		return err
	}

	rc := b.Pool.Get()
	defer rc.Close()

	to := int(timeout / time.Millisecond)
	if to == 0 {
		to = int(defaultCallTimeout / time.Millisecond)
	}

	_, err = rc.Do("EVAL",
		callScript,
		2, // the number of keys
		fmt.Sprintf(callTimeoutKey, cp.URI, cp.MsgUUID), // key[1] : the SET key with expiration
		fmt.Sprintf(callKey, cp.URI),                    // key[2] : the LIST key
		to,        // argv[1] : the timeout in milliseconds
		p,         // argv[2] : the call payload
		b.CallCap, // argv[3] : the LIST capacity
	)
	return err
}

// Result registers a call result in the broker.
func (b *Broker) Result(rp *msg.ResPayload, timeout time.Duration) error {
	// TODO : implement...
	return nil
}

// Publish publishes an event to a channel.
func (b *Broker) Publish(channel string, pp *msg.PubPayload) error {
	p, err := json.Marshal(pp)
	if err != nil {
		return err
	}

	rc := b.Pool.Get()
	defer rc.Close()

	_, err = rc.Do("PUBLISH", channel, p)
	return err
}

// PubSub returns a pub-sub connection that can be used to subscribe and
// unsubscribe to channels, and to process incoming events.
func (b *Broker) PubSub() (broker.PubSubConn, error) {
	rc := b.Pool.Get()
	return newPubSubConn(rc, b.LogFunc), nil
}

// Calls returns a calls connection that can be used to process the call
// requests for the specified URIs.
func (b *Broker) Calls(uris ...string) (broker.CallsConn, error) {
	rc := b.Pool.Get()
	return newCallsConn(rc, b.LogFunc, uris...), nil
}

func logf(fn func(string, ...interface{}), f string, args ...interface{}) {
	if fn != nil {
		fn(f, args...)
	} else {
		log.Printf(f, args...)
	}
}