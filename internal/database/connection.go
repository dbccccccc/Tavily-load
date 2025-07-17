package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Host     string
	Port     string
	Username string
	Password string
	Database string
	MaxOpenConns int
	MaxIdleConns int
	ConnMaxLifetime time.Duration
}

type DB struct {
	*sql.DB
	config *Config
}

func NewConnection(config *Config) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logrus.Info("Successfully connected to MySQL database")

	return &DB{
		DB:     db,
		config: config,
	}, nil
}

func (db *DB) Close() error {
	logrus.Info("Closing database connection")
	return db.DB.Close()
}

func (db *DB) GetConfig() *Config {
	return db.config
}

func (db *DB) Ping() error {
	return db.DB.Ping()
}