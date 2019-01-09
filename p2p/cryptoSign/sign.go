package cryptoSign

import (
	cryptoRand "crypto/rand"
	"errors"
	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/nacl/sign"
	"strconv"
)

const (
	privkeySize = 64
	pubkeySize = 32
)

type PrivateKey interface {
	Sign(message []byte) []byte
	String() string
}

type privateKey struct {
	bytes *[privkeySize]byte
}

func (priv privateKey) String() string {
	return base58.Encode(priv.bytes[:])
}

func (priv privateKey) Sign(message []byte) []byte {
	signed := sign.Sign(nil, message, priv.bytes)
	return signed
}

func NewPrivateKey(bytes []byte) (PrivateKey, error) {
	if len(bytes) != privkeySize {
		return nil, errors.New("invalid private key size: " + strconv.Itoa(len(bytes)))
	}
	key := privateKey{&[privkeySize]byte{}}
	copy(key.bytes[:], bytes)
	return key, nil
}

// NewPrivateKeyFromString creates a new private key a base58 encoded string.
func NewPrivateKeyFromString(s string) (PrivateKey, error) {
	data := base58.Decode(s)
	return NewPrivateKey(data)
}

type PublicKey interface {
	Open(message []byte) (out []byte, ok bool)
	Bytes() []byte
	String() string
	Raw() *[pubkeySize]byte
}

type publicKey struct {
	bytes *[pubkeySize]byte
}

func (pub publicKey) Open(message []byte) (out []byte, ok bool) {
	return sign.Open(nil, message, pub.bytes)
}

func (pub publicKey) Bytes() []byte {
	return pub.bytes[:]
}

func (pub publicKey) String() string {
	return base58.Encode(pub.bytes[:])
}

func (pub publicKey) Raw() *[pubkeySize]byte {
	return pub.bytes
}

func NewPublicKey(bytes []byte) (PublicKey, error) {
	if len(bytes) != pubkeySize {
		return nil, errors.New("invalid public key size: " + strconv.Itoa(len(bytes)))
	}
	key := publicKey{&[pubkeySize]byte{}}
	copy(key.bytes[:], bytes)
	return key, nil
}

func NewPublicKeyFromString(s string) (PublicKey, error) {
	data := base58.Decode(s)
	return NewPublicKey(data)
}

func GenerateKeyPair() (PrivateKey, PublicKey, error) {
	public, private, err := sign.GenerateKey(cryptoRand.Reader)
	if err != nil {
		return nil, nil, err
	}

	return privateKey{private}, publicKey{public}, nil
}
