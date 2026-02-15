package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rudraa2005/LogiLens/api-gateway-service/services"
)

type AuthHandler struct {
	AuthService *services.AuthService
}

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func NewAuthHandler(authService *services.AuthService) *AuthHandler {
	return &AuthHandler{
		AuthService: authService,
	}
}

func (ah *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	err := ah.AuthService.Signup(r.Context(), req.Email, req.Password, req.Role, req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User Created Successfully"})
}

func (ah *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "INVALID REQUEST", http.StatusBadRequest)
		return
	}

	user, token, err := ah.AuthService.Login(r.Context(), req.Email, req.Password)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(LoginResponse{Token: token, ID: user.Id, Name: user.Name, Email: user.Email, Role: user.Role})
}
