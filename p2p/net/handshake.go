package net

import (
	"errors"
	"fmt"
	"github.com/gogo/protobuf/proto"
	"github.com/spacemeshos/go-spacemesh/p2p/config"
	"github.com/spacemeshos/go-spacemesh/p2p/cryptoBox"
	"github.com/spacemeshos/go-spacemesh/p2p/cryptoSign"
	"github.com/spacemeshos/go-spacemesh/p2p/pb"
	"golang.org/x/crypto/nacl/sign"
	"time"
)

// HandshakeReq specifies the handshake protocol request message identifier. pattern is [protocol][version][method-name].
const HandshakeReq = "/handshake/1.0/handshake-req/"

// HandshakeResp specifies the handshake protocol response message identifier.
const HandshakeResp = "/handshake/1.0/handshake-resp/"

func GenerateHandshakeRequestData(peerPubkey, pubkey cryptoSign.PublicKey, privkey cryptoSign.PrivateKey,
	networkID int8, port uint16) ([]byte, NetworkSession, cryptoBox.Key, error) {

	return generateHandshakeMessageAndSession(peerPubkey, pubkey, privkey, networkID, port, HandshakeReq)
}

func ProcessHandshakeRequest(message []byte, pubkey cryptoSign.PublicKey, privkey cryptoSign.PrivateKey,
	networkID int8, port uint16) ([]byte, NetworkSession, uint16, error) {

	peerPubkey, peerEphemeralPubkey, peerPort, err := parseAndValidateHandshakeMessage(message, nil, &networkID)
	if err != nil {
		return nil, nil, 0, err
	}

	response, session, ephemeralPrivkey, err :=
		generateHandshakeMessageAndSession(peerPubkey, pubkey, privkey, networkID, port, HandshakeResp)
	if err != nil {
		return nil, nil, 0, err
	}

	sharedSecret := cryptoBox.GenerateSharedSecret(ephemeralPrivkey, peerEphemeralPubkey)
	session.SetSharedSecret(sharedSecret)

	return response, session, peerPort, nil
}

func ProcessHandshakeResponse(message []byte, ephemeralPrivkey cryptoBox.Key, session NetworkSession) error {

	_, peerEphemeralPubkey, _, err := parseAndValidateHandshakeMessage(message, session.PeerPubkey(), nil)
	if err != nil {
		return err
	}

	sharedSecret := cryptoBox.GenerateSharedSecret(ephemeralPrivkey, peerEphemeralPubkey)
	session.SetSharedSecret(sharedSecret)

	return nil
}

func generateHandshakeMessageAndSession(peerPubkey, ownPubkey cryptoSign.PublicKey, ownPrivkey cryptoSign.PrivateKey,
	networkID int8, port uint16, protocol string) ([]byte, NetworkSession, cryptoBox.Key, error) {

	ephemeralPubkey, ephemeralPrivkey, err := cryptoBox.GenerateKeyPair()
	if err != nil {
		return nil, nil, nil, err
	}
	handshakeData := &pb.HandshakeData{
		NetworkID:     int32(networkID),
		Port:          uint32(port),
		ClientVersion: config.ClientVersion,
		Timestamp:     time.Now().Unix(),
		Protocol:      protocol,
		NodePubKey:    ownPubkey.Bytes(),
		PubKey:        ephemeralPubkey.Bytes(),
	}
	unsignedMessage, err := proto.Marshal(handshakeData)
	message := ownPrivkey.Sign(unsignedMessage)

	session := NewNetworkSession(peerPubkey)

	return message, session, ephemeralPrivkey, nil
}

func parseAndValidateHandshakeMessage(message []byte, peerPubkey cryptoSign.PublicKey, networkID *int8) (
	cryptoSign.PublicKey, cryptoBox.Key, uint16, error) {

	req := &pb.HandshakeData{}
	err := proto.Unmarshal(message[sign.Overhead:], req)
	if err != nil {
		return nil, nil, 0, err
	}

	if networkID != nil && *networkID != int8(req.NetworkID) {
		//TODO: drop and blacklist this sender
		return nil, nil, 0, fmt.Errorf("request net id (%d) is different than local net id (%d)", req.NetworkID, networkID)
	}

	if peerPubkey == nil {
		peerPubkey, err = cryptoSign.NewPublicKey(req.NodePubKey)
		if err != nil {
			return nil, nil, 0, err
		}
	}

	if _, ok := sign.Open(nil, message, peerPubkey.Raw()); !ok {
		return nil, nil, 0, errors.New("signature validation failed")
	}

	peerEphemeralPubkey, err := cryptoBox.NewKeyFromBytes(req.PubKey)
	if err != nil {
		return nil, nil, 0, err
	}

	return peerPubkey, peerEphemeralPubkey, uint16(req.Port), nil
}
