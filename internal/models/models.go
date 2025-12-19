package models

type Player struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Surname  string  `json:"surname"`
	RegNum   string  `json:"reg_num"`
	Handicap float64 `json:"handicap"`
}

type Flight struct {
	ID           int    `json:"id"`
	Token        string `json:"token"`
	Name         string `json:"name"`
	StartingHole int    `json:"starting_hole"`
}

type FlightPlayer struct {
	FlightID int `json:"flight_id"`
	PlayerID int `json:"player_id"`
}

type Score struct {
	ID         int `json:"id"`
	PlayerID   int `json:"player_id"`
	HoleNumber int `json:"hole_number"`
	Strokes    int `json:"strokes"`
}
