package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDatabase(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	
	// Test database creation
	db, err := NewDatabase(tempDir)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	// Test that database file was created
	dbPath := filepath.Join(tempDir, "orchestrator.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Errorf("Database file was not created at %s", dbPath)
	}
	
	// Test health check
	if err := db.Health(); err != nil {
		t.Errorf("Database health check failed: %v", err)
	}
}

func TestDatabaseMigration(t *testing.T) {
	tempDir := t.TempDir()
	
	db, err := NewDatabase(tempDir)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	// Test that tables were created
	tables := []string{"resources", "nodes", "pod_assignments"}
	
	for _, table := range tables {
		query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?`
		var name string
		err := db.DB().QueryRow(query, table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s was not created: %v", table, err)
		}
		if name != table {
			t.Errorf("Expected table name %s, got %s", table, name)
		}
	}
}

func TestDatabaseTransaction(t *testing.T) {
	tempDir := t.TempDir()
	
	db, err := NewDatabase(tempDir)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()
	
	// Test transaction creation
	tx, err := db.BeginTx()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	
	// Test rollback
	if err := tx.Rollback(); err != nil {
		t.Errorf("Failed to rollback transaction: %v", err)
	}
}