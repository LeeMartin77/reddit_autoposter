package storage

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type Token struct {
	ID     string
	Token  string
	Expiry string
}

type TokenStorage interface {
	Get(id string) (*Token, error)
	Insert(token *Token) error
	Update(token *Token) error
	Delete(id string) error
	Close() error
}

type SQLiteTokenStorage struct {
	db *sql.DB
}

var (
	ErrNoToken = fmt.Errorf("token with id not found")
)

var create_migrations_table = `CREATE TABLE IF NOT EXISTS migrations (idx INTEGER PRIMARY KEY);`

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS tokens (
		id TEXT PRIMARY KEY,
		token TEXT NOT NULL,
		expiry TEXT
	);`,
}

func NewStorage(dbPath string) (TokenStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not open SQLite database: %w", err)
	}

	_, err = db.Exec(create_migrations_table)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrations table: %w", err)
	}

	// this is horrific but I'm feeling lazy tonight
	for i, migration := range migrations {
		row := db.QueryRow("SELECT idx FROM migrations WHERE idx = ?", i)

		var idx int
		if err := row.Scan(&idx); err != nil {
			if err == sql.ErrNoRows {
				_, err := db.Exec(migration)
				if err != nil {
					return nil, fmt.Errorf("migration failed: %w", err)
				}
				_, err = db.Exec("INSERT INTO migrations (idx) VALUES (?)", i)
				if err != nil {
					return nil, fmt.Errorf("could not insert record of migration: %w", err)
				}
				continue
			}
			return nil, fmt.Errorf("could not read idx: %w", err)
		}

	}

	return &SQLiteTokenStorage{db: db}, nil
}

func (s *SQLiteTokenStorage) Get(id string) (*Token, error) {
	row := s.db.QueryRow("SELECT id, token, expiry FROM tokens WHERE id = ?", id)

	var t Token
	if err := row.Scan(&t.ID, &t.Token, &t.Expiry); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoToken
		}
		return nil, fmt.Errorf("could not retrieve token: %w", err)
	}

	return &t, nil
}

func (s *SQLiteTokenStorage) Insert(token *Token) error {
	_, err := s.db.Exec("INSERT INTO tokens (id, token, expiry) VALUES (?, ?, ?)", token.ID, token.Token, token.Expiry)
	if err != nil {
		return fmt.Errorf("could not insert token: %w", err)
	}
	return nil
}

func (s *SQLiteTokenStorage) Update(token *Token) error {
	_, err := s.db.Exec("UPDATE tokens SET token = ?, expiry = ? WHERE id = ?", token.Token, token.Expiry, token.ID)
	if err != nil {
		return fmt.Errorf("could not update token: %w", err)
	}
	return nil
}

func (s *SQLiteTokenStorage) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM tokens WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("could not delete token: %w", err)
	}
	return nil
}

func (s *SQLiteTokenStorage) Close() error {
	return s.db.Close()
}
