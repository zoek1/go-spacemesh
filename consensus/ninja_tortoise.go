package consensus

import (
	"container/list"
	"github.com/spacemeshos/go-spacemesh/mesh"
	"math"
)

type vec [2]int

var ( //correction vectors type

	//Vote
	Implicit = vec{1, -1} //Implicit +1 Explicit -1
	Explicit = vec{-1, 1} //Implicit -1 Explicit  1	//todo unused
	Neutral  = vec{0, 0}  //Implicit  0 Explicit  0 //todo unused

	//Opinion
	Support = vec{1, 0}
	Against = vec{-1, 0}
	Abstain = vec{0, 0}
)

func (a vec) Add(v vec) vec {
	a[0] += v[0]
	a[1] += v[1]
	return a
}

func (a vec) Negate() vec {
	a[0] = a[0] * -1
	a[1] = a[1] * -1
	return a
}

func (a vec) Multiplay(x int) vec {
	a[0] = a[0] * x
	a[1] = a[1] * x
	return a
}

type NinjaBlock struct {
	*mesh.Block
	BlockVotes map[mesh.LayerID]*votingPattern //Explicit voting , Implicit votes is derived from the view of the latest Explicit voting pattern
	ViewEdges  map[BlockID]struct{}            //block view V(b)
}

func NewNinjaBlock(b *mesh.Block) *NinjaBlock {
	return nil
}

const ( //Threshold
	GlobalThreshold = 1   //ThetaG
	LocalThreshold  = 1   //ThetaL
	LayerSize       = 200 //Tave
)

type votingPattern struct {
	id []byte
	mesh.LayerID
}

func (vp votingPattern) Layer() mesh.LayerID {
	return vp.LayerID
}

//todo memory optimizations
type ninjaTortoise struct {
	mesh.Mesh
	pBase      *votingPattern
	tGood      map[mesh.LayerID]*votingPattern        // good pattern for layer i
	tSupport   map[*votingPattern]int                 //for pattern p the number of blocks that support p
	tEffective map[*NinjaBlock]*votingPattern         //Explicit voting pattern of latest layer for a block
	tPattern   map[*votingPattern][]*NinjaBlock       // set of blocks that comprise pattern p
	tVote      map[*votingPattern]map[*NinjaBlock]vec // global opinion

	tTally    map[*votingPattern]map[*NinjaBlock]vec          //for pattern p and block b count votes for b according to p
	tExplicit map[*NinjaBlock]map[mesh.LayerID]*votingPattern // explict votes from block to layer pattern

	tComplete          map[*votingPattern]bool                            //todo no initialization need to do explicitly at end of update tables ?
	tEffectiveToBlocks map[*votingPattern][]*NinjaBlock                   //todo no initialization need to do explicitly at start of update tables ?
	tPatSupport        map[*votingPattern]map[mesh.LayerID]*votingPattern //todo no initialization need to do explicitly at start of update tables ?
	tCorrect           map[*NinjaBlock]map[*votingPattern]vec             //correction vectors
}

func (ni ninjaTortoise) GlobalOpinion(p *votingPattern, x *NinjaBlock) vec {
	v := ni.tTally[p][x]
	delta := int(p.LayerID - x.Layer())
	threshold := GlobalThreshold * delta * LayerSize
	if v[0] > threshold {
		return Support
	} else if v[1] > threshold {
		return Against
	} else {
		return Abstain
	}
}

func (ni ninjaTortoise) UpdateCorrectionVectors(p *votingPattern) {
	for _, b := range ni.tEffectiveToBlocks[p] { //for all b who's effective vote is p
		for _, x := range ni.tPattern[p] { //for all b in pattern p
			if _, found := ni.tExplicit[b][x.Layer()]; found { //if Texplicit[b][x]!=0 check correctness of x.layer and found
				ni.tCorrect[x][p] = ni.tVote[p][x].Negate() //Tcorrect[b][x] = -Tvote[p][x]
			}
		}
	}
}

func (ni ninjaTortoise) UpdatePatternTally(pBase *votingPattern, g *votingPattern, p *votingPattern) {

	//init
	stack := list.New()
	for b := range ni.tPattern[p] {
		stack.PushFront(b)
	}
	// bfs this sucker to get all blocks who's effective vote pattern is g and layer id i Pbase<i<p
	var b *NinjaBlock
	corr := make(map[*votingPattern]vec)
	effCount := 0
	for b = stack.Front().Value.(*NinjaBlock); b != nil; {
		//corr = corr + TCorrect[B]
		for k, v := range ni.tCorrect[b] {
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
	for b := range ni.tTally[p] {
		ni.tTally[p][b] = ni.tTally[p][b].Add(ni.tVote[g][b].Multiplay(effCount).Add(corr[p]))
	}

}

func (ni ninjaTortoise) UpdateTables(b []*NinjaBlock, i mesh.LayerID) { //i most recent layer
	blocksVotimgOnLayer := make(map[mesh.LayerID]int)
	l := i
	//for j from max(Layer(pBase)+1,i-w) to i
	for j := mesh.LayerID(math.Max(float64(ni.pBase.Layer()+1), float64(i-100))); j < i; j++ {
		sUpdated := map[*votingPattern]struct{}{}
		for _, block := range b {
			var p *votingPattern
			var found bool
			//block explicitly votes for layer j
			if p, found = block.BlockVotes[j]; found {
				ni.tExplicit[block][j] = p
			} else {
				p, found = ni.tPatSupport[ni.tEffective[block]][j] //block implicitly votes for layer j
			}
			if found {
				ni.tSupport[p] = ni.tSupport[p] + 1 //add to supporting patterns
				sUpdated[p] = struct{}{}            //add to updated patterns
			}
		}
		//for each p that is updated and not the good layer of j check if it is now good
		for p := range sUpdated {
			//if p is good
			if ni.tGood[j] != p {
				//if a majority supports p
				if float64(ni.tSupport[p]) > 0.5*float64(blocksVotimgOnLayer[p.Layer()]) {
					ni.tGood[p.Layer()] = p
					//if p is the new minimal good layer
					if p.Layer() < l {
						l = p.Layer()
					}
				}
			}
		}
	}
	//iterate from l to i where l is the minimal newly good pattern
	for j := l; j < i; j++ {
		p := ni.tGood[j]
		ni.tTally[p] = ni.tTally[ni.pBase]
		for k := ni.pBase.Layer(); j < p.Layer(); k++ {
			if gK := ni.tGood[k]; gK != nil {
				ni.UpdatePatternTally(ni.pBase, gK, p)
			}
		}

		ni.forBlockInView(ni.tPattern[p], ni.pBase.Layer(), func(b *NinjaBlock) {
			if ni.pBase == ni.tExplicit[b][ni.pBase.Layer()] {
				ni.tTally[p][b] = ni.tTally[p][b].Add(Explicit) //todo check if this is true
			} else if ni.tExplicit[b][p.Layer()] != nil {
				ni.tTally[p][b] = ni.tTally[p][b].Add(Implicit) //todo check if this is true
			} else {
				ni.tTally[p][b] = ni.tTally[p][b].Add(Abstain) //todo check if this is true
			}
		})

		ni.forBlockInWindow(ni.pBase.Layer(), p.Layer(), func(b *NinjaBlock) {
			ni.tVote[p][b] = ni.GlobalOpinion(p, b) //todo this is not here
		})

		//todo check if p is complete if so pBase = p, this is not in the article
		// update completeness as part of vote update
		ni.UpdateCorrectionVectors(p) //todo ?????? check if this is the correct place to call this
		if ni.tComplete[p] {
			ni.pBase = p
		}

	}
}

func (ni ninjaTortoise) forBlockInView(blocks []*NinjaBlock, layer mesh.LayerID, foo func(*NinjaBlock)) {
	stack := list.New()
	for b := range blocks {
		stack.PushFront(b)
	}
	for b := stack.Front(); b != nil; {
		if block, err := ni.Mesh.GetBlock(b.Value.(mesh.BlockID)); err == nil {
			foo(NewNinjaBlock(block))
			//push children to bfs queue
			for bChild := range block.ViewEdges {
				if block.Layer() > layer { //dont traverse too deep
					stack.PushBack(bChild)
				}
			}
		}
	}
}

func (ni ninjaTortoise) forBlockInWindow(i mesh.LayerID, j mesh.LayerID, foo func(*NinjaBlock)) {
	for ; i <= j; j++ {
		if layer, err := ni.Mesh.GetLayer(i); err != nil { //todo relay on Database cache or implement specific cache here ?
			for _, b := range layer.Blocks() {
				foo(NewNinjaBlock(b))
			}
		}
	}
}

//todo where is negative support count  ???
//todo block wights to prevent attacks yuval's question
