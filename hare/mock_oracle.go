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

type Registrable interface {
	Register(isHonest bool, id string)
	Unregister(isHonest bool, id string)
}

type Rolacle interface {
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
	honest map[string]struct{}
	faulty map[string]struct{}
	emaps  map[int]map[string]struct{}
	mutex  sync.Mutex
	mapRW  sync.RWMutex
}

func newFixedRolacle(N int) *FixedRolacle {
	rolacle := &FixedRolacle{}
	rolacle.honest = make(map[string]struct{}, N)
	rolacle.faulty = make(map[string]struct{}, N)
	rolacle.emaps = make(map[int]map[string]struct{}, N)

	return rolacle
}

func (fo *FixedRolacle) update(m map[string]struct{}, client string) {
	fo.mutex.Lock()

	if _, exist := m[client]; exist {
		fo.mutex.Unlock()
		return
	}

	m[client] = struct{}{}

	fo.mutex.Unlock()
}

func (fo *FixedRolacle) Register(isHonest bool, client string) {
	if isHonest {
		fo.update(fo.honest, client)
	} else {
		fo.update(fo.faulty, client)
	}
}

func (fo *FixedRolacle) Unregister(isHonest bool, client string) {
	fo.mutex.Lock()
	if isHonest {
		delete(fo.honest, client)
	} else {
		delete(fo.faulty, client)
	}
	fo.mutex.Unlock()
}

func cloneMap(m map[string]struct{}) map[string]struct{} {
	c := make(map[string]struct{}, len(m))
	for k, v := range m {
		c[k] = v
	}

	return c
}

func pickUnique(pickCount int, orig map[string]struct{}, dest map[string]struct{}) {
	i := 0
	for k := range orig { // randomly pass on clients
		if i == pickCount { // pick exactly size
			break
		}

		dest[k] = struct{}{}
		delete(orig, k) // unique pick
		i++
	}
}

func (fo *FixedRolacle) generateEligibility(expCom int) map[string]struct{} {
	emap := make(map[string]struct{}, expCom)

	if expCom == 0 {
		return emap
	}

	expHonest := expCom/2 + 1
	if expHonest > len(fo.honest) {
		log.Error("Not enough registered honest. Expected %v<=%v", expHonest, len(fo.honest))
		panic("Not enough registered honest")
	}

	hon := cloneMap(fo.honest)
	pickUnique(expHonest, hon, emap)

	expFaulty := expCom/2 - 1
	if expFaulty > len(fo.faulty) {
		if len(fo.faulty) > 0 {
			log.Warning("Not enough registered dishonest to pick from. Expected %v<=%v. Picking %v instead", expFaulty, len(fo.faulty), len(fo.faulty))
		} else {
			log.Info("No registered dishonest to pick from. Picking honest instead")
		}
		expFaulty = len(fo.faulty)
	}

	if expFaulty > 0 { // pick faulty
		fau := cloneMap(fo.faulty)
		pickUnique(expFaulty, fau, emap)
	}

	rem := expCom/2-1 - len(fo.faulty)
	if rem > 0 { // need to pickUnique the remaining from honest
		pickUnique(rem, hon, emap)
	}

	return emap
}

func (fo *FixedRolacle) Eligible(instanceID *InstanceId, K int, committeeSize int, pubKey string, proof []byte) bool {
	fo.mapRW.RLock()
	total := len(fo.honest) + len(fo.faulty)
	fo.mapRW.RUnlock()

	// normalize committee size
	size := committeeSize
	if committeeSize > total {
		log.Error("committee size bigger than the number of clients. Expected %v<=%v", committeeSize, total)
		size = total
	}

	fo.mapRW.Lock()
	// generate if not exist for the requested K
	if _, exist := fo.emaps[K]; !exist {
		fo.emaps[K] = fo.generateEligibility(size)
	}
	fo.mapRW.Unlock()

	// get eligibility result
	_, exist := fo.emaps[K][pubKey]

	return exist
}
