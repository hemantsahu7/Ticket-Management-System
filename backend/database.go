package main

import (
	"database/sql"
	"time"
)

func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL
		);
		CREATE TABLE tickets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK(status IN ('open', 'in_progress', 'closed')),
			user_id INTEGER NOT NULL REFERENCES users(id),
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX tickets_user_id_idx ON tickets(user_id);
	`)
	return err
}

type rowScanner interface{ Scan(...any) error }

func scanTicket(row rowScanner) (ticket, error) {
	var t ticket
	var createdAt, updatedAt string
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.UserID, &createdAt, &updatedAt)
	if err != nil {
		return ticket{}, err
	}
	t.CreatedAt, t.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
	return t, nil
}

func (a *app) ticketByID(id int) (ticket, error) {
	return scanTicket(a.db.QueryRow(`SELECT id, title, description, status, user_id, created_at, updated_at FROM tickets WHERE id = ?`, id))
}

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
