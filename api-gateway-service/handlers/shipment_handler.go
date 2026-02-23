package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/rudraa2005/LogiLens/api-gateway-service/middleware"
	"github.com/rudraa2005/LogiLens/api-gateway-service/services"
)

type ShipmentHandler struct {
	ShipmentService *services.ShipmentService
}

type ShipmentRequest struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}

func NewShipmentHandler(ShipmentService *services.ShipmentService) *ShipmentHandler {
	return &ShipmentHandler{
		ShipmentService: ShipmentService,
	}
}

func (sh *ShipmentHandler) CreateShipment(w http.ResponseWriter, r *http.Request) {
	var req ShipmentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	claims, err := middleware.GetUser(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
	userID := claims.UserID
	log.Println("UserID being sent:", userID)
	shipment, err := sh.ShipmentService.CreateShipment(
		r.Context(),
		req.Origin,
		req.Destination,
		userID,
	)

	if err != nil {
		log.Println("gRPC error:", err)
		http.Error(w, "Shipment Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shipment)
}

func (sh *ShipmentHandler) GetShipment(w http.ResponseWriter, r *http.Request) {
	var req ShipmentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "INVALID REQUEST", http.StatusBadRequest)
		return
	}
	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	shipment, err := sh.ShipmentService.GetShipment(r.Context(), userID)
	if err != nil {
		http.Error(w, "Service Error", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shipment)
}

func (sh *ShipmentHandler) MarkInTransit(w http.ResponseWriter, r *http.Request) {

	shipmentID := chi.URLParam(r, "shipment_id")
	var req ShipmentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}
	shipment, err := sh.ShipmentService.MarkInTransit(r.Context(), shipmentID)
	if err != nil {
		http.Error(w, "Service Error", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shipment)
}

func (sh *ShipmentHandler) MarkDelivered(w http.ResponseWriter, r *http.Request) {

	shipmentID := chi.URLParam(r, "shipment_id")
	var req ShipmentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}
	shipment, err := sh.ShipmentService.MarkDelivered(r.Context(), shipmentID)
	if err != nil {
		http.Error(w, "Service Error", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shipment)
}
