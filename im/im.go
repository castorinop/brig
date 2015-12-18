package im

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/Sirupsen/logrus"
	xmpp "github.com/tsuibin/goxmpp2/xmpp"

	"golang.org/x/crypto/otr"
)

// Debug is a flag that enables some debug prints when set to true.
var Debug bool

var (
	ErrTimeout = fmt.Errorf("Timeout reached during OTR io")
)

func init() {
	Debug = false
	xmpp.Debug = false
}

// TODO: Prevent send to unavailable partner?
// TODO: Compare fingerprints. (store to file with key)

type Conversation struct {
	// Jid of your Conversation.
	Jid xmpp.JID

	// Client is a pointer to the client this cnv belongs to.
	Client *Client

	// recv provides all messages sent from this cnv.
	recv chan []byte

	// send can be used to send arbitary messages to this cnv.
	send chan []byte

	// Did we initiated the conversation to this cnv?
	initiated bool

	// This cnv completed the auth-game
	authorised bool

	// the underlying otr conversation
	conversation *otr.Conversation

	// A backlog of messages send before otr auth.
	backlog [][]byte

	// used in Read() to compensate against small read-buffers.
	readBuf *bytes.Buffer

	// This is set to a value > 0 if the conversation ended.
	cnvIsDead uint32
}

func newConversation(jid xmpp.JID, client *Client, privKey *otr.PrivateKey) *Conversation {
	sendChan := make(chan []byte)
	recvChan := make(chan []byte)

	go func() {
		for data := range sendChan {
			if err := client.send(jid, data); err != nil {
				log.Warningf("im-send: %v", err)
			}
		}
	}()

	return &Conversation{
		Jid:     jid,
		Client:  client,
		recv:    recvChan,
		send:    sendChan,
		backlog: make([][]byte, 0),
		readBuf: &bytes.Buffer{},
		conversation: &otr.Conversation{
			PrivateKey: privKey,
		},
	}
}

func (b *Conversation) Write(buf []byte) (int, error) {
	if b.Ended() {
		return 0, fmt.Errorf("Write: conversation ended.")
	}

	ticker := time.NewTicker(b.Client.Timeout)

	select {
	case <-ticker.C:
		return 0, ErrTimeout
	case b.send <- buf:
		return len(buf), nil
	}
}

func (b *Conversation) Read(buf []byte) (int, error) {
	msg, err := b.ReadMessage()
	if err != nil {
		return 0, err
	}

	b.readBuf.Write(msg)
	return b.readBuf.Read(buf)
}

// ReadMessage returns exactly one message.
func (b *Conversation) ReadMessage() ([]byte, error) {
	if b.Ended() {
		return nil, fmt.Errorf("Read: conversation ended.")
	}

	ticker := time.NewTicker(b.Client.Timeout)

	select {
	case <-ticker.C:
		return nil, ErrTimeout
	case msg, ok := <-b.recv:
		if ok {
			return msg, nil
		}

		return nil, fmt.Errorf("Read: conversation ended during read.")
	}
}

// NOTE: adieu() is called with c.Lock() hold.
func (b *Conversation) adieu() {
	// Make sure Write()/Read() does not block anymore.
	atomic.StoreUint32(&b.cnvIsDead, 1)

	b.authorised = false

	if b.conversation != nil {
		// End() returns some messages that can be used to revert the connection
		// back to a normal non-OTR connection. We just don't send those.
		b.conversation.End()
	}

	// Wakeup any Write/Read calls.
	close(b.send)
	close(b.recv)
}

func (b *Conversation) Ended() bool {
	return atomic.LoadUint32(&b.cnvIsDead) > 0
}

// Config can be passed to NewClient to configure how the details.
type Config struct {
	// Jid is the login user.
	Jid xmpp.JID

	// TLSConfig is used in building the login communication.
	TLSConfig tls.Config

	// Password is the XMPP login password.
	Password string

	// The place where the private otr key is stored.
	KeyPath string

	// The place where fingerprints are stored.
	KeyStorePath string

	// Timeout before Read or Write will error with ErrTimeout.
	Timeout time.Duration
}

// Client is an XMPP client with OTR support.
// Before establishing a connection, OTR will be triggered
// and the Socialist Millionaire Protocol is played through,
// using the minilock IDs of the participants.
type Client struct {
	sync.Mutex

	// Embedded client
	C *xmpp.Client

	// Path to a otr-key file. If empty, a new one will be generated.
	KeyPath string

	// Connection Status channel:
	Status chan xmpp.Status

	// Timeout before Write/Read will timeout on error.
	Timeout time.Duration

	// JID to each individual cnv.
	// Only active connections are stored here.
	buddies map[xmpp.JID]*Conversation

	// buddies that send initial messages to us are pushed to this chan.
	incomingBuddies chan *Conversation

	// Needed to compare previous fingerprints
	keys KeyStore

	// Lookup map for online status for Client.C.Roster
	online map[xmpp.JID]bool
}

func (c *Client) addPresence(ps *xmpp.Presence) {
	c.Lock()
	defer c.Unlock()

	log.Debugf("Partner presence `%v`: %v", ps.From, ps.Type != "unavailable")
	c.online[ps.From] = (ps.Type != "unavailable")
}

func (c *Client) isOnline(jid xmpp.JID) bool {
	c.Lock()
	defer c.Unlock()

	return c.online[jid]
}

// locked cnv lookup
func (c *Client) lookupConversation(jid xmpp.JID) (*Conversation, bool) {
	c.Lock()
	defer c.Unlock()

	cnv, ok := c.buddies[jid]
	return cnv, ok
}

func (c *Client) removeConversation(jid xmpp.JID) {
	c.Lock()
	defer c.Unlock()

	if cnv, ok := c.buddies[jid]; ok {
		cnv.adieu()
	}

	delete(c.buddies, jid)
}

// NewClient returns a ready client or nil on error.
func NewClient(config *Config) (*Client, error) {
	keyStore, err := NewFsKeyStore(config.KeyStorePath)
	if err != nil {
		return nil, err
	}

	c := &Client{
		KeyPath:         config.KeyPath,
		Timeout:         config.Timeout,
		buddies:         make(map[xmpp.JID]*Conversation),
		incomingBuddies: make(chan *Conversation),
		online:          make(map[xmpp.JID]bool),
		keys:            keyStore,
	}

	if config.Timeout <= 0 {
		c.Timeout = 20 * time.Second
	}

	xmppClient, err := xmpp.NewClient(
		&config.Jid, config.Password, config.TLSConfig,
		nil, xmpp.Presence{}, c.Status,
	)

	if err != nil {
		log.Fatalf("NewClient(%v): %v", config.Jid, err)
		return nil, err
	}

	c.C = xmppClient

	// Remember to update the status:
	go func() {
		for status := range c.Status {
			log.Debugf("connection status %d", status)
		}
	}()

	// Recv loop: Handle incoming messages, filter OTR.
	go func() {
		for stanza := range c.C.Recv {
			switch msg := stanza.(type) {
			case *xmpp.Message:
				response, err := c.recv(msg)
				if err != nil {
					log.Warningf("im-recv: %v", err)
				}

				if response != nil {
					if cnv, ok := c.lookupConversation(msg.From); ok {
						// Compensate for slow receivers:
						go func() { cnv.recv <- joinBodies(response) }()
					}
				}
			case *xmpp.Presence:
				if msg.Type == "unavailable" {
					if _, ok := c.lookupConversation(msg.From); ok {
						log.Infof("Removed otr conversation with %v", msg.From)
						c.removeConversation(msg.From)
					}
				}

				c.addPresence(msg)
			}
		}
	}()

	return c, nil
}

// IsOnline cheks if the partner is online.
// On startup, this might block until the first presence messages are available.
func (c *Client) IsOnline(jid xmpp.JID) bool {
	// TODO: Actually wait for first presence.
	return c.isOnline(jid)
}

// Talk opens a conversation with another peer.
func (c *Client) Talk(jid xmpp.JID) (*Conversation, error) {
	// Begin the OTR dance:
	if err := c.send(jid, nil); err != nil {
		return nil, err
	}

	if cnv, ok := c.lookupConversation(jid); ok {
		return cnv, nil
	}

	return nil, nil
}

// Listen waits for new buddies that talk to us.
func (c *Client) Listen() *Conversation {
	return <-c.incomingBuddies
}

func genPrivateKey(key *otr.PrivateKey, path string) error {
	key.Generate(rand.Reader)
	keyDump := key.Serialize(nil)

	if err := ioutil.WriteFile(path, keyDump, 0600); err != nil {
		return err
	}

	log.Infof("Key Generated: %x", key.Serialize(nil))
	return nil
}

// loadPrivateKey generates a valid otr.PrivateKey.
// This function should never fail in normal cases since it
// will attempt to generate a new key and write it to path as fallback.
func loadPrivateKey(path string) (*otr.PrivateKey, error) {
	key := &otr.PrivateKey{}

	// Try to load an existing one:
	if file, err := os.Open(path); err == nil {
		if data, err := ioutil.ReadAll(file); err == nil {
			if _, ok := key.Parse(data); ok {
				return key, nil
			}
		}
	}

	// Generate a new one as fallback or initial case:
	if err := genPrivateKey(key, path); err != nil {
		return nil, err
	}

	return key, nil
}

// NOTE: This function has to be called with c.Lock() held!
func (c *Client) lookupOrInitConversation(jid xmpp.JID) (*Conversation, bool, error) {
	_, ok := c.buddies[jid]

	if !ok {
		log.Infof("new otr-conversation: `%v`", string(jid))
		privKey, err := loadPrivateKey(c.KeyPath)
		if err != nil {
			log.Errorf("otr-key-gen failed: %v", err)
			return nil, false, err
		}

		c.buddies[jid] = newConversation(jid, c, privKey)
	}

	return c.buddies[jid], !ok, nil
}

func (c *Client) recv(msg *xmpp.Message) (*xmpp.Message, error) {
	plain, responses, isNoOtrMsg, err := c.recvRaw(joinBodies(msg), msg.From)
	if err != nil {
		return nil, err
	}

	// Turn every fragment into a separate xmpp message:
	for _, outMsg := range responses {
		if Debug {
			fmt.Printf("  SEND BACK: %v\n", truncate(string(outMsg), 30))
		}
		c.C.Send <- createMessage(c.C.Jid, msg.From, string(outMsg))
	}

	response := createMessage(msg.From, c.C.Jid, string(plain))
	if isNoOtrMsg {
		return response, nil
	}

	return nil, nil
}

func (c *Client) recvRaw(input []byte, from xmpp.JID) ([]byte, [][]byte, bool, error) {
	cnv, isNew, err := c.lookupOrInitConversation(from)
	if err != nil {
		return nil, nil, false, err
	}

	// We talk to this cnv the first time.
	if isNew {
		cnv.initiated = false
		c.incomingBuddies <- cnv

		// First received message should be the otr query.
		// Sometimes a xmpp server might deliver old messages dating from the
		// last conversation. In this case we just print a (probably harmless) warning.
		if !bytes.Contains(input, []byte(otr.QueryMessage)) {
			return nil, nil, false, fmtOtrErr("init", input, fmt.Errorf("First message was not OTT query"))
		}
	}

	// Pipe input through the conversation:
	otrCnv := cnv.conversation
	data, encrypted, stateChange, responses, err := otrCnv.Receive(input)
	if err != nil {
		return nil, nil, false, fmtOtrErr("recv", input, err)
	}

	if Debug {
		fmt.Printf("RECV: `%v` `%v` (encr: %v %v %v) (state-change: %v)\n",
			truncate(string(data), 30),
			truncate(string(input), 30),
			encrypted,
			otrCnv.IsEncrypted(),
			cnv.authorised,
			stateChange,
		)
	}

	auth := func(question string, answer []byte) error {
		authResp, err := otrCnv.Authenticate(question, answer)
		if err != nil {
			log.Warningf("im: Authentication error: %v", err)
			return err
		}

		responses = append(responses, authResp...)
		return nil
	}

	// Handle any otr conversation state change:
	switch stateChange {
	case otr.NewKeys: // We exchanged keys, channel is encrypted now.
		if cnv.initiated {
			if err := auth("weis nich?", []byte("eule")); err != nil {
				return nil, nil, false, err
			}
		}
	case otr.SMPSecretNeeded: // We received a question and have to answer.
		question := otrCnv.SMPQuestion()
		log.Debugf("[!] Answer a question '%s'", question)
		if err := auth(question, []byte("eule")); err != nil {
			return nil, nil, false, err
		}
	case otr.SMPComplete: // We or they completed the quest.
		log.Debugf("[!] Answer is correct")
		if cnv.initiated == false && cnv.authorised == false {
			if err := auth("wer weis nich?", []byte("eule")); err != nil {
				return nil, nil, false, err
			}
		}

		if cnv.initiated == true && cnv.authorised {
			for _, backlogMsg := range cnv.backlog {
				base64Texts, err := cnv.conversation.Send(backlogMsg)
				if err != nil {
					return nil, nil, false, fmtOtrErr("send", backlogMsg, err)
				}

				responses = append(responses, base64Texts...)
			}
			cnv.backlog = make([][]byte, 0)
		}

		cnv.authorised = true
	case otr.SMPFailed: // We or they failed.
		log.Debugf("[!] Answer is wrong")
		fallthrough
	case otr.ConversationEnded:
		cnv.adieu()
		delete(c.buddies, cnv.Jid)
	}

	return data, responses, stateChange == otr.NoChange && encrypted && len(data) > 0, nil
}

// Send sends `text` to participant `to`.
// A new otr session will be established if required.
// It is allowed that `text` to be nil. This might trigger the otr exchange,
// but does not send any real messages.
func (c *Client) send(to xmpp.JID, text []byte) error {
	c.Lock()
	defer c.Unlock()

	cnv, isNew, err := c.lookupOrInitConversation(to)
	if err != nil {
		return err
	}

	if isNew {
		cnv.initiated = true

		// Send the initial ?OTRv2? query:
		if err := c.sendRaw(to, []byte(otr.QueryMessage), cnv); err != nil {
			return fmt.Errorf("im: OTR Authentication failed: %v", err)
		}
	}

	if text == nil {
		return nil
	}

	if !cnv.authorised {
		cnv.backlog = append(cnv.backlog, text)
		return nil
	}

	return c.sendRaw(to, text, cnv)
}

func (c *Client) sendRaw(to xmpp.JID, text []byte, cnv *Conversation) error {
	base64Texts, err := cnv.conversation.Send(text)

	if Debug {
		fmt.Printf("SEND(%v|%v): %v => %v\n",
			cnv.conversation.IsEncrypted(), cnv.authorised,
			string(text), truncate(string(base64Texts[0]), 30),
		)
	}

	if err != nil {
		log.Warningf("im: send:", err)
		return err
	}

	for _, base64Text := range base64Texts {
		c.C.Send <- createMessage(c.C.Jid, to, string(base64Text))
	}

	return nil
}

// Close terminates all open connections.
func (c *Client) Close() {
	c.Lock()
	defer c.Unlock()

	for _, cnv := range c.buddies {
		cnv.adieu()
	}
	c.C.Close()
}