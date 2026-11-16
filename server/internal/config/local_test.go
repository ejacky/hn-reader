package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalDataDirOverride(t *testing.T) {
	got := localDataDir("windows", func(key string) string {
		if key == "HN_READER_DATA_DIR" {
			return `D:\custom\data`
		}
		return ""
	}, func() (string, error) {
		t.Fatal("executable should not be consulted when the override is set")
		return "", nil
	})
	if got != `D:\custom\data` {
		t.Fatalf("LocalDataDir=%q, want D:\\custom\\data", got)
	}
}

func TestLocalDataDirWindowsDefaults(t *testing.T) {
	localAppData := `C:\Users\jacky\AppData\Local`
	got := localDataDir("windows", func(key string) string {
		if key == "LOCALAPPDATA" {
			return localAppData
		}
		return ""
	}, func() (string, error) {
		t.Fatal("executable should not be consulted when LOCALAPPDATA is set")
		return "", nil
	})
	want := filepath.Join(localAppData, "HN Reader")
	if got != want {
		t.Fatalf("LocalDataDir=%q, want %q", got, want)
	}
	if !strings.HasSuffix(got, "HN Reader") {
		t.Fatalf("LocalDataDir=%q should end in HN Reader", got)
	}

	got = localDataDir("windows", func(key string) string {
		if key == "PROGRAMDATA" {
			return `C:\ProgramData`
		}
		return ""
	}, func() (string, error) {
		return "", os.ErrNotExist
	})
	want = filepath.Join(`C:\ProgramData`, "HN Reader")
	if got != want {
		t.Fatalf("LocalDataDir=%q, want %q", got, want)
	}
}

func TestLocalDataDirFallsBackToExecutable(t *testing.T) {
	got := localDataDir("linux", func(string) string { return "" }, func() (string, error) {
		return "/opt/hn-reader/hn-reader", nil
	})
	if got != "/opt/hn-reader" {
		t.Fatalf("LocalDataDir=%q, want /opt/hn-reader", got)
	}
}

func TestMigrateLegacyDataCopiesDatabaseAndConfig(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "new")
	legacyDir := filepath.Join(root, "old")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}

	legacyDB := filepath.Join(legacyDir, "hn-reader.db")
	if err := os.WriteFile(legacyDB, []byte("database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyDB+"-wal", []byte("wal"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, ".env"), []byte("LLM_API_KEY=legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := migrateLegacyData(dataDir, legacyDir)
	want := filepath.Join(dataDir, "hn-reader.db")
	if got != want {
		t.Fatalf("migrateLegacyData returned %q, want %q", got, want)
	}
	if body, err := os.ReadFile(want); err != nil || string(body) != "database" {
		t.Fatalf("target database = %q, %v", body, err)
	}
	if body, err := os.ReadFile(want + "-wal"); err != nil || string(body) != "wal" {
		t.Fatalf("target WAL = %q, %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(dataDir, ".env")); err != nil || string(body) != "LLM_API_KEY=legacy" {
		t.Fatalf("target .env = %q, %v", body, err)
	}
}

func TestMigrateLegacyDataKeepsExistingTarget(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "new")
	legacyDir := filepath.Join(root, "old")
	for _, dir := range []string{dataDir, legacyDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	targetDB := filepath.Join(dataDir, "hn-reader.db")
	if err := os.WriteFile(targetDB, []byte("current"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "hn-reader.db"), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, ".env"), []byte("LLM_API_KEY=legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := migrateLegacyData(dataDir, legacyDir)
	if got != targetDB {
		t.Fatalf("migrateLegacyData returned %q, want %q", got, targetDB)
	}
	if body, _ := os.ReadFile(targetDB); string(body) != "current" {
		t.Fatalf("target database was overwritten: %q", body)
	}
	if body, _ := os.ReadFile(filepath.Join(dataDir, ".env")); string(body) != "LLM_API_KEY=legacy" {
		t.Fatalf("missing .env was not copied: %q", body)
	}
}

func TestMigrateLegacyDataFallsBackWhenCopyFails(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "new")
	legacyDir := filepath.Join(root, "old")
	for _, dir := range []string{dataDir, legacyDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// A directory where the database file should be makes copyFile fail.
	legacyDB := filepath.Join(legacyDir, "hn-reader.db")
	if err := os.Mkdir(legacyDB, 0o755); err != nil {
		t.Fatal(err)
	}

	got := migrateLegacyData(dataDir, legacyDir)
	if got != legacyDB {
		t.Fatalf("migrateLegacyData returned %q, want legacy path %q", got, legacyDB)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "hn-reader.db")); !os.IsNotExist(err) {
		t.Fatalf("partial target database should have been removed, err=%v", err)
	}
}

func TestMigrateLegacyDataFallsBackWhenWALCopyFails(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "new")
	legacyDir := filepath.Join(root, "old")
	for _, dir := range []string{dataDir, legacyDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	legacyDB := filepath.Join(legacyDir, "hn-reader.db")
	if err := os.WriteFile(legacyDB, []byte("database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyDB+"-wal", []byte("wal"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A directory where the target WAL file should be makes the copy fail.
	if err := os.Mkdir(filepath.Join(dataDir, "hn-reader.db-wal"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := migrateLegacyData(dataDir, legacyDir)
	if got != legacyDB {
		t.Fatalf("migrateLegacyData returned %q, want legacy path %q", got, legacyDB)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "hn-reader.db")); !os.IsNotExist(err) {
		t.Fatalf("target database should have been removed, err=%v", err)
	}
}

func TestLoadLocalResolvesRelativeDBPath(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HN_READER_DATA_DIR", dataDir)
	t.Setenv("DB_PATH", "custom.db")

	cfg := LoadLocal()
	if cfg.DatabasePath != filepath.Join(dataDir, "custom.db") {
		t.Fatalf("DatabasePath=%q, want %q", cfg.DatabasePath, filepath.Join(dataDir, "custom.db"))
	}
}

func TestLoadLocalUsesDataDir(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("HN_READER_DATA_DIR", dataDir)
	t.Setenv("DB_PATH", "")

	cfg := LoadLocal()
	if cfg.DataDir != dataDir {
		t.Fatalf("DataDir=%q, want %q", cfg.DataDir, dataDir)
	}
	if cfg.DatabasePath != filepath.Join(dataDir, "hn-reader.db") {
		t.Fatalf("DatabasePath=%q, want %q", cfg.DatabasePath, filepath.Join(dataDir, "hn-reader.db"))
	}
}
