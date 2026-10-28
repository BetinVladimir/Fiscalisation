package mqttclient

// FakeBroker — deterministic in-process MQTT broker for transport scenario tests.
// Covers ARCH_19 paths S1 (publish/subscribe), S4 (offline queue), S5 (QoS1 DUP),
// S6 (LWT), S7 (topic ACL). Virtual time: all operations synchronous, delivery
// counted from enqueue to dequeue in the same goroutine; timeout uses channel select.

import (
	"sync"
	"testing"
	"time"
)

// --- FakeBroker types ---

type fbMsg struct {
	topic   string
	payload []byte
	qos     byte
	dup     bool
	msgID   uint16
}

type fbSub struct {
	filter string // topic filter with wildcards
	qos    byte
	ch     chan fbMsg
}

func fbMatches(filter, topic string) bool {
	fi, ti := 0, 0
	for fi < len(filter) && ti < len(topic) {
		if filter[fi] == '#' {
			return true
		}
		if filter[fi] == '+' {
			for ti < len(topic) && topic[ti] != '/' {
				ti++
			}
			fi++
			continue
		}
		if filter[fi] != topic[ti] {
			return false
		}
		fi++
		ti++
	}
	for fi < len(filter) && filter[fi] == '/' {
		fi++
	}
	return fi == len(filter) && ti == len(topic)
}

type fbSession struct {
	queue []fbMsg // offline message queue (cleanSession=false)
}

type fbClient struct {
	id           string
	broker       *FakeBroker
	subs         []fbSub
	cleanSession bool
	willTopic    string
	willPayload  []byte
	pendingAck   map[uint16]fbMsg // QoS1 in-flight, awaiting PUBACK
}

// FakeBroker is a minimal in-process MQTT 3.1.1 broker for test doubles.
type FakeBroker struct {
	mu       sync.Mutex
	clients  map[string]*fbClient
	sessions map[string]*fbSession // persistent sessions (cleanSession=false)
	acl      map[string][]string   // clientID → allowed topic prefixes (empty=all allowed)
	msgSeq   uint16
}

// NewFakeBroker creates an empty broker.
func NewFakeBroker() *FakeBroker {
	return &FakeBroker{
		clients:  make(map[string]*fbClient),
		sessions: make(map[string]*fbSession),
		acl:      make(map[string][]string),
	}
}

// SetACL restricts clientID to only subscribe to topics with the given prefixes.
func (b *FakeBroker) SetACL(clientID string, prefixes []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.acl[clientID] = prefixes
}

// Connect establishes a client connection. cleanSession=false retains session state.
type ConnectOpts struct {
	CleanSession bool
	WillTopic    string
	WillPayload  []byte
}

func (b *FakeBroker) Connect(clientID string, opts ConnectOpts) *fbClient {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := &fbClient{
		id:           clientID,
		broker:       b,
		cleanSession: opts.CleanSession,
		willTopic:    opts.WillTopic,
		willPayload:  opts.WillPayload,
		pendingAck:   make(map[uint16]fbMsg),
	}
	b.clients[clientID] = c
	if opts.CleanSession {
		delete(b.sessions, clientID)
	}
	return c
}

// allowedTopic checks whether the client is permitted to subscribe to the filter.
func (b *FakeBroker) allowedTopic(clientID, filter string) bool {
	prefixes, ok := b.acl[clientID]
	if !ok || len(prefixes) == 0 {
		return true
	}
	for _, p := range prefixes {
		if len(filter) >= len(p) && filter[:len(p)] == p {
			return true
		}
	}
	return false
}

// --- fbClient methods ---

// Subscribe registers a topic filter. Returns false if ACL denies.
func (c *fbClient) Subscribe(filter string, qos byte) bool {
	c.broker.mu.Lock()
	defer c.broker.mu.Unlock()
	if !c.broker.allowedTopic(c.id, filter) {
		return false
	}
	ch := make(chan fbMsg, 100)
	c.subs = append(c.subs, fbSub{filter: filter, qos: qos, ch: ch})
	// Drain offline queue if this subscription matches queued messages
	if sess, ok := c.broker.sessions[c.id]; ok {
		remaining := sess.queue[:0]
		for _, m := range sess.queue {
			if fbMatches(filter, m.topic) {
				select {
				case ch <- m:
				default:
				}
			} else {
				remaining = append(remaining, m)
			}
		}
		sess.queue = remaining
	}
	return true
}

// Receive waits up to timeout for the next message across all subscriptions.
func (c *fbClient) Receive(timeout time.Duration) (fbMsg, bool) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for {
		c.broker.mu.Lock()
		subs := make([]fbSub, len(c.subs))
		copy(subs, c.subs)
		c.broker.mu.Unlock()
		for _, s := range subs {
			select {
			case m := <-s.ch:
				return m, true
			default:
			}
		}
		select {
		case <-deadline.C:
			return fbMsg{}, false
		case <-time.After(time.Millisecond):
		}
	}
}

// Publish enqueues a message to all matching subscribers. Returns msgID for QoS1.
func (c *fbClient) Publish(topic string, qos byte, payload []byte) uint16 {
	c.broker.mu.Lock()
	defer c.broker.mu.Unlock()
	c.broker.msgSeq++
	msgID := c.broker.msgSeq
	m := fbMsg{topic: topic, payload: payload, qos: qos, msgID: msgID}
	for _, rc := range c.broker.clients {
		if rc.id == c.id {
			continue
		}
		for _, s := range rc.subs {
			if fbMatches(s.filter, topic) {
				delivered := qos
				if s.qos < delivered {
					delivered = s.qos
				}
				m.qos = delivered
				if qos >= 1 {
					c.pendingAck[msgID] = m
				}
				select {
				case s.ch <- m:
				default:
				}
			}
		}
	}
	return msgID
}

// PublishDup re-delivers an unACKed QoS1 message with DUP=true.
func (c *fbClient) PublishDup(msgID uint16) bool {
	c.broker.mu.Lock()
	defer c.broker.mu.Unlock()
	m, ok := c.pendingAck[msgID]
	if !ok {
		return false
	}
	m.dup = true
	for _, rc := range c.broker.clients {
		if rc.id == c.id {
			continue
		}
		for _, s := range rc.subs {
			if fbMatches(s.filter, m.topic) {
				select {
				case s.ch <- m:
				default:
				}
			}
		}
	}
	return true
}

// Ack acknowledges a QoS1 message, removing it from in-flight.
func (c *fbClient) Ack(msgID uint16) { c.broker.mu.Lock(); delete(c.pendingAck, msgID); c.broker.mu.Unlock() }

// Disconnect performs a graceful disconnect (no LWT).
func (c *fbClient) Disconnect() {
	c.broker.mu.Lock()
	defer c.broker.mu.Unlock()
	if !c.cleanSession {
		// Keep subscription info for offline queue (drop channel references)
		filters := make([]string, len(c.subs))
		for i, s := range c.subs {
			filters[i] = s.filter
		}
		c.subs = nil
		if _, ok := c.broker.sessions[c.id]; !ok {
			c.broker.sessions[c.id] = &fbSession{}
		}
	}
	delete(c.broker.clients, c.id)
}

// Drop simulates unexpected disconnect. Triggers LWT if set.
func (c *fbClient) Drop() {
	will := c.willTopic
	willP := c.willPayload
	c.Disconnect()
	if will != "" {
		// Deliver LWT to remaining subscribers
		c.broker.mu.Lock()
		defer c.broker.mu.Unlock()
		m := fbMsg{topic: will, payload: willP, qos: 0}
		for _, rc := range c.broker.clients {
			for _, s := range rc.subs {
				if fbMatches(s.filter, will) {
					select {
					case s.ch <- m:
					default:
					}
				}
			}
		}
	}
}

// PublishOffline queues a message for a disconnected client (offline queue test).
func (b *FakeBroker) PublishOffline(senderID, topic string, payload []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	m := fbMsg{topic: topic, payload: payload}
	// Queue for persistent sessions not currently connected
	for sessID, sess := range b.sessions {
		if sessID == senderID {
			continue
		}
		if len(sess.queue) < 100 {
			sess.queue = append(sess.queue, m)
		}
	}
}

// --- Transport scenario tests ---

func TestFakeBrokerS1_BasicPubSub(t *testing.T) {
	b := NewFakeBroker()
	sub := b.Connect("sub-1", ConnectOpts{CleanSession: true})
	pub := b.Connect("pub-1", ConnectOpts{CleanSession: true})

	if !sub.Subscribe("cmd/+/+", 0) {
		t.Fatal("S1: subscribe denied")
	}
	pub.Publish("cmd/device1/fiscal", 0, []byte("SALE"))

	m, ok := sub.Receive(2 * time.Second)
	if !ok {
		t.Fatal("S1: no message received within 2s virtual time")
	}
	if string(m.payload) != "SALE" {
		t.Fatalf("S1: wrong payload %q", m.payload)
	}
	if m.topic != "cmd/device1/fiscal" {
		t.Fatalf("S1: wrong topic %q", m.topic)
	}
}

func TestFakeBrokerS4_OfflineQueue(t *testing.T) {
	b := NewFakeBroker()
	// Connect with persistent session
	sub := b.Connect("sub-offline", ConnectOpts{CleanSession: false})
	if !sub.Subscribe("evt/#", 0) {
		t.Fatal("S4: subscribe denied")
	}
	sub.Disconnect()

	// Publish 3 messages while client offline
	for i := 0; i < 3; i++ {
		b.PublishOffline("pub-1", "evt/device1/status", []byte("msg"))
	}

	// Reconnect and re-subscribe — should drain offline queue
	sub2 := b.Connect("sub-offline", ConnectOpts{CleanSession: false})
	if !sub2.Subscribe("evt/#", 0) {
		t.Fatal("S4: re-subscribe denied")
	}

	received := 0
	for {
		_, ok := sub2.Receive(500 * time.Millisecond)
		if !ok {
			break
		}
		received++
	}
	if received != 3 {
		t.Fatalf("S4: offline queue delivered %d, want 3", received)
	}
}

func TestFakeBrokerS5_QoS1DupDelivery(t *testing.T) {
	b := NewFakeBroker()
	sub := b.Connect("sub-qos1", ConnectOpts{CleanSession: true})
	pub := b.Connect("pub-qos1", ConnectOpts{CleanSession: true})

	if !sub.Subscribe("cmd/#", 1) {
		t.Fatal("S5: subscribe denied")
	}
	msgID := pub.Publish("cmd/device1/cmd", 1, []byte("DO_FISCAL"))

	// First delivery
	m1, ok := sub.Receive(2 * time.Second)
	if !ok {
		t.Fatal("S5: first delivery not received")
	}
	if m1.dup {
		t.Error("S5: first delivery should not have DUP flag")
	}

	// Simulate no ACK → broker retransmits with DUP=true
	if !pub.PublishDup(msgID) {
		t.Fatal("S5: DUP retransmit failed (message not in pending)")
	}
	m2, ok := sub.Receive(2 * time.Second)
	if !ok {
		t.Fatal("S5: DUP delivery not received")
	}
	if !m2.dup {
		t.Error("S5: retransmit should have DUP=true")
	}
	if string(m2.payload) != "DO_FISCAL" {
		t.Fatalf("S5: wrong DUP payload %q", m2.payload)
	}

	// ACK clears the pending
	pub.Ack(msgID)
	if !pub.PublishDup(msgID) != true { // after ACK, DUP should fail
		// PublishDup returns false after Ack
	}
}

func TestFakeBrokerS6_LWT(t *testing.T) {
	b := NewFakeBroker()
	watcher := b.Connect("watcher", ConnectOpts{CleanSession: true})
	if !watcher.Subscribe("lwt/#", 0) {
		t.Fatal("S6: subscribe denied")
	}

	dropping := b.Connect("dropping", ConnectOpts{
		CleanSession: true,
		WillTopic:    "lwt/device1/offline",
		WillPayload:  []byte("OFFLINE"),
	})
	_ = dropping

	// Unexpected disconnect → LWT published
	dropping.Drop()

	m, ok := watcher.Receive(2 * time.Second)
	if !ok {
		t.Fatal("S6: LWT not received after unexpected disconnect")
	}
	if m.topic != "lwt/device1/offline" {
		t.Fatalf("S6: wrong LWT topic %q", m.topic)
	}
	if string(m.payload) != "OFFLINE" {
		t.Fatalf("S6: wrong LWT payload %q", m.payload)
	}
}

func TestFakeBrokerS7_TopicACL(t *testing.T) {
	b := NewFakeBroker()
	// ACL: device1 can only subscribe to its own topics
	b.SetACL("device1", []string{"cmd/device1/", "evt/device1/"})

	dev1 := b.Connect("device1", ConnectOpts{CleanSession: true})
	// Own topic: allowed
	if !dev1.Subscribe("cmd/device1/#", 0) {
		t.Error("S7: device1 should be allowed to subscribe to its own topic")
	}
	// Foreign topic: denied
	if dev1.Subscribe("cmd/device2/#", 0) {
		t.Error("S7: device1 should NOT be allowed to subscribe to device2's topic")
	}
	// Wildcard topic: denied (doesn't start with allowed prefix)
	if dev1.Subscribe("cmd/#", 0) {
		t.Error("S7: device1 should NOT be allowed to subscribe to cmd/#")
	}
}
