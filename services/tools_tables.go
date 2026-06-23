package services

import (
	"fmt"
	"strings"

	"go-rss-ui/database"
)

var coreTableTruncateQueries = map[string]string{
	"users": "TRUNCATE TABLE users CASCADE",
	"feeds": "TRUNCATE TABLE feeds CASCADE",
	"items": "TRUNCATE TABLE items CASCADE",
}

func ClearAllCoreTables() error {
	ensurePrimaryDatabase()

	for _, tableName := range []string{"items", "feeds", "users"} {
		if err := database.DB.Exec(coreTableTruncateQueries[tableName]).Error; err != nil {
			return err
		}
	}

	return nil
}

func ClearCoreTable(tableName string) error {
	ensurePrimaryDatabase()

	query, ok := coreTableTruncateQueries[strings.ToLower(tableName)]
	if !ok {
		return fmt.Errorf("invalid table name. allowed tables: users, feeds, items")
	}

	return database.DB.Exec(query).Error
}
