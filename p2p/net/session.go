package net

import (
	"github.com/spacemeshos/go-spacemesh/p2p/cryptoBox"
	"github.com/spacemeshos/go-spacemesh/p2p/cryptoSign"
)

// NetworkSession is an authenticated network session between 2 peers.
// Sessions may be used between 'connections' until they expire.
// Session provides the encryptor/decryptor for all messages exchanged between 2 peers.
// enc/dec is using an ephemeral sym key exchanged securely between the peers via the handshake protocol
// The handshake protocol goal is to create an authenticated network session.
type NetworkSession interface {
	ID() cryptoSign.PublicKey // Unique session id, currently the peer pubkey TODO: remove
	PeerPubkey() cryptoSign.PublicKey

	SetSharedSecret(sharedSecret cryptoBox.Key)

	OpenMessage(boxedMessage []byte) ([]byte, error) // decrypt data using session dec key
	SealMessage(message []byte) []byte               // encrypt data using session enc key
}

var _ NetworkSession = &NetworkSessionImpl{}
var _ NetworkSession = &SessionMock{}

// TODO: add support for idle session expiration

// NetworkSessionImpl implements NetworkSession.
type NetworkSessionImpl struct {
	id           cryptoSign.PublicKey
	sharedSecret cryptoBox.Key
	peerPubkey   cryptoSign.PublicKey
}

func (n *NetworkSessionImpl) PeerPubkey() cryptoSign.PublicKey {
	return n.peerPubkey
}

func (n *NetworkSessionImpl) SetSharedSecret(sharedSecret cryptoBox.Key) {
	n.sharedSecret = sharedSecret
}

// String returns the session's identifier string.
func (n *NetworkSessionImpl) String() string {
	return n.peerPubkey.String()
}

// ID returns the session's unique id
func (n *NetworkSessionImpl) ID() cryptoSign.PublicKey {
	return n.peerPubkey
}

// Encrypt encrypts in binary data with the session's sym enc key.
func (n *NetworkSessionImpl) SealMessage(message []byte) []byte {
	// TODO: verify that a shared secret has been set
	return n.sharedSecret.Seal(message)
}

// Decrypt decrypts in binary data that was encrypted with the session's sym enc key.
func (n *NetworkSessionImpl) OpenMessage(boxedMessage []byte) (message []byte, err error) {
	// TODO: verify that a shared secret has been set
	return n.sharedSecret.Open(boxedMessage)
}

// NewNetworkSession creates a new network session based on provided data
func NewNetworkSession(peerPubkey cryptoSign.PublicKey) *NetworkSessionImpl {
	n := &NetworkSessionImpl{
		peerPubkey: peerPubkey,
	}

	return n
}
