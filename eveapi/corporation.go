package eveapi

type Corporation struct {
	AllianceID    int     `json:"alliance_id"`
	CeoID         int     `json:"ceo_id"`
	CreatorID     int     `json:"creator_id"`
	DateFounded   string  `json:"date_founded"`
	Description   string  `json:"description"`
	HomeStationID int     `json:"home_station_id"`
	MemberCount   int     `json:"member_count"`
	Name          string  `json:"name"`
	Shares        int     `json:"shares"`
	TaxRate       float64 `json:"tax_rate"`
	Ticker        string  `json:"ticker"`
	URL           string  `json:"url"`
}
