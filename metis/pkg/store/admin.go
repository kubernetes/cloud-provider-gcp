package store

import (
	"context"
	"fmt"
	"time"
)

// AdminListCIDRBlocks fetches all cidr_blocks records natively formatted as generic text output.
func (s *Store) AdminListCIDRBlocks(ctx context.Context) ([]string, [][]string, error) {
	return s.adminQueryTable(ctx, "cidr_blocks")
}

// AdminGetCIDRBlock fetches a specific cidr_blocks record by ID natively formatted as generic text output.
func (s *Store) AdminGetCIDRBlock(ctx context.Context, id string) ([]string, [][]string, error) {
	return s.adminQueryTableByID(ctx, "cidr_blocks", id)
}

// AdminListIPAddresses fetches all ip_addresses records natively formatted as generic text output.
func (s *Store) AdminListIPAddresses(ctx context.Context) ([]string, [][]string, error) {
	return s.adminQueryTable(ctx, "ip_addresses")
}

// AdminGetIPAddress fetches a specific ip_addresses record by ID natively formatted as generic text output.
func (s *Store) AdminGetIPAddress(ctx context.Context, id string) ([]string, [][]string, error) {
	return s.adminQueryTableByID(ctx, "ip_addresses", id)
}

// adminQueryTable fetches the entire contents of a given table natively using SQLite column mapping.
func (s *Store) adminQueryTable(ctx context.Context, tableName string) ([]string, [][]string, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", tableName))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query table %s: %w", tableName, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get columns: %w", err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get column types: %w", err)
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
		for i, col := range columns {
			if col == nil {
				rowData = append(rowData, "NULL")
			} else {
				colName := cols[i]
				colType := colTypes[i].DatabaseTypeName()
				switch v := col.(type) {
				case []byte:
					rowData = append(rowData, string(v))
				case time.Time:
					rowData = append(rowData, v.Format(time.RFC3339))
				case int64:
					if colType == "TIMESTAMP" || colType == "DATETIME" || colName == "created_at" || colName == "updated_at" || colName == "release_at" || colName == "allocated_at" {
						t := time.UnixMilli(v)
						rowData = append(rowData, t.Format(time.RFC3339))
					} else {
						rowData = append(rowData, fmt.Sprintf("%v", v))
					}
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

// adminQueryTableByID fetches a specific record from a given table by its ID.
func (s *Store) adminQueryTableByID(ctx context.Context, tableName string, id string) ([]string, [][]string, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s WHERE id = ?", tableName), id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query table %s by id %s: %w", tableName, id, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get columns: %w", err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get column types: %w", err)
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
		for i, col := range columns {
			if col == nil {
				rowData = append(rowData, "NULL")
			} else {
				colName := cols[i]
				colType := colTypes[i].DatabaseTypeName()
				switch v := col.(type) {
				case []byte:
					rowData = append(rowData, string(v))
				case time.Time:
					rowData = append(rowData, v.Format(time.RFC3339))
				case int64:
					if colType == "TIMESTAMP" || colType == "DATETIME" || colName == "created_at" || colName == "updated_at" || colName == "release_at" || colName == "allocated_at" {
						t := time.UnixMilli(v)
						rowData = append(rowData, t.Format(time.RFC3339))
					} else {
						rowData = append(rowData, fmt.Sprintf("%v", v))
					}
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
