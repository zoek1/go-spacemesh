package cryptoBox

import (
	crypto_rand "crypto/rand"
	"errors"
	"fmt"
	"github.com/btcsuite/btcutil/base58"
	"github.com/spacemeshos/go-spacemesh/log"
	"golang.org/x/crypto/nacl/box"
	"io"
)

const (
	keySize = 32
	nonceSize = 24
)

type Key interface {
	Bytes() []byte
	Raw() *[keySize]byte
	String() string
	Pretty() string

	Seal(message []byte) (out []byte)
	Open(encryptedMessage []byte) (out []byte, err error)
}

type key struct {
	bytes *[keySize]byte
}

func (k key) Raw() *[keySize]byte {
	return k.bytes
}

func NewKeyFromString(s string) (Key, error) {
	bytes := base58.Decode(s)
	if len(bytes) == 0 {
		return nil, errors.New("unable to decode key")
	}
	return NewKeyFromBytes(bytes)
}

func NewKeyFromBytes(bytes []byte) (Key, error) {
	if len(bytes) != keySize {
		return nil, errors.New("invalid key size")
	}
	k := key{&[keySize]byte{}}
	copy(k.bytes[:], bytes)
	return k, nil
}

func (k key) Bytes() []byte {
	return k.bytes[:]
}

func (k key) String() string {
	return base58.Encode(k.bytes[:])
}

func (k key) Pretty() string {
	pstr := k.String()
	maxRunes := 6
	if len(pstr) < maxRunes {
		maxRunes = len(pstr)
	}
	return fmt.Sprintf("<Key %s>", pstr[:maxRunes])
}

func getRandomNonce() (nonce *[nonceSize]byte) {
	if _, err := io.ReadFull(crypto_rand.Reader, nonce[:]); err != nil {
		panic(err)
	}
	return
}

func (k key) Seal(message []byte) (out []byte) {
	nonce := getRandomNonce() // TODO: replace with counter to prevent replays
	return box.SealAfterPrecomputation(nonce[:], message, nonce, k.bytes)
}

func (k key) Open(encryptedMessage []byte) (out []byte, err error) {
	var nonce *[nonceSize]byte
	copy(nonce[:], encryptedMessage[:nonceSize])
	message, ok := box.OpenAfterPrecomputation(nil, encryptedMessage[nonceSize:], nonce, k.bytes)
	if !ok {
		return nil, errors.New("opening boxed message failed")
	}
	return message, nil
}

func GenerateKeyPair() (Key, Key, error) {
	public, private, err := box.GenerateKey(crypto_rand.Reader)
	if err != nil {
		log.Error("failed to generate key pair\n")
		return nil, nil, err
	}

	return key{private}, key{public}, nil
}

func GenerateSharedSecret(ephemeralPrivkey, peerEphemeralPubkey Key) Key {
	sharedSecret := key{&[keySize]byte{}}
	box.Precompute(sharedSecret.bytes, peerEphemeralPubkey.Raw(), ephemeralPrivkey.Raw())
	return sharedSecret
}
