package hare

import (
	"github.com/spacemeshos/go-spacemesh/log"
	"hash/fnv"
	"math"
	"sync"
)

type Role byte

const (
	Passive = Role(0)
	Active  = Role(1)
	Leader  = Role(2)
)

type Stringer interface {
	String() string
}

type Rolacle interface {
	Register(id string)
	Unregister(id string)
	Eligible(instanceID *InstanceId, K int, committeeSize int, pubKey string, proof []byte) bool
}

type hasherU32 struct {
}

func newHasherU32() *hasherU32 {
	h := new(hasherU32)

	return h
}

func (h *hasherU32) Hash(data []byte) uint32 {
	fnv := fnv.New32()
	fnv.Write(data)
	return fnv.Sum32()
}

func (h *hasherU32) MaxValue() uint32 {
	return math.MaxUint32
}

type MockHashOracle struct {
	clients map[string]struct{}
	mutex   sync.RWMutex
	hasher  *hasherU32
}

// N is the expected comity size
func NewMockHashOracle(expectedSize int) *MockHashOracle {
	mock := new(MockHashOracle)
	mock.clients = make(map[string]struct{}, expectedSize)
	mock.hasher = newHasherU32()

	return mock
}

func (mock *MockHashOracle) Register(client string) {
	mock.mutex.Lock()

	if _, exist := mock.clients[client]; exist {
		mock.mutex.Unlock()
		return
	}

	mock.clients[client] = struct{}{}
	mock.mutex.Unlock()
}

func (mock *MockHashOracle) Unregister(client string) {
	mock.mutex.Lock()
	delete(mock.clients, client)
	mock.mutex.Unlock()
}

// Calculates the threshold for the given committee size
func (mock *MockHashOracle) calcThreshold(committeeSize int) uint32 {
	mock.mutex.RLock()
	numClients := len(mock.clients)
	mock.mutex.RUnlock()

	if numClients == 0 {
		log.Error("Called calcThreshold with 0 clients registered")
		return 0
	}

	if committeeSize > numClients {
		log.Error("Requested for a committee bigger than the number of registered clients. Expected at least %v clients Actual: %v",
			committeeSize, numClients)
		return 0
	}

	return uint32(uint64(committeeSize) * uint64(mock.hasher.MaxValue()) / uint64(numClients))
}

// Eligible if a proof is valid for a given committee size
func (mock *MockHashOracle) Eligible(instanceID *InstanceId, K int, committeeSize int, pubKey string, proof []byte) bool {
	if proof == nil {
		log.Warning("Oracle query with proof=nil. Returning false")
		return false
	}

	// calculate hash of proof
	proofHash := mock.hasher.Hash(proof)
	if proofHash <= mock.calcThreshold(committeeSize) { // check threshold
		return true
	}

	return false
}

type FixedRolacle struct {
	clients map[string]struct{}
	emaps   map[int]map[string]struct{}
	mutex   sync.Mutex
	mapRW   sync.RWMutex
}

func newFixedRolacle(defaultSize int) *FixedRolacle {
	rolacle := &FixedRolacle{}
	rolacle.clients = make(map[string]struct{}, defaultSize)
	rolacle.emaps = make(map[int]map[string]struct{}, defaultSize) // actually shouldn't expect same size

	return rolacle
}

func (fo *FixedRolacle) Register(client string) {
	fo.mutex.Lock()

	if _, exist := fo.clients[client]; exist {
		fo.mutex.Unlock()
		return
	}

	fo.clients[client] = struct{}{}
	fo.mutex.Unlock()
}

func (fo *FixedRolacle) Unregister(client string) {
	fo.mutex.Lock()
	delete(fo.clients, client)
	fo.mutex.Unlock()
}

func generateEligibility(clients map[string]struct{}, size int) map[string]struct{} {
	emap := make(map[string]struct{}, size)

	i := 0
	for k := range clients { // randomly pass on clients
		if i == size { // pick exactly size
			break
		}

		emap[k] = struct{}{}
		i++
	}

	return emap
}

func (fo *FixedRolacle) Eligible(instanceID *InstanceId, K int, committeeSize int, pubKey string, proof []byte) bool {
	fo.mapRW.RLock()
	total := len(fo.clients)
	fo.mapRW.RUnlock()

	// normalize committee size
	size := committeeSize
	if committeeSize > total {
		log.Warning("committee size bigger than the number of clients")
		size = total
	}

	fo.mapRW.Lock()
	// generate if not exist for the requested K
	if _, exist := fo.emaps[K]; !exist {
		fo.emaps[K] = generateEligibility(fo.clients, size)
	}
	fo.mapRW.Unlock()

	// get eligibility result
	_, exist := fo.emaps[K][pubKey]

	return exist
}
