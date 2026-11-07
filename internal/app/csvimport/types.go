package csvimport

type CutRow struct {
	Row           int    `json:"row"`
	Name          string `json:"name"`
	DateTime      string `json:"datetime"`
	SecondsBefore int    `json:"secondsBefore"`
	SecondsAfter  int    `json:"secondsAfter"`
}

type Issue struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"`
}

type ParseResult struct {
	Rows   []CutRow `json:"rows"`
	Issues []Issue  `json:"issues"`
}
