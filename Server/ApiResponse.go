package Server

type APIResponse struct {
	Time struct {
		UpdatedISO string `json:"updatedISO"`
	} `json:"time"`
	Bpi map[string]struct {
		RateFloat float64 `json:"rate_float"`
	} `json:"bpi"`
}
