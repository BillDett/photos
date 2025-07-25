package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"photo-library-server/models"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LibraryHandler handles library-related HTTP requests
type LibraryHandler struct {
	db *gorm.DB
}

// NewLibraryHandler creates a new library handler
func NewLibraryHandler(db *gorm.DB) *LibraryHandler {
	return &LibraryHandler{db: db}
}

// Helper functions for directory management
func isValidPath(path string) bool {
	// Basic validation - no empty paths, no absolute paths to system directories
	if path == "" || strings.HasPrefix(path, "/etc") || strings.HasPrefix(path, "/sys") || strings.HasPrefix(path, "/proc") {
		return false
	}
	// Clean the path and check for directory traversal attempts
	cleanPath := filepath.Clean(path)
	return !strings.Contains(cleanPath, "..") && cleanPath != "/" && cleanPath != "."
}

func createDirectoryIfNotExists(path string) error {
	// Create directory with proper permissions
	return os.MkdirAll(path, 0755)
}

func removeDirectoryIfExists(path string) error {
	// Only remove if it exists and is a directory
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return os.RemoveAll(path)
	}
	return nil
}

// CreateLibrary creates a new library
func (h *LibraryHandler) CreateLibrary(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required,min=1,max=100"`
		Description string `json:"description" binding:"max=500"`
		Images      string `json:"images" binding:"required,min=1,max=500"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": processValidationError(err)})
		return
	}

	// Validate the images path format (basic validation)
	if !isValidPath(req.Images) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid images path format"})
		return
	}

	// Check if library with same name already exists
	var existingLibrary models.Library
	if err := h.db.Where("name = ?", req.Name).First(&existingLibrary).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Library with this name already exists"})
		return
	}

	// Check if library with same images path already exists
	if err := h.db.Where("images = ?", req.Images).First(&existingLibrary).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Library with this images path already exists"})
		return
	}

	library := models.Library{
		Name:        req.Name,
		Description: req.Description,
		Images:      req.Images,
	}

	// Create the images directory
	if err := createDirectoryIfNotExists(req.Images); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create images directory"})
		return
	}

	if err := h.db.Create(&library).Error; err != nil {
		// Cleanup directory if database creation fails
		removeDirectoryIfExists(req.Images)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create library"})
		return
	}

	c.JSON(http.StatusCreated, library)
}

// GetLibraries returns all libraries
func (h *LibraryHandler) GetLibraries(c *gin.Context) {
	var libraries []models.Library

	query := h.db.Model(&models.Library{})

	// Optional: include counts
	if c.Query("include_counts") == "true" {
		query = query.Preload("Albums").Preload("Photos")
	}

	if err := query.Find(&libraries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch libraries"})
		return
	}

	c.JSON(http.StatusOK, libraries)
}

// GetLibrary returns a specific library by ID
func (h *LibraryHandler) GetLibrary(c *gin.Context) {
	libraryID := c.Param("id")

	id, err := uuid.Parse(libraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}

	var library models.Library
	query := h.db.Model(&models.Library{})

	// Optional: include related data
	if c.Query("include_albums") == "true" {
		query = query.Preload("Albums")
	}
	if c.Query("include_photos") == "true" {
		query = query.Preload("Photos")
	}

	if err := query.First(&library, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch library"})
		return
	}

	c.JSON(http.StatusOK, library)
}

// UpdateLibrary updates a library
func (h *LibraryHandler) UpdateLibrary(c *gin.Context) {
	libraryID := c.Param("id")

	id, err := uuid.Parse(libraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}

	var req struct {
		Name        *string `json:"name,omitempty" binding:"omitempty,min=1,max=100"`
		Description *string `json:"description,omitempty" binding:"omitempty,max=500"`
		Images      *string `json:"images,omitempty" binding:"omitempty,min=1,max=500"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": processValidationError(err)})
		return
	}

	// Validate the images path format if provided
	if req.Images != nil && !isValidPath(*req.Images) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid images path format"})
		return
	}

	var library models.Library
	if err := h.db.First(&library, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch library"})
		return
	}

	// Check if another library with same name exists (only if name is being updated)
	if req.Name != nil {
		var existingLibrary models.Library
		if err := h.db.Where("name = ? AND id != ?", *req.Name, id).First(&existingLibrary).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Library with this name already exists"})
			return
		}
	}

	// Check if another library with same images path exists (only if path is changing)
	var pathChanged bool
	if req.Images != nil && *req.Images != library.Images {
		var existingLibrary models.Library
		if err := h.db.Where("images = ? AND id != ?", *req.Images, id).First(&existingLibrary).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Library with this images path already exists"})
			return
		}
		pathChanged = true
	}

	// Update only provided fields
	if req.Name != nil {
		library.Name = *req.Name
	}
	if req.Description != nil {
		library.Description = *req.Description
	}
	if req.Images != nil {
		library.Images = *req.Images
	}

	// If images path is changing, handle directory operations
	if pathChanged {
		// Create new directory
		if err := createDirectoryIfNotExists(library.Images); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create new images directory"})
			return
		}

		// TODO: Move existing files from old path to new path
		// This is a complex operation that should be done carefully
		// For now, we'll just create the new directory and let users handle migration
	}

	if err := h.db.Save(&library).Error; err != nil {
		// If database save failed and we created a new directory, clean it up
		if pathChanged {
			removeDirectoryIfExists(library.Images)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update library"})
		return
	}

	c.JSON(http.StatusOK, library)
}

// DeleteLibrary deletes a library and all its associated data
func (h *LibraryHandler) DeleteLibrary(c *gin.Context) {
	libraryID := c.Param("id")

	id, err := uuid.Parse(libraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}

	var library models.Library
	if err := h.db.First(&library, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch library"})
		return
	}

	// Use transaction to ensure data consistency
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete all photos in this library (this will also clean up photo_tags and album_photos via foreign key constraints)
	if err := tx.Where("library_id = ?", id).Delete(&models.Photo{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete library photos"})
		return
	}

	// Delete all albums in this library
	if err := tx.Where("library_id = ?", id).Delete(&models.Album{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete library albums"})
		return
	}

	// Delete the library itself
	if err := tx.Delete(&library).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete library"})
		return
	}

	tx.Commit()

	// Remove the library's images directory and all its contents
	if err := removeDirectoryIfExists(library.Images); err != nil {
		// Log error but don't fail the request since DB is already updated
		// In production, you might want to queue this for retry
		c.JSON(http.StatusOK, gin.H{
			"message": "Library deleted successfully",
			"warning": "Failed to remove some image files, manual cleanup may be required",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Library deleted successfully"})
}

// GetLibraryStats returns statistics for a library
func (h *LibraryHandler) GetLibraryStats(c *gin.Context) {
	libraryID := c.Param("id")

	id, err := uuid.Parse(libraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}

	// Check if library exists
	var library models.Library
	if err := h.db.First(&library, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch library"})
		return
	}

	stats := struct {
		LibraryID   uuid.UUID `json:"library_id"`
		LibraryName string    `json:"library_name"`
		PhotoCount  int64     `json:"photo_count"`
		AlbumCount  int64     `json:"album_count"`
		TagCount    int64     `json:"tag_count"`
		TotalSize   int64     `json:"total_size_bytes"`
	}{
		LibraryID:   library.ID,
		LibraryName: library.Name,
	}

	// Count photos
	h.db.Model(&models.Photo{}).Where("library_id = ?", id).Count(&stats.PhotoCount)

	// Count albums
	h.db.Model(&models.Album{}).Where("library_id = ?", id).Count(&stats.AlbumCount)

	// Count unique tags used in this library
	h.db.Table("tags").
		Joins("JOIN photo_tags ON tags.id = photo_tags.tag_id").
		Joins("JOIN photos ON photo_tags.photo_id = photos.id").
		Where("photos.library_id = ?", id).
		Distinct("tags.id").
		Count(&stats.TagCount)

	// Calculate total file size
	h.db.Model(&models.Photo{}).
		Where("library_id = ?", id).
		Select("COALESCE(SUM(file_size), 0)").
		Row().Scan(&stats.TotalSize)

	c.JSON(http.StatusOK, stats)
}
