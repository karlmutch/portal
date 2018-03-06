package portal

import (
	"github.com/SentimensRG/ctx"
	uuid "github.com/satori/go.uuid"
)

// ID uniquely identifies a Portal
type ID uuid.UUID

func (id ID) String() string { return uuid.UUID(id).String() }

// NewID generates a unique ID
func NewID() ID { return ID(uuid.Must(uuid.NewV4())) }

// BindConnectCloser can Bind and Connect to an address
type BindConnectCloser interface {
	Connect(string) error
	Bind(string) error
	Close()
}

// Recver can receive values
type Recver interface {
	Recv() interface{}
	TryRecv() (interface{}, error)
}

// Sender can send values
type Sender interface {
	Send(interface{})
	TrySend(interface{}) error
}

// ReadOnly is the portal equivalent of <-chan
type ReadOnly interface {
	BindConnectCloser
	Recver
}

// WriteOnly is the portal equivalent of chan<-
type WriteOnly interface {
	BindConnectCloser
	Sender
}

// Portal is the main access handle applications use to access the protocol
// system.  It is an abstraction of an application's "connection" to a
// messaging topology.  Applications can have more than one Socket open
// at a time.
type Portal interface {
	ID() ID
	BindConnectCloser
	Sender
	Recver
}

// Endpoint is used by the Protocol implementation to access the underlying
// channel transport
type Endpoint interface {
	ctx.Doner
	ID() ID
	Close()
	SendChannel() <-chan *Message
	RecvChannel() chan<- *Message
	Signature() ProtocolSignature
}

// ProtocolSignature defines which protocols can talk to each other
type ProtocolSignature interface {
	// ProtocolNumber returns a 16-bit value for the protocol number,
	// as assigned by the SP governing body. (IANA?)
	Number() uint16

	// PeerNumber() returns a 16-bit number for our peer protocol.
	PeerNumber() uint16
}

// Protocol implements messaging topography elements.  Each protocol will
// implement Protocol and each half of a protocol pair (e.g. REQ/REP) will
// implement Protocol.
type Protocol interface {
	ProtocolSignature

	// Init is called by the core to allow the protocol to perform
	// any initialization steps it needs.  It should save the handle
	// for future use, as well.
	Init(ProtocolPortal)

	// AddEndpoint is called when a new Endpoint is added to the socket.
	// Typically this is as a result of connect or accept completing.
	AddEndpoint(Endpoint)

	// RemoveEndpoint is called when an Endpoint is removed from the socket.
	// Typically this indicates a disconnected or closed connection.
	RemoveEndpoint(Endpoint)
}

// ProtocolPortal is the "handle" given to protocols to interface with the
// socket.  The Protocol implementation should not access any sockets or pipes
// except by using functions made available on the ProtocolSocket.  Note
// that all functions listed here are non-blocking.
type ProtocolPortal interface {
	ID() ID

	// SendChannel represents the channel used to send messages.  The
	// application injects messages to it, and the protocol consumes
	// messages from it.  The channel may be closed when the core needs to
	// create a new channel, typically after an option is set that requires
	// the channel to be reconfigured.  (OptionWriteQLen) When the protocol
	// implementation notices this, it should call this function again to obtain
	// the value of the new channel.
	SendChannel() <-chan *Message

	// RecvChannel is the channel used to receive messages.  The protocol
	// should inject messages to it, and the application will consume them
	// later.
	RecvChannel() chan<- *Message

	// The protocol can wait on this channel to close.  When it is closed,
	// it indicates that the application has closed the upper read socket,
	// and the protocol should stop any further read operations on this
	// instance.
	CloseChannel() <-chan struct{}
}

// ProtocolSendHook allows protocol implementers to extend existing protocols
type ProtocolSendHook interface {
	// SendHook is called when the application calls Send.
	// If false is returned, the message will be silently dropped.
	// Note that the message may be dropped for other reasons,
	// such as if backpressure is applied.
	SendHook(*Message) bool
}

// ProtocolRecvHook allows protocol implementers to extend existing protocols
type ProtocolRecvHook interface {
	// RecvHook is called just before the message is handed to the
	// application.  The message may be modified.  If false is returned,
	// then the message is dropped.
	RecvHook(*Message) bool
}
