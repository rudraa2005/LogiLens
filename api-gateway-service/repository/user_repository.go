package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rudraa2005/LogiLens/api-gateway-service/models"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (u *UserRepository) CreateUser(ctx context.Context, user *models.User) (string, error) {

	query := `
		INSERT INTO users (name, email, hashed_password,role)
		VALUES ($1,$2,$3,$4)
		RETURNING id
	`

	err := u.db.QueryRow(context.Background(), query, user.Name, user.Email, user.HashedPassword, user.Role).Scan(&user.Id)

	return user.Id, err
}

func (u *UserRepository) GetUserById(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT email,role, name
		FROM users
		WHERE id = $1
	`
	var user models.User
	err := u.db.QueryRow(ctx, query, id).Scan(&user.Email, &user.Role, &user.Name)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (u *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User

	query := `
		SELECT id,name,role, hashed_password
		FROM users 
		WHERE email = $1
	`
	err := u.db.QueryRow(ctx, query, email).Scan(&user.Id, &user.Name, &user.Role, &user.HashedPassword)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
