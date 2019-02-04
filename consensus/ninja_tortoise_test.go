package consensus

import (
	"github.com/spacemeshos/go-spacemesh/crypto"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"github.com/stretchr/testify/assert"
	"math"
	"math/rand"
	"testing"
	"time"
)

func NewNinjaTortoise(layerSize uint32) *ninjaTortoise {
	return &ninjaTortoise{
		Log:                log.New("optimized tortoise ", "", ""),
		LayerSize:          layerSize,
		pBase:              votingPattern{},
		blocks:             make(map[mesh.BlockID]*ninjaBlock, K*layerSize),
		tEffective:         make(map[mesh.BlockID]*votingPattern, K*layerSize),
		tCorrect:           make(map[mesh.BlockID]map[votingPattern]vec, K*layerSize),
		layerBlocks:        make(map[mesh.LayerID][]mesh.BlockID, layerSize),
		tExplicit:          make(map[mesh.BlockID]map[mesh.LayerID]*votingPattern, K),
		tGood:              make(map[mesh.LayerID]votingPattern, K),
		tSupport:           make(map[votingPattern]int, layerSize),
		tPattern:           make(map[votingPattern][]mesh.BlockID, layerSize),
		tVote:              make(map[votingPattern]map[mesh.BlockID]vec, layerSize),
		tTally:             make(map[votingPattern]map[mesh.BlockID]vec, layerSize),
		tComplete:          make(map[votingPattern]struct{}, layerSize),
		tEffectiveToBlocks: make(map[votingPattern][]mesh.BlockID, layerSize),
		tPatSupport:        make(map[votingPattern]map[mesh.LayerID]*votingPattern, layerSize),
	}
}

func TestNinjaTortoise_case1(t *testing.T) {
	alg := NewNinjaTortoise(2)
	l := createGenesisLayer()
	genesisId := l.Blocks()[0].ID()
	alg.initGenesis(l.Blocks(), Genesis)
	l = createMulExplicitLayer(l.Index()+1, []*mesh.Layer{l}, 2, 1)
	alg.initGenPlus1(l.Blocks(), Genesis+1)
	for i := 0; i < 1; i++ {
		lyr := createMulExplicitLayer(l.Index()+1, []*mesh.Layer{l}, 2, 2)
		start := time.Now()
		alg.UpdateTables(lyr.Blocks(), lyr.Index())
		alg.Info("Time to process layer: %v ", time.Since(start))
		l = lyr
		for b, vec := range alg.tTally[alg.pBase] {
			alg.Debug("------> tally for block %d according to complete pattern %d are %d", b, alg.pBase, vec)
		}
	}

	alg.Debug("print all block votes for new pbase ")

	assert.True(t, alg.tTally[alg.pBase][genesisId] == vec{2, 0}, "vote was %d insted of %d", alg.tTally[alg.pBase][genesisId], vec{2, 0})
}

//vote explicitly only for previous layer
//correction vectors have no affect here
func TestNinjaTortoise_case2(t *testing.T) {
	alg := NewNinjaTortoise(2)
	l := createGenesisLayer()
	genesisId := l.Blocks()[0].ID()
	alg.initGenesis(l.Blocks(), Genesis)
	l = createMulExplicitLayer(l.Index()+1, []*mesh.Layer{l}, 2, 1)
	alg.initGenPlus1(l.Blocks(), Genesis+1)
	for i := 0; i < 30; i++ {
		lyr := createMulExplicitLayer(l.Index()+1, []*mesh.Layer{l}, 2, 2)
		start := time.Now()
		alg.UpdateTables(lyr.Blocks(), lyr.Index())
		alg.Info("Time to process layer: %v ", time.Since(start))
		l = lyr
		for b, vec := range alg.tTally[alg.pBase] {
			alg.Debug("------> tally for block %d according to complete pattern %d are %d", b, alg.pBase, vec)
		}
		assert.True(t, alg.tTally[alg.pBase][genesisId] == vec{2 + 2*i, 0}, "lyr %d tally was %d insted of %d", lyr.Index(), alg.tTally[alg.pBase][genesisId], vec{2 + 2*i, 0})
	}
}

//vote explicitly for two previous layers
//correction vectors compensate for double count
func TestNinjaTortoise_case3(t *testing.T) {
	alg := NewNinjaTortoise(2)
	l := createGenesisLayer()
	genesisId := l.Blocks()[0].ID()
	alg.initGenesis(l.Blocks(), Genesis)
	l2 := l
	l = createMulExplicitLayer(l.Index()+1, []*mesh.Layer{l}, 2, 1)
	alg.initGenPlus1(l.Blocks(), Genesis+1)

	for i := 0; i < 30; i++ {
		lyr := createMulExplicitLayer(l.Index()+1, []*mesh.Layer{l, l2}, 2, 2)
		start := time.Now()
		alg.UpdateTables(lyr.Blocks(), lyr.Index())
		alg.Info("Time to process layer: %v ", time.Since(start))
		l2 = l
		l = lyr
		for b, vec := range alg.tTally[alg.pBase] {
			alg.Debug("------> tally for block %d according to complete pattern %d are %d", b, alg.pBase, vec)
		}
		assert.True(t, alg.tTally[alg.pBase][genesisId] == vec{2 + 2*i, 0}, "lyr %d tally was %d insted of %d", lyr.Index(), alg.tTally[alg.pBase][genesisId], vec{2 + 2*i, 0})
	}
}

func createMulExplicitLayer(index mesh.LayerID, prev []*mesh.Layer, blocksInLayer int, patternSize int) *mesh.Layer {
	ts := time.Now()
	coin := false
	// just some random Data
	data := []byte(crypto.UUIDString())
	l := mesh.NewLayer(index)
	var patterns [][]int
	for _, l := range prev {
		blocks := l.Blocks()
		blocksInPrevLayer := len(blocks)
		patterns = append(patterns, chooseRandomPattern(blocksInPrevLayer, int(math.Min(float64(blocksInPrevLayer), float64(patternSize)))))
	}

	for i := 0; i < blocksInLayer; i++ {
		bl := mesh.NewBlock(coin, data, ts, 1)
		for idx, pat := range patterns {
			for _, id := range pat {
				b := prev[idx].Blocks()[id]
				bl.AddVote(mesh.BlockID(b.Id))
			}
		}
		for _, prevBloc := range prev[len(prev)-1].Blocks() {
			bl.AddView(mesh.BlockID(prevBloc.Id))
		}
		l.AddBlock(bl)
	}
	log.Info("Created layer Id %v", l.Index())
	return l
}

func createLayer(prev *mesh.Layer, blocksInLayer int, patternSize int) *mesh.Layer {
	ts := time.Now()
	coin := false
	// just some random Data
	data := []byte(crypto.UUIDString())
	l := mesh.NewLayer(prev.Index() + 1)
	blocks := prev.Blocks()
	blocksInPrevLayer := len(blocks)
	pattern := chooseRandomPattern(blocksInPrevLayer, patternSize)
	for i := 0; i < blocksInLayer; i++ {
		bl := mesh.NewBlock(coin, data, ts, 1)
		for _, id := range pattern {
			b := blocks[id]
			bl.AddVote(mesh.BlockID(b.Id))
		}
		for _, prevBloc := range prev.Blocks() {
			bl.AddView(mesh.BlockID(prevBloc.Id))
		}
		l.AddBlock(bl)
	}
	log.Info("Created layer Id %v", l.Index())
	return l
}

func chooseRandomPattern(blocksInLayer int, patternSize int) []int {
	rand.Seed(time.Now().UnixNano())
	p := rand.Perm(blocksInLayer)
	indexes := make([]int, 0, patternSize)
	for _, r := range p[:patternSize] {
		indexes = append(indexes, r)
	}
	return indexes
}

func TestVec_Add(t *testing.T) {

}

func TestVec_Negate(t *testing.T) {

}

func TestVec_Multiply(t *testing.T) {

}

func TestNinjaTortoise_GlobalOpinion(t *testing.T) {

}

func TestNinjaTortoise_UpdatePatternTally(t *testing.T) {
}

func TestForEachInView(t *testing.T) {

}
