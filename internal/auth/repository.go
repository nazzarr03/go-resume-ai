package auth

import (
	"github.com/nazzarr03/go-resume-ai/db/entity"
	"gorm.io/gorm"
)

type AuthRepository struct {
	DB *gorm.DB
}

func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{DB: db}
}

func (a *AuthRepository) Login(req *LoginRequest) (*entity.User, error) {
	var user entity.User
	err := a.DB.
		Model(&entity.User{}).
		Preload("UserType").
		Where("email = ?", req.Email).
		First(&user).Error

	return &user, err
}
