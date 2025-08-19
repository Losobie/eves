package eveapi

import "time"

type Character struct {
	AllianceID     *int      `json:"alliance_id,omitempty"`
	Birthday       time.Time `json:"birthday"`
	BloodlineID    int       `json:"bloodline_id"`
	CorporationID  int       `json:"corporation_id"`
	Description    *string   `json:"description,omitempty"`
	Gender         string    `json:"gender"`
	Name           string    `json:"name"`
	RaceID         int       `json:"race_id"`
	SecurityStatus *float64  `json:"security_status,omitempty"`
	Title          *string   `json:"title,omitempty"`
}
