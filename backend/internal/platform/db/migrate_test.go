package db

import (
	"testing"
	"testing/fstest"
)

func TestDiscoverMigrations_OrdersByVersionAndIgnoresNonMigrationFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"0002_add_accounts.up.sql":   {Data: []byte("-- up")},
		"0002_add_accounts.down.sql": {Data: []byte("-- down")},
		"0001_init.up.sql":           {Data: []byte("-- up")},
		"README.md":                  {Data: []byte("not a migration")},
	}

	files, err := discoverMigrations(fsys)
	if err != nil {
		t.Fatalf("discoverMigrations() error = %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("len(files) = %d, want 2 (got %+v)", len(files), files)
	}
	if files[0].Version != "0001_init" {
		t.Errorf("files[0].Version = %q, want 0001_init", files[0].Version)
	}
	if files[1].Version != "0002_add_accounts" {
		t.Errorf("files[1].Version = %q, want 0002_add_accounts", files[1].Version)
	}
}

func TestDiscoverMigrations_EmptyDirectoryYieldsNoFiles(t *testing.T) {
	files, err := discoverMigrations(fstest.MapFS{})
	if err != nil {
		t.Fatalf("discoverMigrations() error = %v", err)
	}
	if len(files) != 0 {
		t.Errorf("len(files) = %d, want 0", len(files))
	}
}
