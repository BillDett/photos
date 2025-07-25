package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Library represents a photo library with a unique name
type Library struct {
	ID          uuid.UUID `json:"id" gorm:"type:char(36);primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Description string    `json:"description"`
	Images      string    `json:"images" gorm:"uniqueIndex;not null"` // Filepath where photos are stored
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Albums      []Album   `json:"albums,omitempty" gorm:"foreignKey:LibraryID"`
	Photos      []Photo   `json:"photos,omitempty" gorm:"foreignKey:LibraryID"`
}

// Album represents a photo album within a library
type Album struct {
	ID          uuid.UUID `json:"id" gorm:"type:char(36);primaryKey"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description"`
	LibraryID   uuid.UUID `json:"library_id" gorm:"type:char(36);not null;index"`
	Library     Library   `json:"library,omitempty" gorm:"foreignKey:LibraryID"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Photos      []Photo   `json:"photos,omitempty" gorm:"many2many:album_photos;"`
}

// Photo represents a photo with metadata
type Photo struct {
	ID           uuid.UUID `json:"id" gorm:"type:char(36);primaryKey"`
	Filename     string    `json:"filename" gorm:"not null"`
	OriginalName string    `json:"original_name" gorm:"not null"`
	FilePath     string    `json:"file_path" gorm:"not null"`
	MimeType     string    `json:"mime_type" gorm:"not null"`
	FileSize     int64     `json:"file_size" gorm:"not null"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Rating       *int      `json:"rating" gorm:"check:rating >= 0 AND rating <= 5"` // 0-5, nullable
	LibraryID    uuid.UUID `json:"library_id" gorm:"type:char(36);not null;index"`
	Library      Library   `json:"library,omitempty" gorm:"foreignKey:LibraryID"`
	UploadedAt   time.Time `json:"uploaded_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Tags         []Tag     `json:"tags,omitempty" gorm:"many2many:photo_tags;"`
	Albums       []Album   `json:"albums,omitempty" gorm:"many2many:album_photos;"`
}

// Tag represents a textual tag that can be applied to photos
type Tag struct {
	ID        uuid.UUID `json:"id" gorm:"type:char(36);primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex;not null"`
	Color     string    `json:"color"` // Optional hex color for UI
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Photos    []Photo   `json:"photos,omitempty" gorm:"many2many:photo_tags;"`
}

// PhotoTag represents the many-to-many relationship between photos and tags
type PhotoTag struct {
	PhotoID uuid.UUID `gorm:"type:char(36);primaryKey"`
	TagID   uuid.UUID `gorm:"type:char(36);primaryKey"`
	Photo   Photo     `gorm:"foreignKey:PhotoID"`
	Tag     Tag       `gorm:"foreignKey:TagID"`
}

// AlbumPhoto represents the many-to-many relationship between albums and photos
type AlbumPhoto struct {
	AlbumID uuid.UUID `gorm:"type:char(36);primaryKey"`
	PhotoID uuid.UUID `gorm:"type:char(36);primaryKey"`
	Album   Album     `gorm:"foreignKey:AlbumID"`
	Photo   Photo     `gorm:"foreignKey:PhotoID"`
	Order   int       `gorm:"default:0"` // For ordering photos within an album
}

// BeforeCreate hook to generate UUID before creating records
func (l *Library) BeforeCreate(tx *gorm.DB) (err error) {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return
}

func (a *Album) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return
}

func (p *Photo) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	if p.UploadedAt.IsZero() {
		p.UploadedAt = time.Now()
	}
	return
}

func (t *Tag) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return
}
