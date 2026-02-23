package models

type Shipment struct {
	UserID      string `json:"user_id"`
	ShipmentID  string `json:"shipment_id"`
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
	Status      string `json:"status"`
}
