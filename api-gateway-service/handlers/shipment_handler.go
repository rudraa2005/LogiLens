package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/rudraa2005/LogiLens/api-gateway-service/services"
)

type ShipmentHandler struct {
	ShipmentService services.ShipmentService
}

type ShipmentRequest struct {
	Origin      string `json:"origin"`
	Destination string `json:"destination"`
}

func NewShipmentHandler(ShipmentService services.ShipmentService) *ShipmentHandler {
	return &ShipmentHandler{
		ShipmentService: ShipmentService,
	}
}

func (sh *ShipmentHandler) CreateShipment(w http.ResponseWriter, r *http.Request) {
	chi.URLParam(r, "user_id")
	var req ShipmentRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Request", http.StatusBadRequest)
		return
	}

	userID, ok := r.Context().Value("user_id").(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	shipment, err := sh.ShipmentService.CreateShipment(r.Context(), req.Origin, req.Destination, userID)
	if err != nil {
		http.Error(w, "Shipment Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shipment)
}
