package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"image2/backend/internal/config"
	"image2/backend/internal/database"
	"image2/backend/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	username := flag.String("username", "admin", "admin username")
	password := flag.String("password", "", "admin password")
	email := flag.String("email", "admin.local@example.com", "admin email")
	dbPath := flag.String("db", "", "sqlite database path; leave empty to use backend .env")
	flag.Parse()

	if err := run(*username, *password, *email, *dbPath); err != nil {
		log.Fatalf("admin user setup failed: %v", err)
	}
}

func run(username, password, email, dbPath string) error {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	email = strings.ToLower(strings.TrimSpace(email))

	if username == "" {
		return fmt.Errorf("username is required")
	}
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if email == "" {
		return fmt.Errorf("email is required")
	}

	cfg := config.Load()
	if strings.TrimSpace(dbPath) != "" {
		cfg.MySQLDSN = ""
		cfg.SQLitePath = strings.TrimSpace(dbPath)
	}

	db, err := database.Open(cfg)
	if err != nil {
		return err
	}
	if err := database.AutoMigrate(db); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	var saved model.User
	err = db.Transaction(func(tx *gorm.DB) error {
		user, found, err := findUserForAdminSetup(tx, username, email)
		if err != nil {
			return err
		}

		now := time.Now()
		verifiedAt := user.EmailVerifiedAt
		if verifiedAt == nil {
			verifiedAt = &now
		}

		if !found {
			user = model.User{
				Username:        username,
				Email:           email,
				PasswordHash:    string(hash),
				EmailVerifiedAt: verifiedAt,
				Status:          model.UserStatusNormal,
				Role:            model.UserRoleAdmin,
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			saved = user
			return nil
		}

		updates := map[string]any{
			"username":          username,
			"password_hash":     string(hash),
			"email_verified_at": verifiedAt,
			"status":            model.UserStatusNormal,
			"role":              model.UserRoleAdmin,
		}

		if canUseEmail(tx, email, user.ID) {
			updates["email"] = email
			user.Email = email
		}

		if err := tx.Model(&model.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
			return err
		}

		user.Username = username
		user.PasswordHash = string(hash)
		user.EmailVerifiedAt = verifiedAt
		user.Status = model.UserStatusNormal
		user.Role = model.UserRoleAdmin
		saved = user
		return nil
	})
	if err != nil {
		return err
	}

	log.Printf("admin user is ready: id=%d username=%s email=%s role=%s status=%d", saved.ID, saved.Username, saved.Email, saved.Role, saved.Status)
	return nil
}

func findUserForAdminSetup(tx *gorm.DB, username, email string) (model.User, bool, error) {
	var user model.User
	err := tx.Where("username = ?", username).First(&user).Error
	if err == nil {
		return user, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, false, err
	}

	err = tx.Where("lower(email) = ?", email).First(&user).Error
	if err == nil {
		return user, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, false, nil
	}
	return model.User{}, false, err
}

func canUseEmail(tx *gorm.DB, email string, currentUserID uint64) bool {
	var count int64
	err := tx.Model(&model.User{}).
		Where("lower(email) = ? AND id <> ?", email, currentUserID).
		Count(&count).Error
	return err == nil && count == 0
}
