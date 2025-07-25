package handlers

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"photo-library-server/config"
	"photo-library-server/models"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PhotoHandler handles photo-related HTTP requests
type PhotoHandler struct {
	db     *gorm.DB
	config *config.Config
}

// NewPhotoHandler creates a new photo handler
func NewPhotoHandler(db *gorm.DB, cfg *config.Config) *PhotoHandler {
	return &PhotoHandler{db: db, config: cfg}
}

// UploadPhoto handles photo upload
func (h *PhotoHandler) UploadPhoto(c *gin.Context) {
	// Parse multipart form
	err := c.Request.ParseMultipartForm(h.config.MaxFileSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File too large or invalid form data"})
		return
	}

	// Get library ID
	libraryIDStr := c.PostForm("library_id")
	if libraryIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "library_id is required"})
		return
	}

	libraryID, err := uuid.Parse(libraryIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
		return
	}

	// Verify library exists
	var library models.Library
	if err := h.db.First(&library, libraryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Library not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify library"})
		return
	}

	// Get the uploaded file
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No photo file provided"})
		return
	}
	defer file.Close()

	// Validate file type
	if !h.isValidImageType(header.Header.Get("Content-Type")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image type. Supported types: JPEG, PNG, GIF, WebP, TIFF, BMP"})
		return
	}

	// Validate file size
	if header.Size > h.config.MaxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("File size exceeds maximum allowed size of %d bytes", h.config.MaxFileSize)})
		return
	}

	// Get image dimensions
	width, height, err := h.getImageDimensions(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image file"})
		return
	}

	// Reset file pointer
	file.Seek(0, 0)

	// Generate unique filename
	filename := h.generateUniqueFilename(header.Filename)
	filePath := filepath.Join(library.Images, filename)

	// Ensure library images directory exists
	if err := os.MkdirAll(library.Images, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create library images directory"})
		return
	}

	// Save file to disk
	dst, err := os.Create(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath) // Cleanup on failure
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Parse optional rating
	var rating *int
	if ratingStr := c.PostForm("rating"); ratingStr != "" {
		if r, err := strconv.Atoi(ratingStr); err == nil && r >= 0 && r <= 5 {
			rating = &r
		}
	}

	// Create photo record
	photo := models.Photo{
		Filename:     filename,
		OriginalName: header.Filename,
		FilePath:     filePath,
		MimeType:     header.Header.Get("Content-Type"),
		FileSize:     header.Size,
		Width:        width,
		Height:       height,
		Rating:       rating,
		LibraryID:    libraryID,
		UploadedAt:   time.Now(),
	}

	if err := h.db.Create(&photo).Error; err != nil {
		os.Remove(filePath) // Cleanup on failure
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save photo metadata"})
		return
	}

	// Handle tags if provided
	if tagsStr := c.PostForm("tags"); tagsStr != "" {
		tags := strings.Split(tagsStr, ",")
		for _, tagName := range tags {
			tagName = strings.TrimSpace(tagName)
			if tagName != "" {
				h.addTagToPhoto(&photo, tagName)
			}
		}
	}

	// Load the photo with library for response
	h.db.Preload("Library").Preload("Tags").First(&photo, photo.ID)

	c.JSON(http.StatusCreated, photo)
}

// GetPhotos returns photos, optionally filtered
func (h *PhotoHandler) GetPhotos(c *gin.Context) {
	var photos []models.Photo

	query := h.db.Model(&models.Photo{})

	// Filter by library if specified
	if libraryID := c.Query("library_id"); libraryID != "" {
		id, err := uuid.Parse(libraryID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid library ID"})
			return
		}
		query = query.Where("library_id = ?", id)
	}

	// Filter by rating if specified
	if rating := c.Query("rating"); rating != "" {
		if r, err := strconv.Atoi(rating); err == nil && r >= 0 && r <= 5 {
			query = query.Where("rating = ?", r)
		}
	}

	// Filter by tag if specified
	if tagName := c.Query("tag"); tagName != "" {
		query = query.Joins("JOIN photo_tags ON photos.id = photo_tags.photo_id").
			Joins("JOIN tags ON photo_tags.tag_id = tags.id").
			Where("tags.name = ?", tagName)
	}

	// Pagination
	page := 1
	limit := 50 // Default limit
	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	offset := (page - 1) * limit
	query = query.Offset(offset).Limit(limit)

	// Ordering
	orderBy := c.DefaultQuery("order_by", "uploaded_at")
	orderDir := c.DefaultQuery("order_dir", "desc")
	if orderDir != "asc" && orderDir != "desc" {
		orderDir = "desc"
	}

	allowedOrderFields := []string{"uploaded_at", "created_at", "rating", "filename", "file_size"}
	isValidOrderField := false
	for _, field := range allowedOrderFields {
		if field == orderBy {
			isValidOrderField = true
			break
		}
	}
	if !isValidOrderField {
		orderBy = "uploaded_at"
	}

	query = query.Order(fmt.Sprintf("%s %s", orderBy, orderDir))

	// Optional: include related data
	if c.Query("include_library") == "true" {
		query = query.Preload("Library")
	}
	if c.Query("include_tags") == "true" {
		query = query.Preload("Tags")
	}
	if c.Query("include_albums") == "true" {
		query = query.Preload("Albums")
	}

	if err := query.Find(&photos).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch photos"})
		return
	}

	// Get total count for pagination
	var total int64
	countQuery := h.db.Model(&models.Photo{})
	if libraryID := c.Query("library_id"); libraryID != "" {
		id, _ := uuid.Parse(libraryID)
		countQuery = countQuery.Where("library_id = ?", id)
	}
	if rating := c.Query("rating"); rating != "" {
		if r, err := strconv.Atoi(rating); err == nil && r >= 0 && r <= 5 {
			countQuery = countQuery.Where("rating = ?", r)
		}
	}
	if tagName := c.Query("tag"); tagName != "" {
		countQuery = countQuery.Joins("JOIN photo_tags ON photos.id = photo_tags.photo_id").
			Joins("JOIN tags ON photo_tags.tag_id = tags.id").
			Where("tags.name = ?", tagName)
	}
	countQuery.Count(&total)

	response := gin.H{
		"photos": photos,
		"pagination": gin.H{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	}

	c.JSON(http.StatusOK, response)
}

// GetPhoto returns a specific photo by ID
func (h *PhotoHandler) GetPhoto(c *gin.Context) {
	photoID := c.Param("id")

	id, err := uuid.Parse(photoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid photo ID"})
		return
	}

	var photo models.Photo
	query := h.db.Model(&models.Photo{})

	// Optional: include related data
	if c.Query("include_library") == "true" {
		query = query.Preload("Library")
	}
	if c.Query("include_tags") == "true" {
		query = query.Preload("Tags")
	}
	if c.Query("include_albums") == "true" {
		query = query.Preload("Albums")
	}

	if err := query.First(&photo, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch photo"})
		return
	}

	c.JSON(http.StatusOK, photo)
}

// UpdatePhoto updates photo metadata
func (h *PhotoHandler) UpdatePhoto(c *gin.Context) {
	photoID := c.Param("id")

	id, err := uuid.Parse(photoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid photo ID"})
		return
	}

	var req struct {
		Rating *int `json:"rating" binding:"omitempty,min=0,max=5"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var photo models.Photo
	if err := h.db.First(&photo, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch photo"})
		return
	}

	// Update rating
	photo.Rating = req.Rating

	if err := h.db.Save(&photo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update photo"})
		return
	}

	c.JSON(http.StatusOK, photo)
}

// DeletePhoto deletes a photo and its file
func (h *PhotoHandler) DeletePhoto(c *gin.Context) {
	photoID := c.Param("id")

	id, err := uuid.Parse(photoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid photo ID"})
		return
	}

	var photo models.Photo
	if err := h.db.First(&photo, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch photo"})
		return
	}

	// Use transaction to clean up all relationships
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete photo_tags relationships
	if err := tx.Where("photo_id = ?", id).Delete(&models.PhotoTag{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove photo tags"})
		return
	}

	// Delete album_photos relationships
	if err := tx.Where("photo_id = ?", id).Delete(&models.AlbumPhoto{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove photo from albums"})
		return
	}

	// Delete the photo record
	if err := tx.Delete(&photo).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete photo"})
		return
	}

	tx.Commit()

	// Delete the physical file
	if err := os.Remove(photo.FilePath); err != nil {
		// Log error but don't fail the request since DB is already updated
		// In production, you might want to queue this for retry
		fmt.Printf("Warning: Failed to delete file %s: %v\n", photo.FilePath, err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Photo deleted successfully"})
}

// ServePhoto serves the actual photo file
func (h *PhotoHandler) ServePhoto(c *gin.Context) {
	photoID := c.Param("id")

	id, err := uuid.Parse(photoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid photo ID"})
		return
	}

	var photo models.Photo
	if err := h.db.First(&photo, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Photo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch photo"})
		return
	}

	// Check if file exists
	if _, err := os.Stat(photo.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Photo file not found"})
		return
	}

	c.Header("Content-Type", photo.MimeType)
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", photo.OriginalName))
	c.File(photo.FilePath)
}

// CopyPhoto copies a photo to the same or different library with a new unique identifier
func (h *PhotoHandler) CopyPhoto(c *gin.Context) {
	photoID := c.Param("id")

	sourceID, err := uuid.Parse(photoID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid photo ID"})
		return
	}

	var req struct {
		LibraryID uuid.UUID `json:"library_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify source photo exists
	var sourcePhoto models.Photo
	if err := h.db.Preload("Tags").First(&sourcePhoto, sourceID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Source photo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch source photo"})
		return
	}

	// Verify target library exists
	var targetLibrary models.Library
	if err := h.db.First(&targetLibrary, req.LibraryID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Target library not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify target library"})
		return
	}

	// Check if source file exists
	if _, err := os.Stat(sourcePhoto.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source photo file not found"})
		return
	}

	// Generate new filename for the copy
	newFilename := h.generateUniqueFilename(sourcePhoto.OriginalName)
	newFilePath := filepath.Join(targetLibrary.Images, newFilename)

	// Ensure target library images directory exists
	if err := os.MkdirAll(targetLibrary.Images, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create target library images directory"})
		return
	}

	// Copy the physical file
	if err := h.copyFile(sourcePhoto.FilePath, newFilePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy photo file"})
		return
	}

	// Create new photo record with copied metadata
	newPhoto := models.Photo{
		Filename:     newFilename,
		OriginalName: sourcePhoto.OriginalName,
		FilePath:     newFilePath,
		MimeType:     sourcePhoto.MimeType,
		FileSize:     sourcePhoto.FileSize,
		Width:        sourcePhoto.Width,
		Height:       sourcePhoto.Height,
		Rating:       sourcePhoto.Rating,
		LibraryID:    req.LibraryID,
		UploadedAt:   time.Now(), // New upload time for the copy
	}

	// Use transaction to ensure data consistency
	tx := h.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create the new photo record
	if err := tx.Create(&newPhoto).Error; err != nil {
		tx.Rollback()
		os.Remove(newFilePath) // Cleanup file on failure
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create photo copy"})
		return
	}

	// Copy all tags from source photo to new photo
	for _, tag := range sourcePhoto.Tags {
		photoTag := models.PhotoTag{
			PhotoID: newPhoto.ID,
			TagID:   tag.ID,
		}
		if err := tx.Create(&photoTag).Error; err != nil {
			tx.Rollback()
			os.Remove(newFilePath) // Cleanup file on failure
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to copy photo tags"})
			return
		}
	}

	tx.Commit()

	// Load the new photo with all relationships for response
	h.db.Preload("Library").Preload("Tags").First(&newPhoto, newPhoto.ID)

	c.JSON(http.StatusCreated, gin.H{
		"message":      "Photo copied successfully",
		"original_id":  sourcePhoto.ID,
		"copied_photo": newPhoto,
	})
}

// Helper methods

func (h *PhotoHandler) isValidImageType(mimeType string) bool {
	for _, allowedType := range h.config.AllowedTypes {
		if mimeType == allowedType {
			return true
		}
	}
	return false
}

func (h *PhotoHandler) getImageDimensions(file multipart.File) (int, int, error) {
	img, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return img.Width, img.Height, nil
}

func (h *PhotoHandler) generateUniqueFilename(originalName string) string {
	ext := filepath.Ext(originalName)
	name := strings.TrimSuffix(originalName, ext)
	timestamp := time.Now().Unix()
	uuid := uuid.New().String()[:8]
	return fmt.Sprintf("%s_%d_%s%s", name, timestamp, uuid, ext)
}

func (h *PhotoHandler) addTagToPhoto(photo *models.Photo, tagName string) error {
	// Find or create tag
	var tag models.Tag
	if err := h.db.Where("name = ?", tagName).First(&tag).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new tag
			tag = models.Tag{Name: tagName}
			if err := h.db.Create(&tag).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Create photo-tag relationship
	photoTag := models.PhotoTag{
		PhotoID: photo.ID,
		TagID:   tag.ID,
	}

	return h.db.Create(&photoTag).Error
}

func (h *PhotoHandler) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Ensure file is written to disk
	return destFile.Sync()
}
