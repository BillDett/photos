package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Migrate the schema
	err = db.AutoMigrate(&Library{}, &Album{}, &Photo{}, &Tag{}, &PhotoTag{}, &AlbumPhoto{})
	require.NoError(t, err)

	return db
}

func TestLibrary(t *testing.T) {
	t.Run("Library struct creation", func(t *testing.T) {
		library := Library{
			Name:        "My Photo Library",
			Description: "A collection of family photos",
			Images:      "/path/to/images",
		}

		assert.Equal(t, "My Photo Library", library.Name)
		assert.Equal(t, "A collection of family photos", library.Description)
		assert.Equal(t, "/path/to/images", library.Images)
		assert.Equal(t, uuid.Nil, library.ID) // Should be Nil until BeforeCreate
	})

	t.Run("Library BeforeCreate hook", func(t *testing.T) {
		db := setupTestDB(t)

		library := Library{
			Name:        "Test Library",
			Description: "Test Description",
			Images:      "/test/path",
		}

		// ID should be nil before creation
		assert.Equal(t, uuid.Nil, library.ID)

		// Create the library
		err := db.Create(&library).Error
		require.NoError(t, err)

		// ID should be set after creation
		assert.NotEqual(t, uuid.Nil, library.ID)
		assert.True(t, library.CreatedAt.After(time.Time{}))
		assert.True(t, library.UpdatedAt.After(time.Time{}))
	})

	t.Run("Library with pre-existing UUID", func(t *testing.T) {
		db := setupTestDB(t)

		existingID := uuid.New()
		library := Library{
			ID:          existingID,
			Name:        "Test Library with ID",
			Description: "Test Description",
			Images:      "/test/path",
		}

		err := db.Create(&library).Error
		require.NoError(t, err)

		// ID should remain the same as the pre-existing one
		assert.Equal(t, existingID, library.ID)
	})
}

func TestAlbum(t *testing.T) {
	t.Run("Album struct creation", func(t *testing.T) {
		libraryID := uuid.New()
		album := Album{
			Name:        "Summer Vacation",
			Description: "Photos from our summer trip",
			LibraryID:   libraryID,
		}

		assert.Equal(t, "Summer Vacation", album.Name)
		assert.Equal(t, "Photos from our summer trip", album.Description)
		assert.Equal(t, libraryID, album.LibraryID)
		assert.Equal(t, uuid.Nil, album.ID)
	})

	t.Run("Album BeforeCreate hook", func(t *testing.T) {
		db := setupTestDB(t)

		// Create a library first
		library := Library{
			Name:        "Test Library",
			Description: "Test Description",
			Images:      "/test/path",
		}
		err := db.Create(&library).Error
		require.NoError(t, err)

		album := Album{
			Name:        "Test Album",
			Description: "Test Album Description",
			LibraryID:   library.ID,
		}

		// ID should be nil before creation
		assert.Equal(t, uuid.Nil, album.ID)

		err = db.Create(&album).Error
		require.NoError(t, err)

		// ID should be set after creation
		assert.NotEqual(t, uuid.Nil, album.ID)
		assert.True(t, album.CreatedAt.After(time.Time{}))
		assert.True(t, album.UpdatedAt.After(time.Time{}))
	})
}

func TestPhoto(t *testing.T) {
	t.Run("Photo struct creation", func(t *testing.T) {
		libraryID := uuid.New()
		rating := 4
		photo := Photo{
			Filename:     "photo1.jpg",
			OriginalName: "IMG_1234.jpg",
			FilePath:     "/photos/photo1.jpg",
			MimeType:     "image/jpeg",
			FileSize:     1024000,
			Width:        1920,
			Height:       1080,
			Rating:       &rating,
			LibraryID:    libraryID,
		}

		assert.Equal(t, "photo1.jpg", photo.Filename)
		assert.Equal(t, "IMG_1234.jpg", photo.OriginalName)
		assert.Equal(t, "/photos/photo1.jpg", photo.FilePath)
		assert.Equal(t, "image/jpeg", photo.MimeType)
		assert.Equal(t, int64(1024000), photo.FileSize)
		assert.Equal(t, 1920, photo.Width)
		assert.Equal(t, 1080, photo.Height)
		assert.Equal(t, 4, *photo.Rating)
		assert.Equal(t, libraryID, photo.LibraryID)
		assert.Equal(t, uuid.Nil, photo.ID)
	})

	t.Run("Photo with nil rating", func(t *testing.T) {
		photo := Photo{
			Filename:     "photo2.jpg",
			OriginalName: "IMG_5678.jpg",
			FilePath:     "/photos/photo2.jpg",
			MimeType:     "image/jpeg",
			FileSize:     2048000,
			LibraryID:    uuid.New(),
			Rating:       nil,
		}

		assert.Nil(t, photo.Rating)
	})

	t.Run("Photo BeforeCreate hook", func(t *testing.T) {
		db := setupTestDB(t)

		// Create a library first
		library := Library{
			Name:        "Test Library",
			Description: "Test Description",
			Images:      "/test/path",
		}
		err := db.Create(&library).Error
		require.NoError(t, err)

		beforeTime := time.Now()
		photo := Photo{
			Filename:     "test_photo.jpg",
			OriginalName: "IMG_TEST.jpg",
			FilePath:     "/test/photo.jpg",
			MimeType:     "image/jpeg",
			FileSize:     1000000,
			LibraryID:    library.ID,
		}

		// ID and UploadedAt should be nil/zero before creation
		assert.Equal(t, uuid.Nil, photo.ID)
		assert.True(t, photo.UploadedAt.IsZero())

		err = db.Create(&photo).Error
		require.NoError(t, err)

		// ID should be set and UploadedAt should be set to current time
		assert.NotEqual(t, uuid.Nil, photo.ID)
		assert.True(t, photo.UploadedAt.After(beforeTime))
		assert.True(t, photo.CreatedAt.After(time.Time{}))
		assert.True(t, photo.UpdatedAt.After(time.Time{}))
	})

	t.Run("Photo with pre-existing UploadedAt", func(t *testing.T) {
		db := setupTestDB(t)

		// Create a library first
		library := Library{
			Name:        "Test Library",
			Description: "Test Description",
			Images:      "/test/path",
		}
		err := db.Create(&library).Error
		require.NoError(t, err)

		existingTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		photo := Photo{
			Filename:     "test_photo.jpg",
			OriginalName: "IMG_TEST.jpg",
			FilePath:     "/test/photo.jpg",
			MimeType:     "image/jpeg",
			FileSize:     1000000,
			LibraryID:    library.ID,
			UploadedAt:   existingTime,
		}

		err = db.Create(&photo).Error
		require.NoError(t, err)

		// UploadedAt should remain the existing time
		assert.Equal(t, existingTime, photo.UploadedAt)
	})
}

func TestTag(t *testing.T) {
	t.Run("Tag struct creation", func(t *testing.T) {
		tag := Tag{
			Name:  "landscape",
			Color: "#FF5733",
		}

		assert.Equal(t, "landscape", tag.Name)
		assert.Equal(t, "#FF5733", tag.Color)
		assert.Equal(t, uuid.Nil, tag.ID)
	})

	t.Run("Tag without color", func(t *testing.T) {
		tag := Tag{
			Name: "portrait",
		}

		assert.Equal(t, "portrait", tag.Name)
		assert.Empty(t, tag.Color)
	})

	t.Run("Tag BeforeCreate hook", func(t *testing.T) {
		db := setupTestDB(t)

		tag := Tag{
			Name:  "test-tag",
			Color: "#123456",
		}

		// ID should be nil before creation
		assert.Equal(t, uuid.Nil, tag.ID)

		err := db.Create(&tag).Error
		require.NoError(t, err)

		// ID should be set after creation
		assert.NotEqual(t, uuid.Nil, tag.ID)
		assert.True(t, tag.CreatedAt.After(time.Time{}))
		assert.True(t, tag.UpdatedAt.After(time.Time{}))
	})
}

func TestPhotoTag(t *testing.T) {
	t.Run("PhotoTag struct creation", func(t *testing.T) {
		photoID := uuid.New()
		tagID := uuid.New()

		photoTag := PhotoTag{
			PhotoID: photoID,
			TagID:   tagID,
		}

		assert.Equal(t, photoID, photoTag.PhotoID)
		assert.Equal(t, tagID, photoTag.TagID)
	})

	t.Run("PhotoTag database creation", func(t *testing.T) {
		db := setupTestDB(t)

		// Create library, photo, and tag first
		library := Library{
			Name:        "Test Library",
			Description: "Test Description",
			Images:      "/test/path",
		}
		err := db.Create(&library).Error
		require.NoError(t, err)

		photo := Photo{
			Filename:     "test.jpg",
			OriginalName: "test.jpg",
			FilePath:     "/test.jpg",
			MimeType:     "image/jpeg",
			FileSize:     1000,
			LibraryID:    library.ID,
		}
		err = db.Create(&photo).Error
		require.NoError(t, err)

		tag := Tag{
			Name: "test-tag",
		}
		err = db.Create(&tag).Error
		require.NoError(t, err)

		photoTag := PhotoTag{
			PhotoID: photo.ID,
			TagID:   tag.ID,
		}

		err = db.Create(&photoTag).Error
		require.NoError(t, err)

		assert.Equal(t, photo.ID, photoTag.PhotoID)
		assert.Equal(t, tag.ID, photoTag.TagID)
	})
}

func TestAlbumPhoto(t *testing.T) {
	t.Run("AlbumPhoto struct creation", func(t *testing.T) {
		albumID := uuid.New()
		photoID := uuid.New()

		albumPhoto := AlbumPhoto{
			AlbumID: albumID,
			PhotoID: photoID,
			Order:   1,
		}

		assert.Equal(t, albumID, albumPhoto.AlbumID)
		assert.Equal(t, photoID, albumPhoto.PhotoID)
		assert.Equal(t, 1, albumPhoto.Order)
	})

	t.Run("AlbumPhoto with default order", func(t *testing.T) {
		albumPhoto := AlbumPhoto{
			AlbumID: uuid.New(),
			PhotoID: uuid.New(),
		}

		assert.Equal(t, 0, albumPhoto.Order)
	})

	t.Run("AlbumPhoto database creation", func(t *testing.T) {
		db := setupTestDB(t)

		// Create library, album, and photo first
		library := Library{
			Name:        "Test Library",
			Description: "Test Description",
			Images:      "/test/path",
		}
		err := db.Create(&library).Error
		require.NoError(t, err)

		album := Album{
			Name:        "Test Album",
			Description: "Test Album",
			LibraryID:   library.ID,
		}
		err = db.Create(&album).Error
		require.NoError(t, err)

		photo := Photo{
			Filename:     "test.jpg",
			OriginalName: "test.jpg",
			FilePath:     "/test.jpg",
			MimeType:     "image/jpeg",
			FileSize:     1000,
			LibraryID:    library.ID,
		}
		err = db.Create(&photo).Error
		require.NoError(t, err)

		albumPhoto := AlbumPhoto{
			AlbumID: album.ID,
			PhotoID: photo.ID,
			Order:   5,
		}

		err = db.Create(&albumPhoto).Error
		require.NoError(t, err)

		assert.Equal(t, album.ID, albumPhoto.AlbumID)
		assert.Equal(t, photo.ID, albumPhoto.PhotoID)
		assert.Equal(t, 5, albumPhoto.Order)
	})
}

func TestModelRelationships(t *testing.T) {
	t.Run("Library with Albums and Photos", func(t *testing.T) {
		db := setupTestDB(t)

		// Create library
		library := Library{
			Name:        "Test Library",
			Description: "Test Description",
			Images:      "/test/path",
		}
		err := db.Create(&library).Error
		require.NoError(t, err)

		// Create album
		album := Album{
			Name:        "Test Album",
			Description: "Test Album",
			LibraryID:   library.ID,
		}
		err = db.Create(&album).Error
		require.NoError(t, err)

		// Create photo
		photo := Photo{
			Filename:     "test.jpg",
			OriginalName: "test.jpg",
			FilePath:     "/test.jpg",
			MimeType:     "image/jpeg",
			FileSize:     1000,
			LibraryID:    library.ID,
		}
		err = db.Create(&photo).Error
		require.NoError(t, err)

		// Load library with associations
		var loadedLibrary Library
		err = db.Preload("Albums").Preload("Photos").First(&loadedLibrary, library.ID).Error
		require.NoError(t, err)

		assert.Len(t, loadedLibrary.Albums, 1)
		assert.Len(t, loadedLibrary.Photos, 1)
		assert.Equal(t, album.ID, loadedLibrary.Albums[0].ID)
		assert.Equal(t, photo.ID, loadedLibrary.Photos[0].ID)
	})
}
