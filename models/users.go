package models

import (
	"errors"

	"github.com/alexander-emelyanenko/go-web-server/hash"
	"github.com/alexander-emelyanenko/go-web-server/rand"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"golang.org/x/crypto/bcrypt"
)

const hmacSecretKey = "secret-hmac-key"

var (
	userPwPepper       = "secret-random-string"
	ErrInvalidPassword = errors.New("models: incorrect password provided")
	ErrNotFound        = errors.New("models: resource not found")
	ErrInvalidID       = errors.New("models: ID provided was invalid")
)

type User struct {
	gorm.Model
	Name         string
	Email        string `gorm:"not null;unique_index"`
	Password     string `gorm:"-"`
	PasswordHash string `gorm:"not null"`
	Remember     string `gorm:"-"`
	RememberHash string `gorm:"not null;unique_index"`
}

type UserService struct {
	db   *gorm.DB
	hmac hash.HMAC
}

func (us *UserService) Authenticate(email, password string) (*User, error) {
	foundUser, err := us.ByEmail(email)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(foundUser.PasswordHash), []byte(password+userPwPepper))
	switch err {
	case nil:
		return foundUser, nil
	case bcrypt.ErrMismatchedHashAndPassword:
		return nil, ErrInvalidPassword
	default:
		return nil, err
	}
}

func (us *UserService) Create(user *User) error {
	pwBytes := []byte(user.Password + userPwPepper)
	hashedBytes, err := bcrypt.GenerateFromPassword(pwBytes, bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedBytes)
	user.Password = ""

	if user.Remember == "" {
		token, err := rand.RememberToken()
		if err != nil {
			return err
		}
		user.Remember = token
	}

	user.RememberHash = us.hmac.Hash(user.Remember)

	return us.db.Create(user).Error
}

func (us *UserService) Update(user *User) error {
	if user.Remember != "" {
		user.RememberHash = us.hmac.Hash(user.Remember)
	}
	return us.db.Save(user).Error
}

func (us *UserService) Delete(id uint) error {
	if id == 0 {
		return ErrInvalidID
	}

	user := User{Model: gorm.Model{ID: id}}
	return us.db.Delete(&user).Error
}

func (us *UserService) Close() error {
	return us.db.Close()
}

func (us *UserService) ByID(id uint) (*User, error) {
	var user User
	db := us.db.Where("id = ?", id)
	err := first(db, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (us *UserService) ByEmail(email string) (*User, error) {
	var user User
	db := us.db.Where("email = ?", email)
	err := first(db, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (us *UserService) AutoMigrate() error {
	if err := us.db.AutoMigrate(&User{}).Error; err != nil {
		return err
	}
	return nil
}

func (us *UserService) DestructiveReset() error {
	if err := us.db.DropTableIfExists(&User{}).Error; err != nil {
		return err
	}
	return us.AutoMigrate()
}

func NewUserService(connectionInfo string) (*UserService, error) {
	db, err := gorm.Open("postgres", connectionInfo)
	if err != nil {
		return nil, err
	}
	db.LogMode(true)

	hmac := hash.NewHMAC(hmacSecretKey)

	return &UserService{
		db:   db,
		hmac: hmac,
	}, nil
}

func first(db *gorm.DB, dist interface{}) error {
	err := db.First(dist).Error
	if err == gorm.ErrRecordNotFound {
		return ErrNotFound
	}
	return err
}
