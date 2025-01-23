package domain

import "time"

type BetaUser struct {
	ID           uint      `json:"id" form:"id" gorm:"primary_key"`
	Email        string    `json:"email" form:"email" gorm:"unique;not null"`
	IsSubscribed bool      `json:"is_subscribed" form:"subscribed"`
	CreatedAt    time.Time `json:"created_at"`
}
