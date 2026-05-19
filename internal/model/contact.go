package model

type SourceInfo struct {
	File string
	Row  int
}

type Contact struct {
	ID           string
	Name         string
	Phones       []string
	Emails       []string
	Organization string
	Metadata     map[string]string
	Source       SourceInfo
}

type NormalizedName struct {
	FullName     string
	FirstName    string
	LastName     string
	Organization string
	Normalized   bool
}

type NormalizedPhone struct {
	Raw        string
	Normalized string
	Valid      bool
	Country    string
}

type NormalizedEmail struct {
	Raw        string
	Normalized string
	Valid      bool
}

type MergedContact struct {
	Contact
	MergedFrom  []string
	Confidence  float64
	MergeReason []string
}

type DedupeReport struct {
	TotalInput      int
	DuplicatesFound int
	ClustersFormed  int
}

type DedupeResult struct {
	Clusters [][]Contact
	Merged   []MergedContact
	Report   DedupeReport
}

type ParseError struct {
	Row     int
	Field   string
	Message string
}
