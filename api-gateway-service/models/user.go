package models

type User struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	HashedPassword string `json:"hashed_password"`
	Role           string `json:"role"`
}
