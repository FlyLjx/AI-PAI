package tasks

type Stats struct {
	Total       int `json:"total"`
	Queued      int `json:"queued"`
	Pending     int `json:"pending"`
	Processing  int `json:"processing"`
	Success     int `json:"success"`
	Failed      int `json:"failed"`
	Canceled    int `json:"canceled"`
	TotalImages int `json:"totalImages"`
}
