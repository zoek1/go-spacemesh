package consensus

import (
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"testing"
	"time"
)

func TestNinjaTortoise_UpdateTables(t *testing.T) {
	alg := NewNinjaTortoise()
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

func NewNinjaTortoise() *ninjaTortoise {
	return &ninjaTortoise{
		pBase:              &votingPattern{},
		blocks:             make(map[mesh.BlockID]*ninjaBlock, K*LayerSize),
		tEffective:         make(map[mesh.BlockID]*votingPattern, K*LayerSize),
		tCorrect:           make(map[mesh.BlockID]map[votingPattern]*vec, K*LayerSize),
		layerBlocks:        make(map[mesh.LayerID][]mesh.BlockID, LayerSize),
		tExplicit:          make(map[mesh.LayerID]map[mesh.BlockID]*votingPattern, K),
		tGood:              make(map[mesh.LayerID]*votingPattern, K),
		tSupport:           make(map[votingPattern]int, LayerSize),
		tPattern:           make(map[votingPattern][]mesh.BlockID, LayerSize),
		tVote:              make(map[votingPattern]map[mesh.BlockID]*vec, LayerSize),
		tTally:             make(map[votingPattern]map[mesh.BlockID]*vec, LayerSize),
		tComplete:          make(map[votingPattern]bool, LayerSize),
		tEffectiveToBlocks: make(map[votingPattern][]mesh.BlockID, LayerSize),
		tPatSupport:        make(map[votingPattern]map[mesh.LayerID]*votingPattern, LayerSize),
	}
}

func TestVec_Add(t *testing.T) {

}

func TestVec_Negate(t *testing.T) {

}

func TestVec_Multiply(t *testing.T) {

}

func TestNinjaTortoise_GlobalOpinion(t *testing.T) {

}

func TestNinjaTortoise_UpdateCorrectionVectors(t *testing.T) {

}

func TestNinjaTortoise_UpdatePatternTally(t *testing.T) {

}

func TestForEachInView(t *testing.T) {

}
