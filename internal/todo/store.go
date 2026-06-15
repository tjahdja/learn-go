package todo

import (
	"context"
	"database/sql"
	"fmt"
)

type Database interface {
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row // Fixed: returns *sql.Row, no error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type Store struct {
	DB Database
}

func NewStore(db Database) *Store {
	return &Store{
		DB: db,
	}
}

// FetchAll retrieved all todo tasks from Postgres
func (s *Store) FetchAll(ctx context.Context, id int) ([]TodoItem, error) {
	// Always use a specific column list instead of SELECT *
	query := `SELECT id, task, completed, created_at FROM todos WHERE user_id = $1 ORDER BY created_at DESC`

	// Pass the context down so the database query cancels if the client hangs up
	rows, err := s.DB.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("query failure: %w", err)
	}
	defer rows.Close() // Clean up database memory pool pointers when done

	var items []TodoItem

	// Loop through rows returned by the cursor
	for rows.Next() {
		var item TodoItem
		// Scan columns into struct destination fields in the exact query order
		err := rows.Scan(&item.ID, &item.Task, &item.Completed, &item.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("row scan failure: %w", err)
		}
		items = append(items, item)
	}

	// Double check if any internal errors occurred during streaming iteration
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration failure: %w", err)
	}

	// Return an empty array instead of 'null' if the database is blank
	if items == nil {
		items = []TodoItem{}
	}

	return items, nil
}

// internal/todo/store.go

// Insert adds a new task to the database and returns the fully populated TodoItem
func (s *Store) Insert(ctx context.Context, taskText string) (*TodoItem, error) {
	query := `
		INSERT INTO todos (task)
		VALUES ($1)
		RETURNING id, task, completed, created_at`

	var item TodoItem

	// QueryRowContext executes the insert statement and scans the RETURNING values
	// directly into our target struct fields.
	err := s.DB.QueryRowContext(ctx, query, taskText).Scan(
		&item.ID,
		&item.Task,
		&item.Completed,
		&item.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert todo item: %w", err)
	}

	return &item, nil
}

func (s *Store) UpdateStatus(ctx context.Context, id int, completed bool) (*TodoItem, error) {
	query := `
		UPDATE todos
		SET completed = $1
		WHERE id = $2
		RETURNING id, task, completed, created_at`

	var item TodoItem

	err := s.DB.QueryRowContext(ctx, query, completed, id).Scan(
		&item.ID,
		&item.Task,
		&item.Completed,
		&item.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update todo item status: %w", err)
	}

	return &item, nil
}

func (s *Store) Delete(ctx context.Context, id int) error {
	query := `DELETE FROM todos WHERE id = $1`

	result, err := s.DB.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to execute delete query: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("todo item not found")
	}

	return nil
}
