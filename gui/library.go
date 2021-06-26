package gui

// Key-value pair for an issue
type Issue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Local library
type LocalLibraryData struct {
	LibraryData []LibraryTemplateData `json:"library_data"`
	Issues      []Issue               `json:"issues"`
	NumFiles    int                   `json:"num_files"`
}

// Local library entry
type LibraryTemplateData struct {
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Dlc     string `json:"dlc"`
	TitleId string `json:"titleId"`
	Path    string `json:"path"`
	Icon    string `json:"icon"`
	Update  int    `json:"update"`
	Region  string `json:"region"`
	Type    string `json:"type"`
}
