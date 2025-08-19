package eveapi

type Alliance struct {
	CreatorCorporationID  int    `json:"creator_corporation_id"`
	CreatorID             int    `json:"creator_id"`
	DateFounded           string `json:"date_founded"`
	ExecutorCorporationID int    `json:"executor_corporation_id"`
	Name                  string `json:"name"`
	Ticker                string `json:"ticker"`
}
