package store

import (
	"context"
	"fmt"
)

// QueryTable fetches the entire contents of a given table natively using SQLite column mapping.
func (s *Store) QueryTable(ctx context.Context, tableName string) ([]string, [][]string, error) {
	// Prevent SQL injection by validating tableName against known schema tables.
	if tableName != "cidr_blocks" && tableName != "ip_addresses" {
		return nil, nil, fmt.Errorf("unsupported table: %s", tableName)
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query table %s: %w", tableName, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results [][]string
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var rowData []string
		for _, col := range columns {
			if col == nil {
				rowData = append(rowData, "NULL")
			} else {
				// We can handle specific types generically if needed, but string representation works for admin tools
				switch v := col.(type) {
				case []byte:
					rowData = append(rowData, string(v))
				default:
					rowData = append(rowData, fmt.Sprintf("%v", v))
				}
			}
		}
		results = append(results, rowData)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to iterate rows: %w", err)
	}
	return cols, results, nil
}

// QueryTableByID fetches a specific record from a given table by its ID.
func (s *Store) QueryTableByID(ctx context.Context, tableName string, id string) ([]string, [][]string, error) {
	// Prevent SQL injection by validating tableName against known schema tables.
	if tableName != "cidr_blocks" && tableName != "ip_addresses" {
		return nil, nil, fmt.Errorf("unsupported table: %s", tableName)
	}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s WHERE id = ?", tableName), id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query table %s by id %s: %w", tableName, id, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results [][]string
	for rows.Next() {
		columns := make([]interface{}, len(cols))
		columnPointers := make([]interface{}, len(cols))
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var rowData []string
		for _, col := range columns {
			if col == nil {
				rowData = append(rowData, "NULL")
			} else {
				switch v := col.(type) {
				case []byte:
					rowData = append(rowData, string(v))
				default:
					rowData = append(rowData, fmt.Sprintf("%v", v))
				}
			}
		}
		results = append(results, rowData)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to iterate rows: %w", err)
	}
	return cols, results, nil
}
