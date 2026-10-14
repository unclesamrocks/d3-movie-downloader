package db

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// DomainWhitelist represents a whitelisted domain
type DomainWhitelist struct {
	ID        int64
	Domain    string
	CreatedAt string
}

// Database wraps SQLite database operations for whitelist management
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection and initializes tables
func NewDatabase(dbPath string) (*Database, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode and foreign keys
	if _, err := sqlDB.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		sqlDB.Close()
		return nil, err
	}

	d := &Database{db: sqlDB}
	if err := d.init(); err != nil {
		sqlDB.Close()
		return nil, err
	}

	return d, nil
}

// init creates tables if they don't exist
func (d *Database) init() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_whitelist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_user_id INTEGER NOT NULL UNIQUE,
			created_at TEXT DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS admin_whitelist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_user_id INTEGER NOT NULL UNIQUE,
			created_at TEXT DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS domain_whitelist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT NOT NULL UNIQUE,
			created_at TEXT DEFAULT (datetime('now'))
		);

		CREATE INDEX IF NOT EXISTS idx_user_whitelist_telegram_id ON user_whitelist(telegram_user_id);
		CREATE INDEX IF NOT EXISTS idx_admin_whitelist_telegram_id ON admin_whitelist(telegram_user_id);
		CREATE INDEX IF NOT EXISTS idx_domain_whitelist_domain ON domain_whitelist(domain);
	`)
	return err
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// User whitelist methods

// IsUserWhitelisted checks if a user is in the whitelist
func (d *Database) IsUserWhitelisted(telegramUserID int64) bool {
	var exists bool
	err := d.db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM user_whitelist WHERE telegram_user_id = ?)",
		telegramUserID,
	).Scan(&exists)
	return err == nil && exists
}

// AddUserToWhitelist adds a user to the whitelist
func (d *Database) AddUserToWhitelist(telegramUserID int64) error {
	_, err := d.db.Exec(
		"INSERT OR IGNORE INTO user_whitelist (telegram_user_id) VALUES (?)",
		telegramUserID,
	)
	return err
}

// AddUserToAdminWhitelist adds a user to the admin whitelist
func (d *Database) AddUserToAdminWhitelist(telegramUserID int64) error {
	_, err := d.db.Exec(
		"INSERT OR IGNORE INTO admin_whitelist (telegram_user_id) VALUES (?)",
		telegramUserID,
	)
	return err
}

// RemoveUserFromWhitelist removes a user from the whitelist
func (d *Database) RemoveUserFromWhitelist(telegramUserID int64) error {
	_, err := d.db.Exec(
		"DELETE FROM user_whitelist WHERE telegram_user_id = ?",
		telegramUserID,
	)
	return err
}

// Admin whitelist methods

// IsAdminWhitelisted checks if a user is an admin
func (d *Database) IsAdminWhitelisted(telegramUserID int64) bool {
	var exists bool
	err := d.db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM admin_whitelist WHERE telegram_user_id = ?)",
		telegramUserID,
	).Scan(&exists)
	return err == nil && exists
}

// Domain whitelist methods

// IsDomainWhitelisted checks if a domain is whitelisted (exact match)
func (d *Database) IsDomainWhitelisted(domain string) bool {
	var exists bool
	err := d.db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM domain_whitelist WHERE domain = ?)",
		domain,
	).Scan(&exists)
	return err == nil && exists
}

// IsDomainOrSubdomainWhitelisted checks if a URL's domain or any parent domain is whitelisted
func (d *Database) IsDomainOrSubdomainWhitelisted(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}

	hostname := strings.ToLower(u.Hostname())

	// Check exact domain match
	if d.IsDomainWhitelisted(hostname) {
		return true
	}

	// Check subdomain matches (e.g., api.d3.ru matches d3.ru)
	domains := d.GetAllWhitelistedDomains()
	for _, domain := range domains {
		if hostname == domain.Domain || strings.HasSuffix(hostname, "."+domain.Domain) {
			return true
		}
	}

	return false
}

// AddDomainToWhitelist adds a domain to the whitelist
func (d *Database) AddDomainToWhitelist(domain string) error {
	_, err := d.db.Exec(
		"INSERT OR IGNORE INTO domain_whitelist (domain) VALUES (?)",
		strings.ToLower(domain),
	)
	return err
}

// RemoveDomainFromWhitelist removes a domain from the whitelist
func (d *Database) RemoveDomainFromWhitelist(domain string) error {
	_, err := d.db.Exec(
		"DELETE FROM domain_whitelist WHERE domain = ?",
		strings.ToLower(domain),
	)
	return err
}

// GetAllWhitelistedDomains returns all whitelisted domains
func (d *Database) GetAllWhitelistedDomains() []DomainWhitelist {
	rows, err := d.db.Query("SELECT id, domain, created_at FROM domain_whitelist ORDER BY domain ASC")
	if err != nil {
		return nil
	}
	defer rows.Close()

	var domains []DomainWhitelist
	for rows.Next() {
		var d DomainWhitelist
		if err := rows.Scan(&d.ID, &d.Domain, &d.CreatedAt); err != nil {
			continue
		}
		domains = append(domains, d)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return domains
}
