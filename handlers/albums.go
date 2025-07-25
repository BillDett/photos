package handlers

import (
	"net/http"
	"photo-library-server/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AlbumHandler handles album-related HTTP requests
type AlbumHandler struct {
	db *gorm.DB
}

// NewAlbumHandler creates a new album handler
func NewAlbumHandler(db *gorm.DB) *AlbumHandler {
	return &AlbumHandler{db: db}
}

// CreateAlbum creates a new album
func (h *AlbumHandler) CreateAlbum(c *gin.Context) {
	var req struct {
		Name        string    `json:"name" binding:"required,min=1,max=100"`
		Description string    `json:"description" binding:"max=500"`
		LibraryID   uuid.UUID `json:"library_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": processValidationError(err)})
		return
	}

	// Verify library exists
	var library models.Library
	if err := h.db.First(&library, req.LibraryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify library"})
		return
	}

	album := models.Album{
		Name:        req.Name,
		Description: req.Description,
		LibraryID:   req.LibraryID,
	}

	if err := h.db.Create(&album).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create album"})
		return
	}

	// Load the library for response
	h.db.Preload("Library").First(&album, album.ID)

	c.JSON(http.StatusCreated, album)
}

// GetAlbums returns albums, optionally filtered by library
func (h *AlbumHandler) GetAlbums(c *gin.Context) {
	var albums []models.Album

	query := h.db.Model(&models.Album{})

	// Filter by library if specified
	if libraryID := c.Query("library_id"); libraryID != "" {
		id, err := uuid.Parse(libraryID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
			return
		}
		query = query.Where("library_id = ?", id)
	}

	// Optional: include related data
	if c.Query("include_library") == "true" {
		query = query.Preload("Library")
	}
	if c.Query("include_photos") == "true" {
		query = query.Preload("Photos")
	}

	if err := query.Find(&albums).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch albums"})
		return
	}

	c.JSON(http.StatusOK, albums)
}

// GetAlbum returns a specific album by ID
func (h *AlbumHandler) GetAlbum(c *gin.Context) {
	albumID := c.Param("id")

	id, err := uuid.Parse(albumID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid album ID"})
		return
	}

	var album models.Album
	query := h.db.Model(&models.Album{})

	// Optional: include related data
	if c.Query("include_library") == "true" {
		query = query.Preload("Library")
	}
	if c.Query("include_photos") == "true" {
		query = query.Preload("Photos").Preload("Photos.Tags")
	}

	if err := query.First(&album, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch album"})
		return
	}

	c.JSON(http.StatusOK, album)
}

// UpdateAlbum updates an album
func (h *AlbumHandler) UpdateAlbum(c *gin.Context) {
	albumID := c.Param("id")

	id, err := uuid.Parse(albumID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid album ID"})
		return
	}

	var req struct {
		Name        *string `json:"name,omitempty" binding:"omitempty,min=1,max=100"`
		Description *string `json:"description,omitempty" binding:"omitempty,max=500"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": processValidationError(err)})
		return
	}

	var album models.Album
	if err := h.db.First(&album, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch album"})
		return
	}

	// Update only provided fields
	if req.Name != nil {
		album.Name = *req.Name
	}
	if req.Description != nil {
		album.Description = *req.Description
	}

	if err := h.db.Save(&album).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update album"})
		return
	}

	c.JSON(http.StatusOK, album)
}

// DeleteAlbum deletes an album
func (h *AlbumHandler) DeleteAlbum(c *gin.Context) {
	albumID := c.Param("id")

	id, err := uuid.Parse(albumID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid album ID"})
		return
	}

	var album models.Album
	if err := h.db.First(&album, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch album"})
		return
	}

	// Use transaction to clean up album_photos relationships
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete album_photos relationships
	if err := tx.Where("album_id = ?", id).Delete(&models.AlbumPhoto{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove photos from album"})
		return
	}

	// Delete the album
	if err := tx.Delete(&album).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete album"})
		return
	}

	tx.Commit()
	c.JSON(http.StatusOK, gin.H{"message": "Album deleted successfully"})
}

// AddPhotoToAlbum adds a photo to an album
func (h *AlbumHandler) AddPhotoToAlbum(c *gin.Context) {
	albumID := c.Param("id")

	id, err := uuid.Parse(albumID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid album ID"})
		return
	}

	var req struct {
		PhotoID uuid.UUID `json:"photo_id" binding:"required"`
		Order   int       `json:"order"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": processValidationError(err)})
		return
	}

	// Verify album exists
	var album models.Album
	if err := h.db.First(&album, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Album not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify album"})
		return
	}

	// Verify photo exists and is in the same library
	var photo models.Photo
	if err := h.db.First(&photo, req.PhotoID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify photo"})
		return
	}

	if photo.LibraryID != album.LibraryID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Photo and album must be in the same library"})
		return
	}

	// Check if photo is already in the album
	var existingRelation models.AlbumPhoto
	if err := h.db.Where("album_id = ? AND photo_id = ?", id, req.PhotoID).First(&existingRelation).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Photo is already in this album"})
		return
	}

	albumPhoto := models.AlbumPhoto{
		AlbumID: id,
		PhotoID: req.PhotoID,
		Order:   req.Order,
	}

	if err := h.db.Create(&albumPhoto).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add photo to album"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Photo added to album successfully"})
}

// RemovePhotoFromAlbum removes a photo from an album
func (h *AlbumHandler) RemovePhotoFromAlbum(c *gin.Context) {
	albumID := c.Param("id")
	photoID := c.Param("photo_id")

	albumUUID, err := uuid.Parse(albumID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid album ID"})
		return
	}

	photoUUID, err := uuid.Parse(photoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid photo ID"})
		return
	}

	result := h.db.Where("album_id = ? AND photo_id = ?", albumUUID, photoUUID).Delete(&models.AlbumPhoto{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove photo from album"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found in album"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Photo removed from album successfully"})
}

// UpdatePhotoOrder updates the order of a photo in an album
func (h *AlbumHandler) UpdatePhotoOrder(c *gin.Context) {
	albumID := c.Param("id")
	photoID := c.Param("photo_id")

	albumUUID, err := uuid.Parse(albumID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid album ID"})
		return
	}

	photoUUID, err := uuid.Parse(photoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid photo ID"})
		return
	}

	var req struct {
		Order int `json:"order" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": processValidationError(err)})
		return
	}

	result := h.db.Model(&models.AlbumPhoto{}).
		Where("album_id = ? AND photo_id = ?", albumUUID, photoUUID).
		Update("order", req.Order)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update photo order"})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found in album"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Photo order updated successfully"})
}
