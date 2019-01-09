package net

import (
	"fmt"
	"github.com/spacemeshos/go-spacemesh/p2p/config"
	"github.com/spacemeshos/go-spacemesh/p2p/cryptoBox"
	"github.com/spacemeshos/go-spacemesh/p2p/node"
	"github.com/stretchr/testify/assert"
	"net"
	"strconv"
	"testing"
)

func TestGenerateHandshakeRequestData(t *testing.T) {
	port := 0
	address := fmt.Sprintf("0.0.0.0:%d", port)
	localNode, err := node.NewLocalNode(config.ConfigValues, address, false)
	assert.NoError(t, err, "should be able to create localnode")
	remoteNode, err := node.NewLocalNode(config.ConfigValues, address, false)
	assert.NoError(t, err, "should be able to create localnode")
	con := NewConnectionMock(remoteNode.PublicKey())
	remoteNet, _ := NewNet(config.ConfigValues, remoteNode)

	//outchan := remoteNet.SubscribeOnNewRemoteConnections()
	_, _, _, err = GenerateHandshakeRequestData(con.RemotePublicKey(), localNode.PublicKey(), localNode.PrivateKey(),
		remoteNet.NetworkID(), getPort(t, remoteNode.Node))
	assert.NoError(t, err, "Sanity failed")

}

func getPort(t *testing.T, remote node.Node) uint16 {
	_, port, err := net.SplitHostPort(remote.Address())
	assert.NoError(t, err)
	portint, err := strconv.Atoi(port)
	assert.NoError(t, err)
	return uint16(portint)
}

func generateRequestData(t *testing.T) ([]byte, NetworkSession, node.LocalNode, int8, cryptoBox.Key) {

	initiatorNode, _ := node.GenerateTestNode(t)
	responderNode, _ := node.GenerateTestNode(t)
	netId := int8(1)
	handshakeRequest, initiatorSession, initiatorEphemeralPrivkey, err := GenerateHandshakeRequestData(
		responderNode.PublicKey(),
		initiatorNode.PublicKey(), initiatorNode.PrivateKey(),
		netId, getPort(t, responderNode.Node))
	assert.NoError(t, err, "Failed to generate request")
	return handshakeRequest, initiatorSession, *responderNode, netId, initiatorEphemeralPrivkey
}

func TestProcessHandshakeRequest(t *testing.T) {
	//Sanity
	handshakeRequest, _, responderNode, netId, _ := generateRequestData(t)
	port := getPort(t, responderNode.Node)
	//processing request in responderNode from initiatorNode
	_, _, _, err :=
		ProcessHandshakeRequest(handshakeRequest, responderNode.PublicKey(), responderNode.PrivateKey(), netId, port)
	assert.NoError(t, err, "Sanity processing request failed", err)

	_, _, _, err =
		ProcessHandshakeRequest(handshakeRequest, responderNode.PublicKey(), responderNode.PrivateKey(), netId, port)
	assert.NoError(t, err, "Data modified during test")

	_, _, _, err =
		ProcessHandshakeRequest(handshakeRequest, responderNode.PublicKey(), responderNode.PrivateKey(), netId+1, port)
	assert.Error(t, err, "Didn't receive error on network id incompatible with request")

}

func TestProcessHandshakeResponse(t *testing.T) {
	//Sanity
	handshakeRequest, initiatorSession, responderNode, netId, initiatorEphemeralPrivkey := generateRequestData(t)
	port := getPort(t, responderNode.Node)
	handshakeResponse, responderSession, _, err := ProcessHandshakeRequest(handshakeRequest, responderNode.PublicKey(),
		responderNode.PrivateKey(), netId, port)
	assert.NoError(t, err, "Sanity creating request failed")

	err = ProcessHandshakeResponse(handshakeResponse, initiatorEphemeralPrivkey, initiatorSession)
	assert.NoError(t, err, "Sanity processing response failed")

	err = ProcessHandshakeResponse(handshakeResponse, initiatorEphemeralPrivkey, responderSession)
	assert.Error(t, err, "remote key signing verification of response failed")

	err = ProcessHandshakeResponse(handshakeResponse, initiatorEphemeralPrivkey, initiatorSession)
	assert.NoError(t, err, "Sanity processing response failed")
}
