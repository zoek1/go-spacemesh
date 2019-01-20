package consensus

import (
	"container/list"
	"errors"
	"fmt"
	"github.com/spacemeshos/go-spacemesh/common"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"hash/fnv"
	"math"
	"sort"
)

type vec [2]int

const ( //Threshold
	GlobalThreshold = 1   //ThetaG
	LocalThreshold  = 1   //ThetaL
	LayerSize       = 200 //Tave
	k               = 5   //number of explicit layers to vote for
)

var ( //correction vectors type

	//Vote
	Implicit = &vec{1, -1} //Implicit +1 Explicit -1
	Explicit = &vec{-1, 1} //Implicit -1 Explicit  1	//todo unused
	Neutral  = &vec{0, 0}  //Implicit  0 Explicit  0 //todo unused

	//Opinion
	Support = &vec{1, 0}
	Against = &vec{-1, 0}
	Abstain = &vec{0, 0}
)

func (a *vec) Add(v *vec) *vec {
	a[0] += v[0]
	a[1] += v[1]
	return a
}

func (a *vec) Negate() *vec {
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
	*mesh.Block
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
	pBase      votingPattern
	blocks     map[mesh.BlockID]*ninjaBlock
	tEffective map[mesh.BlockID]*votingPattern         //Explicit voting pattern of latest layer for a block
	tCorrect   map[mesh.BlockID]map[votingPattern]*vec //correction vectors

	layerBlocks map[mesh.LayerID][]mesh.BlockID
	tExplicit   map[mesh.LayerID]map[mesh.BlockID]*votingPattern // explict votes from block to layer pattern
	tGood       map[mesh.LayerID]*votingPattern                  // good pattern for layer i

	tSupport           map[votingPattern]int                   //for pattern p the number of blocks that support p
	tPattern           map[votingPattern][]mesh.BlockID        // set of blocks that comprise pattern p
	tVote              map[votingPattern]map[mesh.BlockID]*vec // global opinion
	tTally             map[votingPattern]map[mesh.BlockID]*vec //for pattern p and block b count votes for b according to p
	tComplete          map[votingPattern]bool
	tEffectiveToBlocks map[votingPattern][]*ninjaBlock
	tPatSupport        map[votingPattern]map[mesh.LayerID]*votingPattern
}

func (ni ninjaTortoise) processBlock(b *mesh.Block) (nb *ninjaBlock) {

	patterns := make(map[mesh.LayerID][]mesh.BlockID)
	for _, bid := range b.BlockVotes {
		b, found := ni.blocks[bid]
		if !found {
			panic("unknown block!")
		}
		arr := patterns[b.Layer()]
		arr = append(arr, bid)
	}
	effective := mesh.LayerID(0)
	for k, v := range patterns {
		if k > effective {
			effective = k
		}
		vp := &votingPattern{id: getId(v), LayerID: k}
		ni.tPattern[*vp] = v
		nb.BlockVotes[k] = vp
	}
	nb.Block = b
	eff := nb.BlockVotes[effective]
	ni.tEffective[nb.ID()] = eff
	ni.tEffectiveToBlocks[*eff] = append(ni.tEffectiveToBlocks[*eff], nb)
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

func (ni ninjaTortoise) forBlockInView(blocks []mesh.BlockID, layer mesh.LayerID, foo func(*ninjaBlock)) {
	stack := list.New()
	for b := range blocks {
		stack.PushFront(b)
	}
	for b := stack.Front(); b != nil; {
		if block, found := ni.blocks[b.Value.(mesh.BlockID)]; found {
			foo(block)
			//push children to bfs queue
			for bChild := range block.ViewEdges {
				if block.Layer() > layer { //dont traverse too deep
					stack.PushBack(bChild)
				}
			}
		}
	}
}

func (ni ninjaTortoise) globalOpinion(p *votingPattern, x *ninjaBlock) (*vec, error) {
	v, found := ni.tTally[*p][x.ID()]
	if !found {
		return nil, errors.New(fmt.Sprintf("%d not in %d view ", x.Id, p))
	}
	delta := int(p.LayerID - x.Layer())
	threshold := GlobalThreshold * delta * LayerSize
	if v[0] > threshold {
		return Support, nil
	} else if v[1] > threshold {
		return Against, nil
	} else {
		return Abstain, nil
	}
}

func (ni ninjaTortoise) updateCorrectionVectors(p *votingPattern) {
	for _, b := range ni.tEffectiveToBlocks[*p] { //for all b who's effective vote is p
		for _, x := range ni.tPattern[*p] { //for all b in pattern p
			if _, found := ni.tExplicit[p.Layer()][b.ID()]; found { //if Texplicit[b][x]!=0 check correctness of x.layer and found
				ni.tCorrect[x][*p] = ni.tVote[*p][x].Negate() //Tcorrect[b][x] = -Tvote[p][x]
			}
		}
	}
}

func (ni ninjaTortoise) updatePatternTally(pBase *votingPattern, g *votingPattern, p *votingPattern) {
	//init
	stack := list.New()
	for b := range ni.tPattern[*p] {
		stack.PushFront(b)
	}
	// bfs this sucker to get all blocks who's effective vote pattern is g and layer id i Pbase<i<p
	var b *ninjaBlock
	corr := make(map[votingPattern]*vec)
	effCount := 0
	for b = stack.Front().Value.(*ninjaBlock); b != nil; {
		//corr = corr + TCorrect[B]
		for k, v := range ni.tCorrect[b.ID()] {
			corr[k] = corr[k].Add(v)
		}
		effCount++
		//push children to bfs queue
		for bChild := range b.ViewEdges {
			if b.Layer() > pBase.Layer() { //dont traverse too deep
				stack.PushBack(bChild)
			}
		}
	}
	for b := range ni.tTally[*p] {
		ni.tTally[*p][b] = ni.tTally[*p][b].Add(ni.tVote[*p][b].Multiplay(effCount).Add(corr[*p]))
	}

}

//for all layers from pBase to i add b's votes, mark good layers
// return new minimal good layer
func (ni ninjaTortoise) findMinimalGoodLayer(i mesh.LayerID, b []*ninjaBlock) mesh.LayerID {
	l := i
	for j := mesh.LayerID(math.Max(float64(ni.pBase.Layer()+1), float64(i-100))); j < i; j++ {

		sUpdated := map[*votingPattern]struct{}{}
		// update block votes on all patterns in blocks view
		for _, block := range b {
			//check if block votes for layer j explicitly or implicitly
			var found bool
			var p *votingPattern
			//explicit
			if p = block.BlockVotes[j]; p != nil {
				found = true
				ni.tExplicit[j][block.ID()] = p
			} else {
				p, found = ni.tPatSupport[*ni.tEffective[block.ID()]][j] //block implicitly votes for layer j
			}
			if found {
				ni.tSupport[*p]++        //add to supporting patterns
				sUpdated[p] = struct{}{} //add to updated patterns
			}

		}

		//todo do this as part of previous for if possible
		//for each p that was updated and not the good layer of j check if it is the good layer
		for p := range sUpdated {
			//if a majority supports p (p is good)
			//according to tal we dont have to know the exact amount, we can multiply layer size by number of layers
			if ni.tGood[j] != p && float64(ni.tSupport[*p]) > 0.5*float64(LayerSize*i-p.Layer()) {
				ni.tGood[p.Layer()] = p
				//if p is the new minimal good layer
				if p.Layer() < l {
					l = p.Layer()
				}
			}
		}
	}
	return l
}

func (ni ninjaTortoise) UpdateTables(B []*mesh.Block, i mesh.LayerID) { //i most recent layer

	//initialize these tables //not in article
	b := make([]*ninjaBlock, 0, len(B))
	for _, block := range B {
		b = append(b, ni.processBlock(block))
		ni.layerBlocks[i] = append(ni.layerBlocks[i], block.ID())
	}

	l := ni.findMinimalGoodLayer(i, b)

	//from minimal good pattern to current layer
	for j := l; j < i; j++ {
		p := *ni.tGood[j]                               //the good layer of j
		ni.tTally[p] = ni.tTally[ni.pBase]              //init tally for p
		for k := ni.pBase.Layer(); k < p.Layer(); k++ { //update pattern tally for each good layer on the way
			if gK := ni.tGood[k]; gK != nil {
				ni.updatePatternTally(&ni.pBase, gK, &p)
			}
		}

		addPatternVote := func(b *ninjaBlock) {
			var v *vec
			exp := ni.tExplicit[p.Layer()][b.ID()]
			if p == *exp && exp != nil {
				v = Explicit
			} else if ni.tExplicit[p.Layer()][b.ID()] != nil {
				v = Implicit
			} else {
				v = Abstain
			}
			ni.tTally[p][b.ID()] = ni.tTally[p][b.ID()].Add(v)
		}

		ni.forBlockInView(ni.tPattern[p], ni.pBase.Layer(), addPatternVote) // for each block in p's view add the pattern votes

		//for all blocks between p and p base update vote
		flag := true
		//update vote for each block between pbase and p
		for i := ni.pBase.Layer(); i <= j; i++ {
			if layer, found := ni.layerBlocks[i]; found {
				bids := make([]mesh.BlockID, 0, LayerSize)
				for idx, bid := range layer {
					b := ni.blocks[bid]
					if vote, err := ni.globalOpinion(&p, b); err == nil {
						ni.tVote[p][b.ID()] = vote
						bids[idx] = bid
					} else {
						flag = false //not complete
					}
				}
				ni.tPatSupport[p][i] = &votingPattern{id: getId(bids), LayerID: i}
			}
		}

		// update completeness of p
		ni.tComplete[p] = flag
		//update correction vectors after vote count
		ni.updateCorrectionVectors(&p)
		if ni.tComplete[p] {
			ni.pBase = p
		}

	}
}
