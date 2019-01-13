package mesh

import (
	"container/list"
	"math"
)

type Vote [2]int
type Tally [2]int
type vote int

const (
	Support = 1
	Against = -1
	Abstain = 0
)

var ( //correction vectors type
	correction2 = Vote{1, -1} //implicit +1 explicit -1
	correction1 = Vote{-1, 1} //implicit -1 explicit  1
	correction3 = Vote{0, 0}  //implicit  0 explicit  0
)

func (a Tally) Add(v Vote) Tally {
	a[0] += v[0]
	a[1] += v[1]
	return a
}

func (a Vote) Add(v Vote) Vote {
	a[0] += v[0]
	a[1] += v[1]
	return a
}

func (a Vote) Negate() Vote {
	a[0] = a[0] * -1
	a[1] = a[1] * -1
	return a
}

func (a Vote) Multiplay(x int) Vote {
	a[0] = a[0] * x
	a[1] = a[1] * x
	return a
}

type NinjaBlock struct {
	Id         BlockID
	LayerIndex LayerID
	BlockVotes map[LayerID]*votingPattern //explicit voting , implicit votes is derived from the view of the latest explicit voting pattern
	ViewEdges  map[BlockID]struct{}       //block view V(b)
}

func (b *NinjaBlock) ID() BlockID {
	return b.Id
}

func (b *NinjaBlock) Layer() LayerID {
	return b.LayerIndex
}

const ( //Threshold
	GlobalThreshold = 1   //ThetaG
	LocalThreshold  = 1   //ThetaL
	LayerSize       = 200 //Tave
)

type votingPattern struct {
	id []byte
	LayerID
}

func (vp votingPattern) Layer() LayerID {
	return vp.LayerID
}

//todo memory optimizations
type ninjaTortoise struct {
	pBase              *votingPattern
	tGood              map[LayerID]*votingPattern                 // good pattern for layer i
	tSupport           map[*votingPattern]int                     //for pattern p the number of blocks that support p
	tEffectiveSupport  map[*votingPattern]int                     //number of blocks that support b
	tEffective         map[*NinjaBlock]*votingPattern             //explicit voting pattern of latest layer for a block
	tExplicit          map[*NinjaBlock]map[LayerID]*votingPattern // explict votes from block to layer pattern
	tIncomplete        map[*votingPattern]bool                    //is pattern complete
	tPattern           map[*votingPattern][]*NinjaBlock           // set of blocks that comprise pattern p
	tEffectiveToBlocks map[*votingPattern][]*NinjaBlock
	tPatSupport        map[*votingPattern]map[LayerID]*votingPattern
	tTally             map[*votingPattern]map[*NinjaBlock]Tally //for pattern p and block b count votes for b according to p
	tVote              map[*votingPattern]map[*NinjaBlock]Vote  // global opinion
	tLocalVotes        map[*votingPattern]map[*NinjaBlock]Vote
	tCorrect           map[*NinjaBlock]map[*votingPattern]Vote
}

func (ni ninjaTortoise) GlobalOpinion(p *votingPattern, x *NinjaBlock) vote {
	v := ni.tTally[p][x]
	delta := int(p.LayerID - x.Layer())
	threshold := GlobalThreshold * delta * layerSize
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
	corr := make(map[*votingPattern]Vote)
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

func (ni ninjaTortoise) UpdateTables(b []*NinjaBlock, i LayerID) {
	blocksVotimgOnLayer := make(map[LayerID]int)
	l := i
	//for j from max(Layer(pBase)+1,i-w) to i
	for j := LayerID(math.Max(float64(ni.pBase.Layer()+1), float64(i-100))); j < i; j++ {
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
	//iterate from l to i where l is the minimal good pattern
	for j := l; j < i; j++ {
		p := ni.tGood[j]
		ni.tTally[p] = ni.tTally[ni.pBase]
		for k := ni.pBase.Layer(); j < p.Layer(); k++ {
			if gK := ni.tGood[k]; gK != nil {
				ni.UpdatePatternTally(ni.pBase, gK, p)
			}
		}
		var b *NinjaBlock
		stack := list.New()
		for b := range ni.tPattern[p] {
			stack.PushFront(b)
		}
		for b = stack.Front().Value.(*NinjaBlock); b != nil; {
			//ni.tTally[p][b] = ni.tTally[p][b].Add(ni.tExplicit[b][j])
			//push children to bfs queue
			for bChild := range b.ViewEdges {
				if b.Layer() > ni.pBase.Layer() { //dont traverse too deep
					stack.PushBack(bChild)
				}
			}
		}
		//ni.tVote[p][x]:=ni.GlobalOpinion(p, x)
	}

}
