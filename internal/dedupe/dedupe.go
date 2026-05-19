// Package dedupe identifies and groups duplicate contacts using configurable
// matching signals: email, phone, name (fuzzy), and organization.
package dedupe

import (
	"math"
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

// buildClusters performs O(n²) greedy clustering. Each contact is compared
// pairwise against all subsequent unassigned contacts. Contacts scoring above
// the threshold are grouped into the same cluster.
func buildClusters(contacts []model.Contact, cfg Config) [][]model.Contact {
	n := len(contacts)
	assigned := make([]bool, n)
	var clusters [][]model.Contact

	for i := range n {
		if assigned[i] {
			continue
		}
		cluster := []model.Contact{contacts[i]}
		assigned[i] = true

		for j := i + 1; j < n; j++ {
			if assigned[j] {
				continue
			}
			if scorePair(contacts[i], contacts[j], cfg) >= cfg.Threshold {
				cluster = append(cluster, contacts[j])
				assigned[j] = true
			}
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
