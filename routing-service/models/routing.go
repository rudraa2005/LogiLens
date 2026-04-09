package models

import "time"

type Node struct {
	NodeID    string  `json:"node_id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Type      string  `json:"type"`
}

type Edge struct {
	ID       string  `json:"id"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	ModeID   string  `json:"mode_id"`
	Distance float64 `json:"distance"`
	Time     float64 `json:"time"`
	Cost     float64 `json:"cost"`

	Geometry []LatLng `json:"geometry"`
}

type TransportMode struct {
	TransportID string  `json:"transport_id"`
	Name        string  `json:"name"`
	AvgSpeed    float64 `json:"avg_speed"`
	CostPerKm   float64 `json:"cost_per_km"`
	Capacity    float64 `json:"capacity"`
}

type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Route struct {
	ID                string `json:"id"`
	SourceNodeID      string `json:"source_node_id"`
	DestinationNodeID string `json:"destination_node_id"`

	TotalDistance float64 `json:"total_distance"`
	TotalTime     float64 `json:"total_time"`
	TotalCost     float64 `json:"total_cost"`

	CreatedAt time.Time `json:"created_at"`
}

type RouteStep struct {
	ID      string `json:"id"`
	RouteID string `json:"route_id"`

	FromNodeID string `json:"from_node_id"`
	ToNodeID   string `json:"to_node_id"`

	EdgeID string `json:"edge_id"`
	ModeID string `json:"mode_id"`

	Sequence int `json:"sequence"`

	Distance float64  `json:"distance"`
	Time     float64  `json:"time"`
	Cost     float64  `json:"cost"`
	Geometry []LatLng `json:"geometry"`
}
