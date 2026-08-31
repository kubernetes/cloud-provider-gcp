/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package store

import (
	"context"
	"fmt"
	"time"
)

// AdminListCIDRBlocks fetches cidr_blocks records natively formatted as
// generic text output, optionally filtered.
func (s *Store) AdminListCIDRBlocks(ctx context.Context, filter string) ([]string, [][]string, error) {
	return s.adminQueryTable(ctx, "cidr_blocks", filter)
}

// AdminListIPAddresses fetches ip_addresses records natively formatted as
// generic text output, optionally filtered.
func (s *Store) AdminListIPAddresses(ctx context.Context, filter string) ([]string, [][]string, error) {
	return s.adminQueryTable(ctx, "ip_addresses", filter)
}

// adminQueryTable fetches the entire contents of a given table natively
// using SQLite column mapping.
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

	var results [][]string
	for rows.Next() {
		columns := make([]any, len(cols))
		columnPointers := make([]any, len(cols))
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
				switch v := col.(type) {
				case []byte:
					rowData = append(rowData, string(v))
				case time.Time:
					rowData = append(rowData, v.Format(time.RFC3339))
				case int64:
					if colName == "created_at" || colName == "updated_at" || colName == "release_at" || colName == "allocated_at" {
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
