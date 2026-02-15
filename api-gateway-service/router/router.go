package router

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/rudraa2005/LogiLens/api-gateway-service/handlers"
)

func NewRouter(ah *handlers.AuthHandler, sh *handlers.ShipmentHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/signup", ah.Signup)
		r.Post("/login", ah.Login)

		r.Group(func(r chi.Router) {
			r.Post("/shipment/create", sh.CreateShipment)
		})
	})
	return r
}
