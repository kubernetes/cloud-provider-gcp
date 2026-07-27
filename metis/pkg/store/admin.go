package store

import (
	"context"
	"fmt"
	"time"
)

// AdminListCIDRBlocks fetches cidr_blocks records natively formatted as generic text output, optionally filtered.
func (s *Store) AdminListCIDRBlocks(ctx context.Context, filter string) ([]string, [][]string, error) {
	return s.adminQueryTable(ctx, "cidr_blocks", filter)
}

// AdminListIPAddresses fetches ip_addresses records natively formatted as generic text output, optionally filtered.
func (s *Store) AdminListIPAddresses(ctx context.Context, filter string) ([]string, [][]string, error) {
	return s.adminQueryTable(ctx, "ip_addresses", filter)
}

// adminQueryTable fetches the entire contents of a given table natively using SQLite column mapping.
func (s *Store) adminQueryTable(ctx context.Context, tableName string, filter string) ([]string, [][]string, error) {
	query := fmt.Sprintf("SELECT * FROM %s", tableName)
	if filter != "" {
		query = fmt.Sprintf("SELECT * FROM %s WHERE %s", tableName, filter)
	}

	rows, err := s.db.QueryContext(ctx, query)
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
