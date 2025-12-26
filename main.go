package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Search   string
	Replace  string
}

func main() {
	config := parseFlags()

	db, err := connectDB(config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	tables, err := getTables(db)
	if err != nil {
		log.Fatalf("Failed to get tables: %v", err)
	}

	totalReplacements := 0
	for _, table := range tables {
		replacements, err := processTable(db, table, config.Search, config.Replace)
		if err != nil {
			log.Printf("Error processing table %s: %v", table, err)
			continue
		}
		totalReplacements += replacements
		if replacements > 0 {
			log.Printf("Table %s: %d replacements", table, replacements)
		}
	}

	log.Printf("Total replacements: %d", totalReplacements)
}

func parseFlags() Config {
	config := Config{}
	flag.StringVar(&config.Host, "host", "localhost", "MySQL host")
	flag.IntVar(&config.Port, "port", 3306, "MySQL port")
	flag.StringVar(&config.User, "user", "", "MySQL user")
	flag.StringVar(&config.Password, "password", "", "MySQL password")
	flag.StringVar(&config.Database, "database", "", "Database name")
	flag.StringVar(&config.Search, "search", "", "String to search for")
	flag.StringVar(&config.Replace, "replace", "", "String to replace with")
	flag.Parse()

	if config.User == "" || config.Database == "" || config.Search == "" {
		log.Fatal("-user, -database, and -search are required")
	}

	return config
}

func connectDB(config Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", config.User, config.Password, config.Host, config.Port, config.Database)
	return sql.Open("mysql", dsn)
}

func getTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SHOW TABLES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}

	return tables, nil
}

func processTable(db *sql.DB, table, search, replace string) (int, error) {
	columns, err := getTextColumns(db, table)
	if err != nil {
		return 0, err
	}

	if len(columns) == 0 {
		return 0, nil
	}

	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return 0, err
	}

	totalReplacements := 0
	for rows.Next() {
		values := make([]interface{}, len(columnTypes))
		valuePtrs := make([]interface{}, len(columnTypes))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return 0, err
		}

		var updates []string
		var args []interface{}
		hasChanges := false

		for _, col := range columns {
			for i, ct := range columnTypes {
				if ct.Name() == col {
					if values[i] != nil {
						strValue := fmt.Sprintf("%v", values[i])
						newValue := strings.ReplaceAll(strValue, search, replace)
						if newValue != strValue {
							updates = append(updates, fmt.Sprintf("%s = ?", col))
							args = append(args, newValue)
							hasChanges = true
							totalReplacements++
						}
					}
					break
				}
			}
		}

		if hasChanges {
			if err := updateRow(db, table, updates, args, columnTypes, values); err != nil {
				return 0, err
			}
		}
	}

	return totalReplacements, nil
}

func getTextColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("DESCRIBE %s", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var field string
		var typ string
		var null string
		var key string
		var defaultVal interface{}
		var extra string
		if err := rows.Scan(&field, &typ, &null, &key, &defaultVal, &extra); err != nil {
			return nil, err
		}

		isText := strings.Contains(strings.ToLower(typ), "char") ||
			strings.Contains(strings.ToLower(typ), "text") ||
			strings.Contains(strings.ToLower(typ), "varchar")
		if isText {
			columns = append(columns, field)
		}
	}

	return columns, nil
}

func updateRow(db *sql.DB, table string, updates []string, args []interface{}, columnTypes []*sql.ColumnType, values []interface{}) error {
	var whereClauses []string
	var whereArgs []interface{}

	for i, ct := range columnTypes {
		if values[i] != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", ct.Name()))
			whereArgs = append(whereArgs, values[i])
		}
	}

	if len(whereClauses) == 0 {
		return fmt.Errorf("no valid WHERE clauses found")
	}

	allArgs := append(args, whereArgs...)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s", table, strings.Join(updates, ", "), strings.Join(whereClauses, " AND "))

	_, err := db.Exec(query, allArgs...)
	return err
}
