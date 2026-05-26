// Package dedupe identifies and groups duplicate contacts using configurable
// matching signals: email, phone, name (fuzzy), and organization.
package dedupe

import (
	"math"
	"sort"
	"strings"

	"github.com/init0/vcf-toolkit/internal/model"
	"github.com/init0/vcf-toolkit/internal/normalize"
)

type Config struct {
	EmailExactWeight   float64
	PhoneExactWeight   float64
	NameFuzzyWeight    float64
	OrganizationWeight float64
	Threshold          float64
	MinNameSimilarity  float64
}

func DefaultConfig() Config {
	return Config{
		EmailExactWeight:   0.40,
		PhoneExactWeight:   0.35,
		NameFuzzyWeight:    0.20,
		OrganizationWeight: 0.05,
		Threshold:          0.50,
		MinNameSimilarity:  0.60,
	}
}

// Deduplicate runs greedy pairwise clustering on the contact list.
// Returns clusters of duplicates, merged contacts, and a summary report.
func Deduplicate(contacts []model.Contact, cfg Config) model.DedupeResult {
	if len(contacts) == 0 {
		return model.DedupeResult{}
	}

	clusters := buildClusters(contacts, cfg)
	merged := mergeClusters(clusters)

	return model.DedupeResult{
		Clusters: clusters,
		Merged:   merged,
		Report: model.DedupeReport{
			TotalInput:      len(contacts),
			DuplicatesFound: len(contacts) - len(clusters),
			ClustersFormed:  len(clusters),
		},
	}
}

// buildClusters forms connected components over candidate matches.
// Candidates are generated via cheap blocking (email/phone exact keys, org exact,
// and a small name prefix key), then filtered via scorePair() >= threshold.
//
// This fixes the transitive-matching bug in greedy anchor clustering and avoids
// naive O(n^2) comparisons for typical datasets.
func buildClusters(contacts []model.Contact, cfg Config) [][]model.Contact {
	n := len(contacts)
	ds := newDisjointSet(n)

	// Build blocking buckets.
	emailBuckets := make(map[string][]int)
	phoneBuckets := make(map[string][]int)
	orgBuckets := make(map[string][]int)
	nameBuckets := make(map[string][]int)

	for i, c := range contacts {
		for _, e := range c.Emails {
			if ne := normalize.NormalizeEmailStr(e); ne != "" {
				emailBuckets[ne] = append(emailBuckets[ne], i)
			}
		}
		for _, p := range c.Phones {
			if dp := normalize.PhoneDigits(p); dp != "" {
				phoneBuckets[dp] = append(phoneBuckets[dp], i)
			}
		}
		if org := strings.TrimSpace(strings.ToLower(c.Organization)); org != "" {
			orgBuckets[org] = append(orgBuckets[org], i)
		}

		// Name bucketing is intentionally weak to avoid accidental giant buckets:
		// we only use the first 6 chars of the normalized name string.
		if ns := normalizeNameString(c.Name); ns != "" {
			if len(ns) > 6 {
				ns = ns[:6]
			}
			nameBuckets[ns] = append(nameBuckets[ns], i)
		}
	}

	// Evaluate bucket pairs and union when score >= threshold.
	applyBucket := func(bucket map[string][]int) {
		for _, idxs := range bucket {
			if len(idxs) < 2 {
				continue
			}
			// De-dupe indices in case multiple fields map to same key.
			sort.Ints(idxs)
			j := 0
			for i := 1; i < len(idxs); i++ {
				if idxs[i] != idxs[j] {
					j++
					idxs[j] = idxs[i]
				}
			}
			idxs = idxs[:j+1]

			for a := 0; a < len(idxs); a++ {
				for b := a + 1; b < len(idxs); b++ {
					i := idxs[a]
					k := idxs[b]
					if scorePair(contacts[i], contacts[k], cfg) >= cfg.Threshold {
						ds.union(i, k)
					}
				}
			}
		}
	}

	applyBucket(emailBuckets)
	applyBucket(phoneBuckets)
	applyBucket(orgBuckets)
	applyBucket(nameBuckets)

	// Build clusters from connected components in a stable order.
	byRoot := make(map[int][]int, n)
	rootMinIndex := make(map[int]int, n)
	for i := range n {
		r := ds.find(i)
		byRoot[r] = append(byRoot[r], i)
		if minIdx, ok := rootMinIndex[r]; !ok || i < minIdx {
			rootMinIndex[r] = i
		}
	}

	roots := make([]int, 0, len(byRoot))
	for r := range byRoot {
		roots = append(roots, r)
	}
	sort.Slice(roots, func(i, j int) bool {
		mi := rootMinIndex[roots[i]]
		mj := rootMinIndex[roots[j]]
		if mi == mj {
			return roots[i] < roots[j]
		}
		return mi < mj
	})

	clusters := make([][]model.Contact, 0, len(byRoot))
	for _, r := range roots {
		idxs := byRoot[r]
		sort.Ints(idxs)
		cluster := make([]model.Contact, 0, len(idxs))
		for _, idx := range idxs {
			cluster = append(cluster, contacts[idx])
		}
		clusters = append(clusters, cluster)
	}
	return clusters
}

// scorePair computes a weighted similarity score between two contacts.
// Returns a value in [0, 1] where higher values indicate a likely match.
func scorePair(a, b model.Contact, cfg Config) float64 {
	var score float64

	if s := matchEmails(a.Emails, b.Emails); s > 0 {
		score += s * cfg.EmailExactWeight
	}
	if s := matchPhones(a.Phones, b.Phones); s > 0 {
		score += s * cfg.PhoneExactWeight
	}
	if s := matchNames(a.Name, b.Name); s > cfg.MinNameSimilarity {
		score += s * cfg.NameFuzzyWeight
	}
	if s := matchOrganizations(a.Organization, b.Organization); s > 0 {
		score += s * cfg.OrganizationWeight
	}

	return score
}

func matchEmails(a, b []string) float64 {
	for _, ea := range a {
		na := normalize.NormalizeEmailStr(ea)
		if na == "" {
			continue
		}
		for _, eb := range b {
			nb := normalize.NormalizeEmailStr(eb)
			if nb == "" {
				continue
			}
			if na == nb {
				return 1.0
			}
		}
	}
	return 0.0
}

func matchPhones(a, b []string) float64 {
	for _, pa := range a {
		da := normalize.PhoneDigits(pa)
		if da == "" {
			continue
		}
		for _, pb := range b {
			if db := normalize.PhoneDigits(pb); da == db {
				return 1.0
			}
		}
	}
	return 0.0
}

func matchNames(a, b string) float64 {
	a = normalizeNameString(a)
	b = normalizeNameString(b)
	if a == "" || b == "" {
		return 0.0
	}
	return jaroWinklerSimilarity(a, b)
}

func normalizeNameString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	return strings.Join(strings.Fields(s), " ")
}

func matchOrganizations(a, b string) float64 {
	a = strings.TrimSpace(strings.ToLower(a))
	b = strings.TrimSpace(strings.ToLower(b))
	if a == "" || b == "" {
		return 0.0
	}
	if a == b {
		return 1.0
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return 0.8
	}
	return 0.0
}

type disjointSet struct {
	parent []int
	rank   []uint8
}

func newDisjointSet(n int) *disjointSet {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	return &disjointSet{
		parent: p,
		rank:   make([]uint8, n),
	}
}

func (ds *disjointSet) find(x int) int {
	for ds.parent[x] != x {
		ds.parent[x] = ds.parent[ds.parent[x]]
		x = ds.parent[x]
	}
	return x
}

func (ds *disjointSet) union(a, b int) {
	ra := ds.find(a)
	rb := ds.find(b)
	if ra == rb {
		return
	}
	if ds.rank[ra] < ds.rank[rb] {
		ds.parent[ra] = rb
		return
	}
	if ds.rank[ra] > ds.rank[rb] {
		ds.parent[rb] = ra
		return
	}
	ds.parent[rb] = ra
	ds.rank[ra]++
}

func mergeClusters(clusters [][]model.Contact) []model.MergedContact {
	var merged []model.MergedContact
	for _, cluster := range clusters {
		if len(cluster) == 1 {
			continue
		}
		merged = append(merged, mergeIntoOne(cluster))
	}
	return merged
}

// mergeIntoOne combines a cluster of duplicate contacts into a single canonical
// contact. It prefers non-empty values for name and org, and deduplicates
// phone/email lists by normalized value.
func mergeIntoOne(cluster []model.Contact) model.MergedContact {
	if len(cluster) == 0 {
		return model.MergedContact{}
	}

	best := cluster[0]
	var from []string

	for _, c := range cluster {
		if c.Source.File != "" && c.Source.Row > 0 {
			from = append(from, c.Source.File)
		}
		if best.Name == "" && c.Name != "" {
			best.Name = c.Name
		}
		if best.Organization == "" && c.Organization != "" {
			best.Organization = c.Organization
		}
	}

	best.Phones = nil
	best.Emails = nil
	phoneSet := make(map[string]bool)
	emailSet := make(map[string]bool)
	for _, c := range cluster {
		for _, p := range c.Phones {
			if d := normalize.PhoneDigits(p); d != "" && !phoneSet[d] {
				phoneSet[d] = true
				best.Phones = append(best.Phones, p)
			}
		}
		for _, e := range c.Emails {
			if ne := normalize.NormalizeEmailStr(e); ne != "" && !emailSet[ne] {
				emailSet[ne] = true
				best.Emails = append(best.Emails, e)
			}
		}
	}

	confidence := 1.0 - (1.0 / float64(len(cluster)+1))

	return model.MergedContact{
		Contact:     best,
		MergedFrom:  from,
		Confidence:  math.Round(confidence*100) / 100,
		MergeReason: []string{"merged_duplicate_cluster"},
	}
}

// jaroWinklerSimilarity computes string similarity with prefix boost.
// Well-suited for short strings like names.
func jaroWinklerSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	len1, len2 := len(s1), len(s2)
	if len1 == 0 || len2 == 0 {
		return 0.0
	}

	matchDistance := max(len1, len2)/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	s1Matches := make([]bool, len1)
	s2Matches := make([]bool, len2)

	var matches, transpositions float64

	for i := range len1 {
		start := max(0, i-matchDistance)
		end := min(i+matchDistance+1, len2)
		for j := start; j < end; j++ {
			if s2Matches[j] || s1[i] != s2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	var k int
	for i := range len1 {
		if !s1Matches[i] {
			continue
		}
		for j := k; j < len2; j++ {
			if !s2Matches[j] {
				continue
			}
			if s1[i] != s2[j] {
				transpositions++
			}
			k = j + 1
			break
		}
	}
	transpositions /= 2

	jaro := (matches/float64(len1) + matches/float64(len2) + (matches-transpositions)/matches) / 3.0

	prefix := 0
	for i := 0; i < min(4, min(len1, len2)); i++ {
		if s1[i] == s2[i] {
			prefix++
		} else {
			break
		}
	}
	return jaro + float64(prefix)*0.1*(1.0-jaro)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
