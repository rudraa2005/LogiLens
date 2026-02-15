package services

import (
	"context"
	"errors"

	"github.com/rudraa2005/LogiLens/api-gateway-service/auth"
	"github.com/rudraa2005/LogiLens/api-gateway-service/models"
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *models.User) (string, error)
	GetUserById(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
}

type AuthService struct {
	userRepo UserRepository
}

func NewAuthService(userRepo UserRepository) *AuthService {
	return &AuthService{
		userRepo: userRepo,
	}
}

func (a *AuthService) Signup(ctx context.Context, email string, password string, role string, name string) error {
	_, err := a.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return errors.New("User already exists")
	}

	hashPassword, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	role = "USER"

	user := &models.User{
		Email:          email,
		HashedPassword: hashPassword,
		Name:           name,
		Role:           role,
	}

	_, err = a.userRepo.CreateUser(ctx, user)
	if err != nil {
		return err
	}

	return nil

}

func (a *AuthService) Login(ctx context.Context, email string, password string) (*models.User, string, error) {

	user, err := a.userRepo.GetByEmail(ctx, email)
	if err == nil {
		if !auth.ComparePassword(password, user.HashedPassword) {
			return nil, "", errors.New("Incorrect Password")
		}

		token, err := auth.CreateToken(user.Id, email, user.Role)
		if err != nil {
			return nil, "", err
		}

		return user, token, nil
	}

	return nil, "", errors.New("User Not Found. Signup first.")
}
