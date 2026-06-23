// Package labelvocab defines the HL7 FHIR authorization label vocabulary and the
// LabelSet algebra: lattice meet (intersection) and Bell-LaPadula dominance.
//
// Confidentiality forms a chain U<L<M<N<R<V; sensitivity categories form a powerset.
// Clearances are stored as their DOWNWARD CLOSURE so a single subset check correctly
// implements lattice dominance over the ordered confidentiality dimension.
// See ../../DECISION.md (Authorization core).
package labelvocab

// Label is a single authorization label (FHIR security label or category).
type Label string

// HL7 FHIR confidentiality chain (least → most restrictive).
const (
	ConfU Label = "conf:U"
	ConfL Label = "conf:L"
	ConfM Label = "conf:M"
	ConfN Label = "conf:N"
	ConfR Label = "conf:R"
	ConfV Label = "conf:V"
)

// confChain is the total order of the confidentiality dimension, ascending.
var confChain = []Label{ConfU, ConfL, ConfM, ConfN, ConfR, ConfV}

// LabelSet is a set of labels.
type LabelSet map[Label]struct{}

// New builds a LabelSet from the given labels.
func New(labels ...Label) LabelSet {
	s := make(LabelSet, len(labels))
	for _, l := range labels {
		s[l] = struct{}{}
	}
	return s
}

// Contains reports whether l is in the set.
func (s LabelSet) Contains(l Label) bool {
	_, ok := s[l]
	return ok
}

// Meet returns the lattice meet (set intersection) of s and other.
// Meet is monotone: it can only shrink, never grow — the basis of "closed under attack".
func (s LabelSet) Meet(other LabelSet) LabelSet {
	out := make(LabelSet)
	for l := range s {
		if other.Contains(l) {
			out[l] = struct{}{}
		}
	}
	return out
}

// Dominates reports whether s dominates required (Bell-LaPadula "no read up"):
// required ⊆ s. Use ClearanceFrom to expand a clearance to its downward closure first,
// so the confidentiality chain is respected by this plain subset check.
func (s LabelSet) Dominates(required LabelSet) bool {
	for l := range required {
		if !s.Contains(l) {
			return false
		}
	}
	return true
}

// ClearanceFrom expands a granted set to its downward closure on the confidentiality
// chain: a grant of ConfN implies {ConfU,ConfL,ConfM,ConfN}. Category labels pass through.
func ClearanceFrom(grants LabelSet) LabelSet {
	out := New()
	maxIdx := -1
	for l := range grants {
		if idx := confIndex(l); idx >= 0 {
			if idx > maxIdx {
				maxIdx = idx
			}
			continue
		}
		out[l] = struct{}{} // non-confidentiality (category) label
	}
	for i := 0; i <= maxIdx; i++ {
		out[confChain[i]] = struct{}{}
	}
	return out
}

func confIndex(l Label) int {
	for i, c := range confChain {
		if c == l {
			return i
		}
	}
	return -1
}
