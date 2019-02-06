package hare

import (
	"github.com/spacemeshos/go-spacemesh/hare/config"
	"github.com/spacemeshos/go-spacemesh/p2p"
	"github.com/spacemeshos/go-spacemesh/p2p/service"
	"testing"
	"time"
)

// Test the consensus process as a whole

var skipBlackBox = false

type fullRolacle interface {
	Registrable
	Rolacle
}

type HareSuite struct {
	termination Closer
	procs       []*ConsensusProcess
	dishonest   []*ConsensusProcess
	initialSets []*Set // all initial sets
	honestSets  []*Set // initial sets of honest
	outputs     []*Set
	name        string
	cfg         config.Config
}

func newHareSuite() *HareSuite {
	hs := new(HareSuite)
	hs.termination = NewCloser()
	hs.outputs = make([]*Set, 0)

	return hs
}

func (his *HareSuite) fill(isHonest bool, set *Set, begin int, end int) {
	for i := begin; i <= end; i++ {
		his.initialSets[i] = set

		if isHonest {
			his.honestSets = append(his.honestSets, set)
		}
	}
}

func (his *HareSuite) waitForTermination() {
	for _, p := range his.procs {
		<-p.CloseChannel()
		his.outputs = append(his.outputs, p.s)
	}

	his.termination.Close()
}

func (his *HareSuite) WaitForTimedTermination(t *testing.T, timeout time.Duration) {
	timer := time.After(timeout)
	go his.waitForTermination()
	select {
	case <-timer:
		t.Fatal("Timeout")
		return
	case <-his.termination.CloseChannel():
		his.checkResult(t)
		return
	}
}

func (his *HareSuite) checkResult(t *testing.T) {
	// build world of values (U)
	u := his.initialSets[0]
	for i := 1; i < len(his.initialSets); i++ {
		u = u.Union(his.initialSets[i])
	}

	// check consistency
	for i := 0; i < len(his.outputs)-1; i++ {
		if !his.outputs[i].Equals(his.outputs[i+1]) {
			t.Errorf("Consistency check failed: Expected: %v Actual: %v", his.outputs[i], his.outputs[i+1])
		}
	}

	// build validity 1 values
	validity1 := NewEmptySet(his.cfg.SetSize)
	ref := NewRefCountTracker(his.cfg.SetSize)
	for _, s := range his.honestSets {
		for _, v := range s.values {
			ref.Track(v)
			if ref.CountStatus(v) >= uint32(his.cfg.F+1) {
				validity1.Add(v)
			}
		}
	}

	// check that the output contains validity1
	if !validity1.IsSubSetOf(his.outputs[0]) {
		t.Errorf("Validity 1 failed: output does not contain the intersection of honest parties. Expected %v to be a subset of %v", validity1, his.outputs[0])
	}

	// build validity 2 values
	validity2 := NewEmptySet(his.cfg.SetSize)
	ref = NewRefCountTracker(his.cfg.SetSize)
	for _, s := range his.initialSets {
		for _, v := range s.values {
			ref.Track(v)
			if ref.CountStatus(v) >= uint32(his.cfg.F+1) {
				validity2.Remove(v)
			} else {
				validity2.Add(v)
			}
		}
	}

	// check that the output has no intersection with the complement of the union of honest
	for _, v := range his.outputs[0].values {
		if validity2.Contains(v) {
			t.Error("Validity 2 failed: unexpected value encountered: ", v)
		}
	}
}

type ConsensusTest struct {
	*HareSuite
}

func newConsensusTest(cfg config.Config) *ConsensusTest {
	ct := new(ConsensusTest)
	ct.HareSuite = newHareSuite()
	ct.cfg = cfg
	ct.honestSets = make([]*Set, 0)

	return ct
}

func (test *ConsensusTest) Create(N int, create func()) {
	for i := 0; i < N; i++ {
		create()
	}
}

func startProcs(procs []*ConsensusProcess) {
	for _, proc := range procs {
		proc.Start()
	}
}

func (test *ConsensusTest) Start() {
	go startProcs(test.procs)
	go startProcs(test.dishonest)
}

func createConsensusProcess(isHonest bool, cfg config.Config, oracle fullRolacle, network p2p.Service, initialSet *Set) *ConsensusProcess {
	broker := NewBroker(network)
	output := make(chan TerminationOutput, 1)
	signing := NewMockSigning()
	oracle.Register(isHonest, signing.Verifier().String())
	proc := NewConsensusProcess(cfg, *instanceId1, initialSet, oracle, signing, network, output)
	broker.Register(proc)
	broker.Start()

	return proc
}

func TestConsensusFixedOracle(t *testing.T) {
	cfg := config.Config{N: 16, F: 8, SetSize: 1, RoundDuration: time.Second * time.Duration(1)}
	test := newConsensusTest(cfg)
	sim := service.NewSimulator()
	totalNodes := 20
	test.initialSets = make([]*Set, totalNodes)
	set1 := NewSetFromValues(value1)
	test.fill(true, set1, 0, totalNodes-1)
	oracle := newFixedRolacle(totalNodes)
	i := 0
	creationFunc := func() {
		s := sim.NewNode()
		proc := createConsensusProcess(true, cfg, oracle, s, test.initialSets[i])
		test.procs = append(test.procs, proc)
		i++
	}
	test.Create(totalNodes, creationFunc)
	test.Start()
	test.WaitForTimedTermination(t, 30*time.Second)
}

func TestSingleValueForHonestSet(t *testing.T) {
	if skipBlackBox {
		t.Skip()
	}

	cfg := config.Config{N: 50, F: 25, SetSize: 1, RoundDuration: time.Second * time.Duration(1)}
	test := newConsensusTest(cfg)
	totalNodes := 50
	sim := service.NewSimulator()
	test.initialSets = make([]*Set, totalNodes)
	set1 := NewSetFromValues(value1)
	test.fill(true, set1, 0, totalNodes-1)
	oracle := newFixedRolacle(totalNodes)
	i := 0
	creationFunc := func() {
		s := sim.NewNode()
		proc := createConsensusProcess(true, cfg, oracle, s, test.initialSets[i])
		test.procs = append(test.procs, proc)
		i++
	}
	test.Create(totalNodes, creationFunc)
	test.Start()
	test.WaitForTimedTermination(t, 30*time.Second)
}

func TestAllDifferentSet(t *testing.T) {
	if skipBlackBox {
		t.Skip()
	}

	cfg := config.Config{N: 10, F: 5, SetSize: 5, RoundDuration: time.Second * time.Duration(1)}
	test := newConsensusTest(cfg)
	sim := service.NewSimulator()
	totalNodes := 10
	test.initialSets = make([]*Set, totalNodes)

	base := NewSetFromValues(value1, value2)
	test.initialSets[0] = base
	test.initialSets[1] = NewSetFromValues(value1, value2, value3)
	test.initialSets[2] = NewSetFromValues(value1, value2, value4)
	test.initialSets[3] = NewSetFromValues(value1, value2, value5)
	test.initialSets[4] = NewSetFromValues(value1, value2, value6)
	test.initialSets[5] = NewSetFromValues(value1, value2, value7)
	test.initialSets[6] = NewSetFromValues(value1, value2, value8)
	test.initialSets[7] = NewSetFromValues(value1, value2, value9)
	test.initialSets[8] = NewSetFromValues(value1, value2, value10)
	test.initialSets[9] = NewSetFromValues(value1, value2, value3, value4)
	for i := 0; i < 10; i++ {
		test.honestSets = append(test.honestSets, test.initialSets[i])
	}

	oracle := newFixedRolacle(totalNodes)
	i := 0
	creationFunc := func() {
		s := sim.NewNode()
		proc := createConsensusProcess(true, cfg, oracle, s, test.initialSets[i])
		test.procs = append(test.procs, proc)
		i++
	}
	test.Create(totalNodes, creationFunc)
	test.Start()
	test.WaitForTimedTermination(t, 30*time.Second)
}

func TestSndDelayedDishonest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	cfg := config.Config{N: 40, F: 20, SetSize: 5, RoundDuration: time.Second * time.Duration(2)}
	test := newConsensusTest(cfg)
	sim := service.NewSimulator()
	totalNodes := 50
	test.initialSets = make([]*Set, totalNodes)
	honest1 := NewSetFromValues(value1, value2, value4, value5)
	honest2 := NewSetFromValues(value1, value3, value4, value6)
	dishonest := NewSetFromValues(value3, value5, value6, value7)
	test.fill(true, honest1, 0, 15)
	test.fill(true, honest2, 16, totalNodes/2+1)
	test.fill(false, dishonest, totalNodes/2+2, totalNodes-1)

	oracle := newFixedRolacle(totalNodes)
	i := 0
	honestFunc := func() {
		s := sim.NewNode()
		proc := createConsensusProcess(true, cfg, oracle, s, test.initialSets[i])
		test.procs = append(test.procs, proc)
		i++
	}

	// create honest
	test.Create(totalNodes/2+1, honestFunc)

	// create dishonest
	dishonestFunc := func() {
		s := sim.NewFaulty(true, 6, 0) // only broadcast delay
		proc := createConsensusProcess(false, cfg, oracle, s, test.initialSets[i])
		test.dishonest = append(test.dishonest, proc)
		i++
	}
	test.Create(totalNodes/2-1, dishonestFunc)

	test.Start()
	test.WaitForTimedTermination(t, 30*time.Second)
}

func TestRecvDelayedDishonest(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	cfg := config.Config{N: 40, F: 20, SetSize: 5, RoundDuration: time.Second * time.Duration(2)}
	test := newConsensusTest(cfg)
	sim := service.NewSimulator()
	totalNodes := 50
	test.initialSets = make([]*Set, totalNodes)
	honest1 := NewSetFromValues(value1, value2, value4, value5)
	honest2 := NewSetFromValues(value1, value3, value4, value6)
	dishonest := NewSetFromValues(value3, value5, value6, value7)
	test.fill(true, honest1, 0, 15)
	test.fill(true, honest2, 16, 2*totalNodes/3)
	test.fill(true, dishonest, 2*totalNodes/3+1, totalNodes-1)

	oracle := newFixedRolacle(totalNodes)
	i := 0
	honestFunc := func() {
		s := sim.NewNode()
		proc := createConsensusProcess(true, cfg, oracle, s, test.initialSets[i])
		test.procs = append(test.procs, proc)
		i++
	}

	// create honest
	test.Create(totalNodes/2+1, honestFunc)

	// create dishonest
	dishonestFunc := func() {
		s := sim.NewFaulty(true, 0, 6) // delay rcv
		proc := createConsensusProcess(false, cfg, oracle, s, test.initialSets[i])
		test.dishonest = append(test.dishonest, proc)
		i++
	}
	test.Create(totalNodes/2-1, dishonestFunc)

	test.Start()
	test.WaitForTimedTermination(t, 30*time.Second)
}
