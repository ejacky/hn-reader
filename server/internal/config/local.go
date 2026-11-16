package config

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joho/godotenv"
)

// LoadLocal loads configuration for the local desktop deployment.
// It searches for .env files in several locations and uses SQLite.
//
// Data lives in the per-user data directory (see LocalDataDir): the database,
// the .env and the log all belong there. The executable directory is still
// accepted as a legacy location so an in-place binary upgrade does not silently
// start with an empty database.
func LoadLocal() *Config {
	dataDir := LocalDataDir()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Printf("config: create data dir %s: %v", dataDir, err)
	}
	log.Printf("config: data dir %s", dataDir)

	dbDefault := migrateLegacyData(dataDir, executableDir())
	loadEnvFile(dataDir)

	dbPath := getEnv("DB_PATH", dbDefault)
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Join(dataDir, dbPath)
	}

	cfg := &Config{
		Port:       getEnv("PORT", "8080"),
		LLMBaseURL: getEnv("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:  getEnv("LLM_API_KEY", ""),
		LLMModel:   getEnv("LLM_MODEL", "gpt-4o-mini"),
		// Local mode leaves this empty by default: the target model may not be a
		// reasoning model, and some providers reject the parameter outright.
		LLMReasoningEffort: getEnv("LLM_REASONING_EFFORT", ""),
		DatabasePath:       dbPath,
		DataDir:            dataDir,
	}

	if cfg.LLMAPIKey == "" {
		log.Println("config: LLM_API_KEY is empty — summarization will fail")
	}
	return cfg
}

// LocalDataDir is where local-mode state belongs.
//
// Windows uses LOCALAPPDATA so the data is per-user and writable without
// elevation. PROGRAMDATA is a fallback for service-like contexts. Other
// platforms keep the executable directory, which is what developers expect from
// "go run ./cmd/hn-reader" and from the old behaviour.
func LocalDataDir() string {
	return localDataDir(runtime.GOOS, os.Getenv, os.Executable)
}

func localDataDir(goos string, getenv func(string) string, executable func() (string, error)) string {
	if dir := getenv("HN_READER_DATA_DIR"); dir != "" {
		return dir
	}
	if goos == "windows" {
		if base := getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "HN Reader")
		}
		if base := getenv("PROGRAMDATA"); base != "" {
			return filepath.Join(base, "HN Reader")
		}
	}
	if exe, err := executable(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}

// migrateLegacyData copies a database and config from the old executable
// directory into the data directory. It returns the database path the caller
// should use: the new path on success (or when there is nothing to migrate), or
// the legacy path if copying failed, so an upgrade never loses data silently.
func migrateLegacyData(dataDir, legacyDir string) string {
	targetDB := filepath.Join(dataDir, "hn-reader.db")
	if legacyDir == "" || samePath(dataDir, legacyDir) {
		return targetDB
	}

	legacyDB := filepath.Join(legacyDir, "hn-reader.db")
	if _, err := os.Stat(targetDB); err == nil {
		// A database already exists here. Keep it authoritative, but still pick
		// up a config file that only exists in the legacy directory.
		copyIfMissing(filepath.Join(legacyDir, ".env"), filepath.Join(dataDir, ".env"))
		return targetDB
	}
	if _, err := os.Stat(legacyDB); err != nil {
		return targetDB
	}

	if err := copyFile(legacyDB, targetDB); err != nil {
		log.Printf("config: migrate legacy database %s -> %s: %v (keeping legacy path)", legacyDB, targetDB, err)
		return legacyDB
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		src := legacyDB + suffix
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, targetDB+suffix); err != nil {
			// The WAL may contain committed transactions. A database without it
			// can look fine and still be missing data, so keep using the legacy
			// copy instead of leaving a misleading target behind.
			log.Printf("config: migrate %s: %v (keeping legacy path)", src, err)
			os.Remove(targetDB)
			os.Remove(targetDB + "-wal")
			os.Remove(targetDB + "-shm")
			return legacyDB
		}
	}
	copyIfMissing(filepath.Join(legacyDir, ".env"), filepath.Join(dataDir, ".env"))
	log.Printf("config: migrated local data from %s to %s", legacyDir, dataDir)
	return targetDB
}

func copyIfMissing(src, dst string) {
	if _, err := os.Stat(dst); err == nil {
		return
	}
	if _, err := os.Stat(src); err != nil {
		return
	}
	if err := copyFile(src, dst); err != nil {
		log.Printf("config: copy %s -> %s: %v", src, dst, err)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(dst)
		return err
	}
	return nil
}

func samePath(a, b string) bool {
	aa, err := filepath.Abs(a)
	if err != nil {
		aa = filepath.Clean(a)
	}
	bb, err := filepath.Abs(b)
	if err != nil {
		bb = filepath.Clean(b)
	}
	return aa == bb
}

func loadEnvFile(dataDir string) {
	candidates := []string{}
	if dataDir != "" {
		candidates = append(candidates, filepath.Join(dataDir, ".env"))
	}
	if execPath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), ".env"))
		log.Printf("config: executable at %s", execPath)
	} else {
		log.Printf("config: os.Executable() failed: %v", err)
	}

	candidates = append(candidates,
		".env",
		"server/.env",
		filepath.Join(os.Getenv("HOME"), ".hn-reader", ".env"),
		filepath.Join(os.Getenv("USERPROFILE"), ".hn-reader", ".env"),
	)

	seen := make(map[string]bool, len(candidates))
	for _, path := range candidates {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := godotenv.Load(path); err != nil {
			log.Printf("config: found %s but failed to load: %v", path, err)
			return
		}
		log.Printf("config: .env loaded from %s", path)
		return
	}

	log.Println("config: no .env file found, using defaults")
}
