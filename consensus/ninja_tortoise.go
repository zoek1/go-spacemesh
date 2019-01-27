package consensus

import (
	"container/list"
	"errors"
	"fmt"
	"github.com/spacemeshos/go-spacemesh/common"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"hash/fnv"
	"math"
	"sort"
)

type vec [2]int

const ( //Threshold
	K               = 5 //number of explicit layers to vote for
	Window          = 100
	LocalThreshold  = 0.8 //ThetaL
	GlobalThreshold = 0.6 //ThetaG
	Genesis         = 0
)

var ( //correction vectors type
	//Opinion
	Support = &vec{1, 0}
	Against = &vec{0, 1}
	Abstain = &vec{0, 0}
)

func (a *vec) Add(v *vec) *vec {
	if a == nil {
		return v
	} else if v == nil {
		return a
	}
	return &vec{a[0] + v[0], a[1] + v[1]}
}

func (a *vec) Negate() *vec {
	if a == nil {
		return &vec{0, 0}
	}
	a[0] = a[0] * -1
	a[1] = a[1] * -1
	return a
}

func (a *vec) Multiplay(x int) *vec {
	a[0] = a[0] * x
	a[1] = a[1] * x
	return a
}

type ninjaBlock struct {
	mesh.Block
	BlockVotes map[mesh.LayerID]*votingPattern //Explicit voting , Implicit votes is derived from the view of the latest Explicit voting pattern
}

type votingPattern struct {
	id uint32 //cant put a slice here wont work well with maps, we need to hash the blockids
	mesh.LayerID
}

func (vp votingPattern) Layer() mesh.LayerID {
	return vp.LayerID
}

//todo memory optimizations
type ninjaTortoise struct {
	log.Log
	LayerSize  uint32
	pBase      *votingPattern
	blocks     map[mesh.BlockID]*ninjaBlock
	tEffective map[mesh.BlockID]*votingPattern         //Explicit voting pattern of latest layer for a block
	tCorrect   map[mesh.BlockID]map[votingPattern]*vec //correction vectors

	layerBlocks map[mesh.LayerID][]mesh.BlockID
	tExplicit   map[mesh.BlockID]map[mesh.LayerID]*votingPattern // explict votes from block to layer pattern
	tGood       map[mesh.LayerID]votingPattern                   // good pattern for layer i

	tSupport           map[votingPattern]int                   //for pattern p the number of blocks that support p
	tPattern           map[votingPattern][]mesh.BlockID        // set of blocks that comprise pattern p
	tVote              map[votingPattern]map[mesh.BlockID]*vec // global opinion
	tTally             map[votingPattern]map[mesh.BlockID]*vec //for pattern p and block b count votes for b according to p
	tComplete          map[votingPattern]struct{}
	tEffectiveToBlocks map[votingPattern][]mesh.BlockID
	tPatSupport        map[votingPattern]map[mesh.LayerID]*votingPattern
}

func (ni *ninjaTortoise) processBlock(b *mesh.Block) *ninjaBlock {

	ni.Debug("process block: %d layer: %d  ", b.Id, b.Layer())

	patterns := make(map[mesh.LayerID][]mesh.BlockID)
	for _, bid := range b.BlockVotes {
		b, found := ni.blocks[bid]
		if !found {
			panic("unknown block!")
		}
		if _, found := patterns[b.Layer()]; !found {
			patterns[b.Layer()] = make([]mesh.BlockID, 0, ni.LayerSize)
		}
		patterns[b.Layer()] = append(patterns[b.Layer()], bid)
	}

	nb := &ninjaBlock{Block: *b}

	if b.Layer() == Genesis {
		log.Debug("process genesis block")
		ni.tEffective[b.ID()] = &votingPattern{}
		return nb
	}

	var effective *votingPattern
	for layerId, v := range patterns {
		vp := &votingPattern{id: getId(v), LayerID: layerId}
		ni.tPattern[*vp] = v
		nb.BlockVotes = make(map[mesh.LayerID]*votingPattern, K)
		nb.BlockVotes[layerId] = vp
		if effective == nil || layerId > effective.Layer() {
			effective = vp
		}
	}

	ni.tEffective[b.ID()] = effective
	v, found := ni.tPattern[*effective]
	if !found {
		v = make([]mesh.BlockID, 0, ni.LayerSize)
	}
	var pattern []mesh.BlockID = nil
	pattern = append(v, b.ID())
	ni.tEffectiveToBlocks[*effective] = pattern

	return nb
}

func getId(bids []mesh.BlockID) uint32 {
	sort.Slice(bids, func(i, j int) bool { return bids[i] < bids[j] })
	// calc
	h := fnv.New32()
	for i := 0; i < len(bids); i++ {
		h.Write(common.Uint32ToBytes(uint32(bids[i])))
	}
	// update
	return h.Sum32()
}

func (ni *ninjaTortoise) forBlockInView(blocks []mesh.BlockID, layer mesh.LayerID, foo func(*ninjaBlock)) map[mesh.LayerID]int {
	stack := list.New()
	for _, b := range blocks {
		stack.PushFront(b)
	}
	layerCounter := make(map[mesh.LayerID]int)
	set := make(map[mesh.BlockID]struct{})
	for b := stack.Front(); b != nil; b = stack.Front() {
		a := stack.Remove(stack.Front()).(mesh.BlockID)
		ni.Debug("handle block %d", a)
		block, found := ni.blocks[a]
		if !found {
			ni.Error("error block %d not found ", a)
		}
		layerCounter[block.Layer()]++
		foo(block)
		//push children to bfs queue
		for _, bChild := range block.ViewEdges {
			if block.Layer() > layer { //dont traverse too deep
				if _, found := set[bChild]; !found {
					set[bChild] = struct{}{}
					stack.PushBack(bChild)
				}
			}
		}
	}
	return layerCounter
}

func (ni *ninjaTortoise) globalOpinion(p *votingPattern, x *ninjaBlock) (*vec, error) {
	v, found := ni.tTally[*p][x.ID()]
	if !found {
		return nil, errors.New(fmt.Sprintf("%d not in %d view ", x.Id, p))
	}
	delta := float64(p.LayerID - x.Layer())
	threshold := float64(GlobalThreshold*delta) * float64(ni.LayerSize)
	ni.Debug("threshold: %d tally: %d ", threshold, v)
	if float64(v[0]) > threshold {
		return Support, nil
	} else if float64(v[1]) > threshold {
		return Against, nil
	} else {
		return Abstain, nil
	}
}

func (ni *ninjaTortoise) updateCorrectionVectors(p votingPattern) {
	for _, b := range ni.tEffectiveToBlocks[p] { //for all b who's effective vote is p
		for _, x := range ni.tPattern[p] { //for all b in pattern p
			if _, found := ni.tExplicit[b][p.Layer()]; found { //if Texplicit[b][x]!=0 check correctness of x.layer and found
				if _, found := ni.tCorrect[x]; !found {
					ni.tCorrect[x] = make(map[votingPattern]*vec)
				}
				ni.Debug("update correction vector for block %d", x)
				ni.tCorrect[x][p] = ni.tVote[p][x].Negate() //Tcorrect[b][x] = -Tvote[p][x]
			}
		}
	}
}

func (ni *ninjaTortoise) updatePatternTally(pBase votingPattern, g votingPattern, p votingPattern) {
	ni.Debug("update tally pbase id:%d layer:%d g id:%d layer:%d p id:%d layer:%d", pBase.id, pBase.Layer(), g.id, g.Layer(), p.id, p.Layer())
	// bfs this sucker to get all blocks who's effective vote pattern is g and layer id i s.t pBase<i<p
	//init p's tally to pBase tally
	stack := list.New()
	//include p
	for _, b := range ni.tPattern[p] {
		stack.PushBack(ni.blocks[b])
	}

	effCount := 0
	corr := &vec{0, 0}
	foo := func(b *ninjaBlock) {
		if *ni.tEffective[b.ID()] == g {
			corr = corr.Add(ni.tCorrect[b.ID()][g])
			effCount++
		}
	}

	ni.forBlockInView(ni.tPattern[p], g.Layer(), foo)

	for i := ni.pBase.Layer(); i <= g.Layer(); i++ {
		if layer, found := ni.layerBlocks[i]; found {
			for _, b := range layer {
				if v, found := ni.tVote[g][b]; found {
					tally := ni.tTally[p][b].Add(&vec{0, v[1]}).Multiplay(effCount).Add(corr)
					ni.Debug("tally for pattern %d  and block %d is %d", p.id, b, tally)
					ni.tTally[p][b] = tally //in g's view -> in p's view
				}
			}
		}
	}
}

//for all layers from pBase to i add b's votes, mark good layers
// return new minimal good layer
func (ni *ninjaTortoise) findMinimalGoodLayer(i mesh.LayerID, b []*ninjaBlock) mesh.LayerID {
	var j mesh.LayerID
	if i == Genesis+1 {
		j = Genesis
	} else if i < Window {
		j = ni.pBase.Layer() + 1
	} else {
		j = mesh.LayerID(math.Max(float64(ni.pBase.Layer()+1), float64(i-Window)))
	}

	var minGood mesh.LayerID
	for ; j < i; j++ {

		sUpdated := map[votingPattern]struct{}{}
		// update block votes on all patterns in blocks view
		for _, block := range b {
			//check if block votes for layer j explicitly or implicitly
			p, found := block.BlockVotes[j]
			if found {
				//explicit
				if _, expFound := ni.tExplicit[block.ID()]; !expFound {
					ni.tExplicit[block.ID()] = make(map[mesh.LayerID]*votingPattern, K*ni.LayerSize)
				}
				ni.tExplicit[block.ID()][j] = p
				ni.tSupport[*p]++         //add to supporting patterns
				sUpdated[*p] = struct{}{} //add to updated patterns

				//implicit
			} else if eff, effFound := ni.tEffective[block.ID()]; effFound {
				p, found = ni.tPatSupport[*eff][j]
				if found {
					ni.tSupport[*p]++         //add to supporting patterns
					sUpdated[*p] = struct{}{} //add to updated patterns
				}
			}
		}

		//todo do this as part of previous for if possible
		//for each p that was updated and not the good layer of j check if it is the good layer
		for p := range sUpdated {
			//if a majority supports p (p is good)
			//according to tal we dont have to know the exact amount, we can multiply layer size by number of layers
			jGood, found := ni.tGood[j]
			threshold := 0.5 * float64(mesh.LayerID(ni.LayerSize)*(i-p.Layer()))

			if (jGood != p || !found) && float64(ni.tSupport[p]) > threshold {
				ni.tGood[p.Layer()] = p
				//if p is the new minimal good layer
				if p.Layer() < i {
					minGood = p.Layer()
				}
			}
		}
	}
	ni.Debug("found minimal good layer %d", minGood)
	return minGood
}

func (ni *ninjaTortoise) addPatternVote(p votingPattern) func(b *ninjaBlock) {
	addPatternVote := func(b *ninjaBlock) {
		for _, ex := range ni.tExplicit[b.ID()] {
			for _, block := range ni.tPattern[*ex] {
				ni.Debug("add pattern vote for pattern %d block %d", p.id, block)
				ni.tTally[p][block].Add(&vec{1, 0})
			}
		}
	}
	return addPatternVote
}

func (ni *ninjaTortoise) HandleGenesis(block *mesh.Block) {

	nb := ni.processBlock(block)
	ni.blocks[nb.ID()] = nb
	ni.layerBlocks[Genesis] = append(ni.layerBlocks[Genesis], block.ID())
	p := votingPattern{id: getId([]mesh.BlockID{block.ID()}), LayerID: Genesis}
	ni.tPattern[p] = []mesh.BlockID{block.ID()}
	ni.tGood[Genesis] = p
	ni.pBase = &p
}

func (ni *ninjaTortoise) UpdateTables(B []*mesh.Block, i mesh.LayerID) mesh.LayerID { //i most recent layer
	ni.Debug("update tables layer %d", i)
	//initialize these tables //not in article
	b := make([]*ninjaBlock, 0, len(B))
	for _, block := range B {
		nb := ni.processBlock(block)
		ni.blocks[nb.ID()] = nb
		b = append(b, nb)
		ni.layerBlocks[i] = append(ni.layerBlocks[i], block.ID())
	}

	l := ni.findMinimalGoodLayer(i, b)

	//from minimal good pattern to current layer
	//update pattern tally for all good layers
	for j := l; j < i && i > Genesis; j++ {
		if p, found := ni.tGood[j]; found {
			//init p's tally to pBase tally
			for k, v := range ni.tTally[*ni.pBase] {
				if _, found := ni.tTally[p]; !found {
					ni.tTally[p] = make(map[mesh.BlockID]*vec)
				}
				ni.tTally[p][k] = v
			}

			//update pattern tally for each good layer on the way
			for k := ni.pBase.Layer() + 1; k < j; k++ {
				if gK, found := ni.tGood[k]; found {
					ni.updatePatternTally(*ni.pBase, gK, p)
				} else {
					ni.Error(" failed for ", k)
				}
			}

			// for each block in p's view add the pattern votes
			layerViewCounter := ni.forBlockInView(ni.tPattern[p], ni.pBase.Layer(), ni.addPatternVote(p))

			//update correction vectors after vote count
			ni.updateCorrectionVectors(p)
			flag := true

			//update vote for each block between pbase and p
			var layer []mesh.BlockID
			for idx := ni.pBase.Layer(); idx < j; idx++ {

				if layer, found = ni.layerBlocks[idx]; !found {
					panic("layer not found ")
				}

				bids := make([]mesh.BlockID, 0, ni.LayerSize)
				for _, bid := range layer {

					//if bid is not in p's view.
					// add negative vote multiplied by the amount of blocks in the view
					// between layer of b and layer of p
					if _, found := ni.tTally[p][bid]; !found {
						if _, found := ni.tTally[p]; !found {
							ni.tTally[p] = make(map[mesh.BlockID]*vec)
						}
						ni.tTally[p][bid] = sumNodesInView(layerViewCounter, idx, p.Layer())
					}
					b := ni.blocks[bid]
					if vote, err := ni.globalOpinion(&p, b); err == nil {
						if val, found := ni.tVote[p]; !found || val == nil {
							ni.tVote[p] = make(map[mesh.BlockID]*vec)
						}
						ni.tVote[p][b.ID()] = vote
						bids = append(bids, bid)
					} else {
						flag = false //not complete
					}
				}
				if val, found := ni.tPatSupport[p]; !found || val == nil {
					ni.tPatSupport[p] = make(map[mesh.LayerID]*votingPattern)
				}
				pid := getId(bids)
				ni.Debug("update support for ", p, " layer ", idx, " supported pattern", pid)
				ni.tPatSupport[p][i] = &votingPattern{id: getId(bids), LayerID: idx}

			}

			// update completeness of p
			if _, found := ni.tComplete[p]; flag && !found {
				ni.tComplete[p] = struct{}{}
				ni.Debug("found new complete and good pattern for layer %d pattern %d with %d support ", l, p.id, ni.tSupport[p])
				ni.pBase = &p
			}
		}
	}

	return ni.pBase.LayerID
}

func sumNodesInView(layerViewCounter map[mesh.LayerID]int, i mesh.LayerID, p mesh.LayerID) *vec {
	var sum int
	for sum = 0; i <= p; i++ {
		sum = sum + layerViewCounter[i]
	}
	return Against.Multiplay(sum)
}

//todo tally for blocks that are not in view
