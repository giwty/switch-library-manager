package db

// Progress update message
type ProgressUpdate struct {
	Curr    int    `json:"curr"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

// Progess updater interface
type ProgressUpdater interface {
	UpdateProgress(curr int, total int, message string)
}
