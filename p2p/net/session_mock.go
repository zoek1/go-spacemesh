package net

import (
	"github.com/spacemeshos/go-spacemesh/p2p/cryptoBox"
	"github.com/spacemeshos/go-spacemesh/p2p/cryptoSign"
)

// SessionMock is a wonderful fluffy teddybear
type SessionMock struct {
	id        cryptoSign.PublicKey
	decResult []byte
	decError  error
	encResult []byte
	encError  error

	pubkey cryptoBox.Key
	keyM   []byte
}

func (sm SessionMock) PeerPubkey() cryptoSign.PublicKey {
	panic("implement me")
}

func (sm SessionMock) SetSharedSecret(sharedSecret cryptoBox.Key) {
	panic("implement me")
}

func NewSessionMock(ID []byte) *SessionMock {
	publicKey, _ := cryptoSign.NewPublicKey(ID)
	return &SessionMock{id: publicKey}
}

// ID is this
func (sm SessionMock) ID() cryptoSign.PublicKey {
	return sm.id
}

// KeyM is this
func (sm SessionMock) KeyM() []byte {
	return sm.keyM
}

// Encrypt is this
func (sm SessionMock) SealMessage(message []byte) []byte {
	out := message
	if sm.encResult != nil {
		out = sm.encResult
	}
	return out
}

// Decrypt is this
func (sm SessionMock) OpenMessage(boxedMessage []byte) ([]byte, error) {
	out := boxedMessage
	if sm.decResult != nil {
		out = sm.decResult
	}
	return out, sm.decError
}

// SetDecrypt is this
func (sm *SessionMock) SetDecrypt(res []byte, err error) {
	sm.decResult = res
	sm.decError = err
}

var _ NetworkSession = (*SessionMock)(nil)
