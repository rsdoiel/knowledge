package main

import (
	"sort"

	knowledge "github.com/rsdoiel/knowledge"
)

// fuzzyTermsClose reports whether a and b are close enough to be treated as
// the same underlying term, for both excludeNearExisting (FC3) and
// clusterCandidateTerms (FC4). Raw distance is tried first -- length-
// pruned per design decision 10 -- and alone already reproduces a plain
// plural/typo pair correctly (chunking/chunkings, distance 1). Only when
// raw distance doesn't qualify does it fall back to comparing both sides'
// stripCommonSuffix forms, also length-pruned: the case that actually
// needs it is a genuine tense-variant pair like chunking/chunked, whose
// raw distance (3) exceeds any reasonable threshold but whose stems
// (both "chunk") are identical. This is a *symmetric* fallback -- unlike
// fuzzy-tag's own FuzzyMatchConceptNames, which only ever stems the
// candidate token, never the canonical concept name it's matched against
// -- because here both a and b are equally unverified candidate terms;
// there is no canonical side to protect from stemming. See
// fuzzy-concept-clustering-design.md's corrected decisions 3/4 for the
// full derivation: stemming both sides unconditionally (as first
// described) breaks the chunking/chunkings pair, and a raw-only check
// breaks the chunking/chunked pair -- only this two-tier form catches
// both.
func fuzzyTermsClose(a, b string, maxDistance int) (distance int, ok bool) {
	if len(a) < minFuzzyTermLength || len(b) < minFuzzyTermLength {
		return 0, false
	}
	if absInt(len(a)-len(b)) <= maxDistance {
		if d := knowledge.LevenshteinDistance(a, b); d <= maxDistance {
			return d, true
		}
	}
	sa, sb := knowledge.StripCommonSuffix(a), knowledge.StripCommonSuffix(b)
	if absInt(len(sa)-len(sb)) <= maxDistance {
		if d := knowledge.LevenshteinDistance(sa, sb); d <= maxDistance {
			return d, true
		}
	}
	return 0, false
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// minFuzzyTermLength gates fuzzyTermsClose against short-word noise --
// found live, smoke-testing against the real agents/knowledge.db: a flat
// distance-1 threshold with no length floor produces heavy false-positive
// clustering on short, common candidate terms purely by coincidence
// ("table" ~ "stable"/"able", "old" ~ "cold"/"told"/"hold"/"fold", "makes"
// ~ "man"/"map"), a different failure mode than the chain-drift risk
// decision 4 already guards against, and one the design's own "flat
// distance <= 1, no length-based split" wording (decision 5) didn't
// anticipate -- that decision is about not loosening the threshold for
// long words, not about tightening it for short ones. Below this length,
// a and b are never compared at all, in either fuzzyTermsClose tier.
// Provisional, like the rest of this feature's constants -- not validated
// beyond this one corpus.
const minFuzzyTermLength = 6

// fuzzyClusterDistance is the flat threshold design decision 5 sets for
// clustering/near-existing exclusion -- deliberately tighter and
// unconditional, unlike fuzzy-tag's length-based 1-or-2 split, since this
// clusters unverified candidates against other unverified candidates,
// where a looser threshold risks unintentionally collapsing two distinct
// new concepts before a human ever sees them separately.
const fuzzyClusterDistance = 1

// nearExistingMatch is one candidate token design decision 3 excludes from
// candidacy because it's a fuzzy near-miss of an already-known concept --
// fuzzy-tag's job, not concept suggest's.
type nearExistingMatch struct {
	Token    string `json:"token"`
	Concept  string `json:"concept"`
	Distance int    `json:"distance"`
}

// excludeNearExisting drops any token from occurrences that's fuzzy-close
// (fuzzyTermsClose, within fuzzyClusterDistance) to an already-known
// concept name, before clustering ever runs (design decision 3) -- an
// excluded token never becomes a cluster seed or member. survivors holds
// every occurrences key not excluded; nearExisting reports what was
// dropped and why, sorted by Token for deterministic output.
func excludeNearExisting(occurrences map[string]int, known map[string]bool) (survivors map[string]bool, nearExisting []nearExistingMatch) {
	survivors = make(map[string]bool, len(occurrences))
	for tok := range occurrences {
		survivors[tok] = true
	}
	for tok := range occurrences {
		bestDistance := -1
		bestConcept := ""
		for name := range known {
			if d, ok := fuzzyTermsClose(tok, name, fuzzyClusterDistance); ok && d > 0 {
				if bestDistance == -1 || d < bestDistance {
					bestDistance = d
					bestConcept = name
				}
			}
		}
		if bestDistance > 0 {
			delete(survivors, tok)
			nearExisting = append(nearExisting, nearExistingMatch{Token: tok, Concept: bestConcept, Distance: bestDistance})
		}
	}
	sort.Slice(nearExisting, func(i, j int) bool { return nearExisting[i].Token < nearExisting[j].Token })
	return survivors, nearExisting
}

// termCluster is one group of candidate terms design decision 4's star
// clustering merged: Seed is the canonical spelling (the highest-occurrence
// member, decision 6), Variants every other member. Occurrences sums
// across members; Items unions each member's item-index set (decision 7)
// -- an item mentioning two variants counts once toward the cluster's df,
// not twice.
type termCluster struct {
	Seed        string
	Variants    []string
	Occurrences int
	Items       map[int]bool
}

// clusterCandidateTerms groups survivors (excludeNearExisting's output) by
// star clustering, not chain/transitive clustering (design decision 4):
// sort by occurrence descending, term ascending on ties; the first
// unassigned token seeds a new cluster; every remaining unassigned token
// within fuzzyClusterDistance *of that seed specifically* joins it. A
// non-seed member is never itself compared against other members, which is
// what keeps an A-B-C-D chain (each adjacent pair close, endpoints not)
// from collapsing into one cluster -- the classic cat/cot/cog/dog drift the
// design's Motivation names.
func clusterCandidateTerms(occurrences map[string]int, itemCounts map[string]map[int]bool, survivors map[string]bool) []termCluster {
	terms := make([]string, 0, len(survivors))
	for tok := range survivors {
		terms = append(terms, tok)
	}
	sort.Slice(terms, func(i, j int) bool {
		if occurrences[terms[i]] != occurrences[terms[j]] {
			return occurrences[terms[i]] > occurrences[terms[j]]
		}
		return terms[i] < terms[j]
	})

	assigned := make(map[string]bool, len(terms))
	var clusters []termCluster
	for _, seed := range terms {
		if assigned[seed] {
			continue
		}
		assigned[seed] = true
		cluster := termCluster{
			Seed:        seed,
			Occurrences: occurrences[seed],
			Items:       cloneIndexSet(itemCounts[seed]),
		}
		for _, tok := range terms {
			if tok == seed || assigned[tok] {
				continue
			}
			if _, ok := fuzzyTermsClose(seed, tok, fuzzyClusterDistance); !ok {
				continue
			}
			assigned[tok] = true
			cluster.Variants = append(cluster.Variants, tok)
			cluster.Occurrences += occurrences[tok]
			for idx := range itemCounts[tok] {
				cluster.Items[idx] = true
			}
		}
		clusters = append(clusters, cluster)
	}
	return clusters
}

// cloneIndexSet copies m so a cluster's Items set can be mutated without
// aliasing the caller's own itemCounts map.
func cloneIndexSet(m map[int]bool) map[int]bool {
	out := make(map[int]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
