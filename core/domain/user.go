package domain

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var phoneRegex = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

const maxURLLength = 2048

type UserProfile struct {
	ID            uuid.UUID
	Name          string
	Fullname      string
	Phone         *string
	AvatarURL     *string
	PhotoPostedAt *time.Time
	CreatedAt     time.Time
}

func NewUserProfile(name, fullname string, phone, avatarURL *string) UserProfile {
	var photoPostedAt *time.Time
	if avatarURL != nil && *avatarURL != "" {
		now := time.Now().UTC()
		photoPostedAt = &now
	}

	return UserProfile{
		ID:            uuid.New(),
		Name:          name,
		Fullname:      fullname,
		Phone:         phone,
		AvatarURL:     avatarURL,
		PhotoPostedAt: photoPostedAt,
		CreatedAt:     time.Now().UTC(),
	}
}

func (u *UserProfile) Validate() error {
	if strings.TrimSpace(u.Name) == "" {
		return errors.New("name can't be empty")
	}
	if strings.TrimSpace(u.Fullname) == "" {
		return errors.New("fullname can't be empty")
	}

	// Безопасная проверка телефона без nil-dereference
	if u.Phone != nil && *u.Phone != "" {
		if !phoneRegex.MatchString(*u.Phone) {
			return errors.New("invalid phone number format")
		}
	}

	if u.AvatarURL == nil || *u.AvatarURL == "" {
		return nil
	}

	rawURL := *u.AvatarURL

	if len(rawURL) > maxURLLength {
		return fmt.Errorf("avatar_url is too long (max %d characters)", maxURLLength)
	}

	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("invalid avatar_url format: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return errors.New("avatar_url must use http or https scheme")
	}

	if parsed.Host == "" {
		return errors.New("avatar_url must include a valid host")
	}

	return nil
}

type UserUpdateRequest struct {
	ID        string
	Name      string
	FullName  string
	Phone     *string
	AvatarURL *string
}

func (u *UserProfile) ApplyPatch(params UserUpdateRequest) error {
	updated := *u

	if params.Name != "" {
		updated.Name = params.Name
	}
	if params.FullName != "" {
		updated.Fullname = params.FullName
	}

	if params.Phone != nil {
		if *params.Phone == "" {
			updated.Phone = nil
		} else {
			updated.Phone = params.Phone
		}
	}

	if params.AvatarURL != nil {
		newAvatar := *params.AvatarURL
		oldAvatar := ""
		if u.AvatarURL != nil {
			oldAvatar = *u.AvatarURL
		}

		if newAvatar != oldAvatar {
			if newAvatar != "" {
				now := time.Now().UTC()
				updated.AvatarURL = params.AvatarURL
				updated.PhotoPostedAt = &now
			} else {
				updated.AvatarURL = nil
				updated.PhotoPostedAt = nil
			}
		}
	}

	if err := updated.Validate(); err != nil {
		return fmt.Errorf("invalid profile patch: %w", err)
	}

	*u = updated
	return nil
}
