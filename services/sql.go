package services

import "go-rss-ui/database"

type SQLQueryResult struct {
	Columns []string
	Rows    []map[string]interface{}
}

func RunSQLQuery(sqlQuery string) (SQLQueryResult, error) {
	ensurePrimaryDatabase()

	rows, err := database.DB.Raw(sqlQuery).Rows()
	if err != nil {
		return SQLQueryResult{}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return SQLQueryResult{}, err
	}

	result := SQLQueryResult{
		Columns: columns,
		Rows:    []map[string]interface{}{},
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return SQLQueryResult{}, err
		}

		row := make(map[string]interface{}, len(columns))
		for i, column := range columns {
			value := values[i]
			if bytesValue, ok := value.([]byte); ok {
				row[column] = string(bytesValue)
			} else {
				row[column] = value
			}
		}

		result.Rows = append(result.Rows, row)
	}

	if err := rows.Err(); err != nil {
		return SQLQueryResult{}, err
	}

	return result, nil
}
