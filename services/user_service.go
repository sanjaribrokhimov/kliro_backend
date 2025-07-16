package services

import (
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type UserService struct {
	DB  *gorm.DB
	RDB *redis.Client
}

func NewUserService(db *gorm.DB, rdb *redis.Client) *UserService {
	return &UserService{DB: db, RDB: rdb}
}

// Здесь будут методы бизнес-логики
