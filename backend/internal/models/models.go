package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Base model using UUIDs
type Base struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// User represents a registered account
type User struct {
	Base
	Username     string        `gorm:"uniqueIndex;type:varchar(50);not null" json:"username"`
	Email        string        `gorm:"uniqueIndex;type:varchar(100);not null" json:"email"`
	Phone        string        `gorm:"uniqueIndex;type:varchar(20)" json:"phone,omitempty"`
	PasswordHash string        `gorm:"type:varchar(255);not null" json:"-"`
	DisplayName  string        `gorm:"type:varchar(100)" json:"display_name"`
	AvatarURL    string        `gorm:"type:varchar(255)" json:"avatar_url,omitempty"`
	Bio          string        `gorm:"type:varchar(255)" json:"bio,omitempty"`
	Status       string        `gorm:"type:varchar(50);default:'offline'" json:"status"`
	LastSeen     time.Time     `json:"last_seen"`
	OTPSecret    string        `gorm:"type:varchar(100)" json:"-"`
	OTPEnabled   bool          `gorm:"default:false" json:"otp_enabled"`
	RecoveryCode string        `gorm:"type:varchar(255)" json:"-"`
	Devices      []Device      `gorm:"foreignKey:UserID" json:"devices,omitempty"`
	PrekeyBundle *PrekeyBundle `gorm:"foreignKey:UserID" json:"prekey_bundle,omitempty"`
}

// Device represents a user's logged-in client device
type Device struct {
	Base
	UserID        uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`
	DeviceToken   string    `gorm:"type:varchar(255)" json:"device_token,omitempty"`
	Platform      string    `gorm:"type:varchar(50);not null" json:"platform"` // ios, android, web, desktop
	DeviceName    string    `gorm:"type:varchar(100)" json:"device_name"`
	IdentityKey   string    `gorm:"type:text;not null" json:"identity_key"` // Base64 X25519 identity key
	RefreshToken  string    `gorm:"type:varchar(255);index" json:"-"`
	TokenExpiry   time.Time `json:"-"`
	LastActiveAt  time.Time `json:"last_active_at"`
}

// PrekeyBundle contains public key components for Signal-like X3DH E2EE
type PrekeyBundle struct {
	Base
	UserID         uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	IdentityKey    string    `gorm:"type:text;not null" json:"identity_key"` // Base64
	SignedPrekey   string    `gorm:"type:text;not null" json:"signed_prekey"` // Base64
	SignedPrekeyID uint32    `gorm:"not null" json:"signed_prekey_id"`
	Signature      string    `gorm:"type:text;not null" json:"signature"` // Base64 signature of SPK using IdentityKey
}

// OneTimePrekey represents single-use prekeys for forward secrecy
type OneTimePrekey struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index:idx_user_key,priority:1;not null" json:"user_id"`
	KeyID     uint32    `gorm:"index:idx_user_key,priority:2;not null" json:"key_id"`
	KeyVal    string    `gorm:"type:text;not null" json:"key_val"` // Base64
	Used      bool      `gorm:"default:false;index" json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// Group represents a multi-user chat room or channel
type Group struct {
	Base
	Name        string        `gorm:"type:varchar(100);not null" json:"name"`
	Description string        `gorm:"type:varchar(255)" json:"description,omitempty"`
	AvatarURL   string        `gorm:"type:varchar(255)" json:"avatar_url,omitempty"`
	OwnerID     uuid.UUID     `gorm:"type:uuid;not null" json:"owner_id"`
	Type        string        `gorm:"type:varchar(50);default:'group'" json:"type"` // group, channel
	InviteCode  string        `gorm:"type:varchar(50);uniqueIndex" json:"invite_code,omitempty"`
	Members     []GroupMember `gorm:"foreignKey:GroupID" json:"members,omitempty"`
}

// GroupMember links a user to a group with roles and permissions
type GroupMember struct {
	GroupID  uuid.UUID `gorm:"type:uuid;primaryKey;index" json:"group_id"`
	UserID   uuid.UUID `gorm:"type:uuid;primaryKey;index" json:"user_id"`
	Role     string    `gorm:"type:varchar(50);default:'member'" json:"role"` // owner, admin, moderator, member
	JoinedAt time.Time `json:"joined_at"`
	User     *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// JSONB is a custom type to handle JSON fields in GORM PostgreSQL
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	var result map[string]interface{}
	err := json.Unmarshal(bytes, &result)
	*j = result
	return err
}

// Message represents an encrypted communication unit
type Message struct {
	Base
	SenderID           uuid.UUID  `gorm:"type:uuid;index;not null" json:"sender_id"`
	ReceiverID         *uuid.UUID `gorm:"type:uuid;index" json:"receiver_id,omitempty"` // Nullable for group messages
	GroupID            *uuid.UUID `gorm:"type:uuid;index" json:"group_id,omitempty"`    // Nullable for direct messages
	ContentType        string     `gorm:"type:varchar(50);default:'text'" json:"content_type"` // text, file, photo, video, audio, location, poll
	EncryptedPayload   string     `gorm:"type:text;not null" json:"encrypted_payload"` // Double Ratchet ciphertext
	EphemeralKey       string     `gorm:"type:text" json:"ephemeral_key,omitempty"` // X25519 public key of current ratchet step
	Counter            uint32     `json:"counter"` // ratchet message count
	SenderRatchetKey   string     `gorm:"type:text" json:"sender_ratchet_key,omitempty"` // current sender ratchet key
	Status             string     `gorm:"type:varchar(50);default:'sent'" json:"status"` // sent, delivered, read
	ExpiresAt          *time.Time `gorm:"index" json:"expires_at,omitempty"` // For self-destruct/disappearing messages
	Metadata           JSONB      `gorm:"type:jsonb" json:"metadata,omitempty"` // For file sizes, thumbnails, voice duration, etc.
}

// MessageReaction allows users to react to messages with emojis
type MessageReaction struct {
	MessageID uuid.UUID `gorm:"type:uuid;primaryKey" json:"message_id"`
	UserID    uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	Reaction  string    `gorm:"type:varchar(50)" json:"reaction"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditLog tracks administrative actions
type AuditLog struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	Action    string    `gorm:"type:varchar(100);not null" json:"action"`
	Details   string    `gorm:"type:text" json:"details"`
	IPAddress string    `gorm:"type:varchar(45)" json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
}
