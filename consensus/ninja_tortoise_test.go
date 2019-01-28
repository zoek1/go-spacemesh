package consensus

import (
	"fmt"
	"github.com/spacemeshos/go-spacemesh/crypto"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"math/rand"
	"testing"
	"time"
)

func TestNinjaTortoise_UpdateTables(t *testing.T) {
	alg := NewNinjaTortoise(200)
	l := createGenesisLayer()
	alg.UpdateTables(l.Blocks(), l.Index())
	for i := 0; i < 11-1; i++ {
		lyr := createFullPointingLayer(l, 200)
		start := time.Now()
		l = lyr
		alg.UpdateTables(l.Blocks(), l.Index())
		log.Info("Time to process layer: %v ", time.Since(start))
	}
}

func NewNinjaTortoise(layerSize uint32) *ninjaTortoise {
	return &ninjaTortoise{
		Log:                log.New("optimized tortoise ", "", ""),
		LayerSize:          layerSize,
		pBase:              nil,
		blocks:             make(map[mesh.BlockID]*ninjaBlock, K*layerSize),
		tEffective:         make(map[mesh.BlockID]*votingPattern, K*layerSize),
		tCorrect:           make(map[mesh.BlockID]map[votingPattern]*vec, K*layerSize),
		layerBlocks:        make(map[mesh.LayerID][]mesh.BlockID, layerSize),
		tExplicit:          make(map[mesh.BlockID]map[mesh.LayerID]*votingPattern, K),
		tGood:              make(map[mesh.LayerID]votingPattern, K),
		tSupport:           make(map[votingPattern]int, layerSize),
		tPattern:           make(map[votingPattern][]mesh.BlockID, layerSize),
		tVote:              make(map[votingPattern]map[mesh.BlockID]*vec, layerSize),
		tTally:             make(map[votingPattern]map[mesh.BlockID]*vec, layerSize),
		tComplete:          make(map[votingPattern]struct{}, layerSize),
		tEffectiveToBlocks: make(map[votingPattern][]mesh.BlockID, layerSize),
		tPatSupport:        make(map[votingPattern]map[mesh.LayerID]*votingPattern, layerSize),
	}
}

func TestNinjaTortoise_UpdateCorrectionVectors(t *testing.T) {
	alg := NewNinjaTortoise(2)
	l := createGenesisLayer()
	alg.UpdateTables(l.Blocks(), Genesis)
	for i := 0; i < 20; i++ {
		lyr := createLayer(l, 2, 1)
		start := time.Now()
		alg.UpdateTables(lyr.Blocks(), lyr.Index())
		log.Info("Time to process layer: %v ", time.Since(start))
		l = lyr
	}

	for k, v := range alg.tVote {
		fmt.Println("key ", k, "val ", v)
	}
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
	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator
	indexes := make([]int, 0, patternSize)
	for i := 0; i < patternSize; i++ {
		indexes = append(indexes, rand.Intn(blocksInLayer))
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
	alg := NewNinjaTortoise(2)
	l := createGenesisLayer()
	alg.UpdateTables(l.Blocks(), Genesis)
	for i := 0; i < 3; i++ {
		lyr := createLayer(l, 2, 1)
		start := time.Now()
		alg.UpdateTables(lyr.Blocks(), lyr.Index())
		log.Info("Time to process layer: %v ", time.Since(start))
		l = lyr
	}

	for k, v := range alg.tVote {
		fmt.Println("key ", k, "val ", v)
	}

}

func TestForEachInView(t *testing.T) {

}
