package database

import (
	"fmt"
	"log"

	"photo-library-server/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database interface defines the contract for database operations
type Database interface {
	GetDB() *gorm.DB
	Migrate() error
	Close() error
}

// SQLiteDB implements the Database interface for SQLite
type SQLiteDB struct {
	db *gorm.DB
}

// NewSQLiteDB creates a new SQLite database connection
func NewSQLiteDB(dbPath string) (*SQLiteDB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &SQLiteDB{db: db}, nil
}

// GetDB returns the underlying GORM database instance
func (s *SQLiteDB) GetDB() *gorm.DB {
	return s.db
}

// Migrate runs database migrations for all models
func (s *SQLiteDB) Migrate() error {
	err := s.db.AutoMigrate(
		&models.Library{},
		&models.Album{},
		&models.Photo{},
		&models.Tag{},
		&models.PhotoTag{},
		&models.AlbumPhoto{},
	)
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database migration completed successfully")
	return nil
}

// Close closes the database connection
func (s *SQLiteDB) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// CreateIndexes creates additional indexes for better performance
func (s *SQLiteDB) CreateIndexes() error {
	// Create composite indexes for better query performance
	if err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_photos_library_uploaded ON photos(library_id, uploaded_at DESC)").Error; err != nil {
		return fmt.Errorf("failed to create photos library-uploaded index: %w", err)
	}

	if err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_photos_rating ON photos(rating) WHERE rating IS NOT NULL").Error; err != nil {
		return fmt.Errorf("failed to create photos rating index: %w", err)
	}

	if err := s.db.Exec("CREATE INDEX IF NOT EXISTS idx_album_photos_order ON album_photos(album_id, \"order\")").Error; err != nil {
		return fmt.Errorf("failed to create album photos order index: %w", err)
	}

	log.Println("Database indexes created successfully")
	return nil
}

// PostgresDB would be implemented here for future PostgreSQL support
// type PostgresDB struct {
//     db *gorm.DB
// }
//
// func NewPostgresDB(dsn string) (*PostgresDB, error) {
//     // Implementation for PostgreSQL
// }
