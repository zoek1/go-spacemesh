package mesh

import (
	"container/list"
)

type Vote [2]vote
type Tally [2]int
type vote int

func (a Vote) Add(v *Vote) Vote {
	a[0] += v[0]
	a[1] += v[1]
	return a
}

type NinjaBlock struct {
	Id         BlockID
	LayerIndex LayerID
	Data       []byte
	BlockVotes map[BlockID]bool
	ViewEdges  map[BlockID]struct{}
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

const ( //Vote
	Support vote = 1
	Abstain vote = 0
	Against vote = -1
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
	TGood              map[int]votingPattern
	TSupport           map[votingPattern]int
	TEffectiveSupport  map[votingPattern]int
	TEffective         map[*NinjaBlock]votingPattern
	TExplicit          map[*NinjaBlock]map[LayerID]votingPattern
	TIncomplete        map[votingPattern]bool
	TPattern           map[votingPattern][]*NinjaBlock
	TEffectiveToBlocks map[votingPattern][]*NinjaBlock
	TPatSupport        map[votingPattern]map[int]votingPattern
	TTally             map[votingPattern]map[*NinjaBlock]Tally
	TVote              map[votingPattern]map[*NinjaBlock]Vote // global opinion
	TLocalVotes        map[votingPattern]map[*NinjaBlock]Vote
	TCorrect           map[*NinjaBlock]map[votingPattern]Vote
}

func (ni ninjaTortoise) GlobalOpinion(p votingPattern, x *NinjaBlock) vote {
	v := ni.TTally[p][x]
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

//UpdateCorrectionVectors
func (ni ninjaTortoise) UpdateCorrectionVectors(p votingPattern) {
	for _, b := range ni.TEffectiveToBlocks[p] { //for all b whos effective vote is p
		for _, x := range ni.TPattern[p] { //for all b in pattern p
			if _, found := ni.TExplicit[b][x.Layer()]; found { //if Texplicit[b][x]!=0 check correctness of x.layer and found
				ni.TCorrect[x][p] = ni.TVote[p][x] //Tcorrect[b][x] = -Tvote[p][x] ?????? //todo ask about -
			}
		}
	}
}

func (ni ninjaTortoise) UpdatePatternTally(pBase votingPattern, g votingPattern, p votingPattern) {

	//init
	stack := list.New()
	for b := range ni.TPattern[pBase] {
		stack.PushFront(b)
	}
	// find bfs this sucker
	var b *NinjaBlock
	corr := make(map[votingPattern]Vote)
	effCount := 0
	for b = stack.Front().Value.(*NinjaBlock); b != nil; {
		//corr = corr + TCorrect[B]
		for k, v := range ni.TCorrect[b] {
			corr[k] = corr[k].Add(&v)
		}
		effCount++
		//push children to bfs queue
		for bChild := range b.ViewEdges {
			if b.Layer() > pBase.Layer() {
				stack.PushBack(bChild)
			}
		}
	}
	//ni.TTally[p] <- ni.TTally[p] + ni.TVote[g]*effCount + corr

}

func (ni ninjaTortoise) UpdateTables(p votingPattern) {

}
