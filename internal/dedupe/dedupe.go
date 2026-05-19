package dedupe

import (
	"math"
	"strings"

	"github.com/init0/vcf-toolkit/internal/model"
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

func Deduplicate(contacts []model.Contact, cfg Config) model.DedupeResult {
	n := len(contacts)
	if n == 0 {
		return model.DedupeResult{
			Clusters: [][]model.Contact{},
			Merged:   []model.MergedContact{},
			Report:   model.DedupeReport{},
		}
	}

	clusters := buildClusters(contacts, cfg)
	merged := mergeClusters(clusters)
	report := model.DedupeReport{
		TotalInput:      n,
		DuplicatesFound: n - len(clusters),
		ClustersFormed:  len(clusters),
	}

	return model.DedupeResult{
		Clusters: clusters,
		Merged:   merged,
		Report:   report,
	}
}

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
			score := scorePair(contacts[i], contacts[j], cfg)
			if score >= cfg.Threshold {
				cluster = append(cluster, contacts[j])
				assigned[j] = true
			}
		}
		clusters = append(clusters, cluster)
	}
	return clusters
}

func scorePair(a, b model.Contact, cfg Config) float64 {
	var score float64

	if emailScore := matchEmails(a.Emails, b.Emails); emailScore > 0 {
		score += emailScore * cfg.EmailExactWeight
	}

	if phoneScore := matchPhones(a.Phones, b.Phones); phoneScore > 0 {
		score += phoneScore * cfg.PhoneExactWeight
	}

	if nameScore := matchNames(a.Name, b.Name); nameScore > cfg.MinNameSimilarity {
		score += nameScore * cfg.NameFuzzyWeight
	}

	if orgScore := matchOrganizations(a.Organization, b.Organization); orgScore > 0 {
		score += orgScore * cfg.OrganizationWeight
	}

	return score
}

func matchEmails(a, b []string) float64 {
	for _, ea := range a {
		na := normalizeEmail(ea)
		if na == "" {
			continue
		}
		for _, eb := range b {
			nb := normalizeEmail(eb)
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

func normalizeEmail(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	idx := strings.Index(email, "@")
	if idx < 0 {
		return ""
	}
	local := email[:idx]
	domain := email[idx+1:]
	if plus := strings.Index(local, "+"); plus >= 0 {
		local = local[:plus]
	}
	return local + "@" + domain
}

func matchPhones(a, b []string) float64 {
	for _, pa := range a {
		cleanedA := normalizePhoneDigits(pa)
		if cleanedA == "" {
			continue
		}
		for _, pb := range b {
			cleanedB := normalizePhoneDigits(pb)
			if cleanedB == "" {
				continue
			}
			if cleanedA == cleanedB {
				return 1.0
			}
		}
	}
	return 0.0
}

func normalizePhoneDigits(s string) string {
	s = removeNonNumeric(s)
	if len(s) == 0 {
		return ""
	}
	if s[0] == '0' {
		s = strings.TrimLeft(s, "0")
	}
	return s
}

func removeNonNumeric(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
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
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
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
		mc := mergeIntoOne(cluster)
		merged = append(merged, mc)
	}
	return merged
}

func mergeIntoOne(cluster []model.Contact) model.MergedContact {
	if len(cluster) == 0 {
		return model.MergedContact{}
	}

	best := cluster[0]
	var reasons []string
	var from []string

	for _, c := range cluster {
		if c.Source.File != "" && c.Source.Row > 0 {
			from = append(from, c.Source.File)
		}
		best = mergeField(best, c, "name", func() bool { return c.Name != "" })
		best = mergeField(best, c, "org", func() bool { return c.Organization != "" })
	}

	best.Phones = nil
	best.Emails = nil
	phoneSet := make(map[string]bool)
	emailSet := make(map[string]bool)
	for _, c := range cluster {
		for _, p := range c.Phones {
			digits := normalizePhoneDigits(p)
			if digits != "" && !phoneSet[digits] {
				phoneSet[digits] = true
				best.Phones = append(best.Phones, p)
			}
		}
		for _, e := range c.Emails {
			ne := normalizeEmail(e)
			if ne != "" && !emailSet[ne] {
				emailSet[ne] = true
				best.Emails = append(best.Emails, e)
			}
		}
	}

	reasons = append(reasons, "merged_duplicate_cluster")
	confidence := 1.0 - (1.0 / float64(len(cluster)+1))

	return model.MergedContact{
		Contact:     best,
		MergedFrom:  from,
		Confidence:  math.Round(confidence*100) / 100,
		MergeReason: reasons,
	}
}

func mergeField(best, c model.Contact, field string, cond func() bool) model.Contact {
	if !cond() {
		return best
	}
	switch field {
	case "name":
		if best.Name == "" {
			best.Name = c.Name
		}
	case "org":
		if best.Organization == "" {
			best.Organization = c.Organization
		}
	}
	return best
}

func jaroWinklerSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	len1 := len(s1)
	len2 := len(s2)
	if len1 == 0 || len2 == 0 {
		return 0.0
	}

	matchDistance := max(len1, len2)/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	s1Matches := make([]bool, len1)
	s2Matches := make([]bool, len2)

	var matches float64
	var transpositions float64

	for i := range len1 {
		start := max(0, i-matchDistance)
		end := min(i+matchDistance+1, len2)

		for j := start; j < end; j++ {
			if s2Matches[j] {
				continue
			}
			if s1[i] != s2[j] {
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

	prefixScale := 0.1
	return jaro + float64(prefix)*prefixScale*(1.0-jaro)
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
