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
	Verbose  bool
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

	if config.Verbose {
		log.Printf("Found %d tables to process", len(tables))
	}

	totalReplacements := 0
	for _, table := range tables {
		replacements, err := processTable(db, table, config.Search, config.Replace, config.Verbose)
		if err != nil {
			log.Printf("Error processing table %s: %v", table, err)
			continue
		}
		totalReplacements += replacements
		if replacements > 0 || config.Verbose {
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
	flag.BoolVar(&config.Verbose, "v", false, "Enable verbose output")
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

func convertToString(value interface{}) string {
	switch v := value.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func processTable(db *sql.DB, table, search, replace string, verbose bool) (int, error) {
	columns, err := getTextColumns(db, table)
	if err != nil {
		return 0, err
	}

	if verbose {
		log.Printf("  Table %s: found text columns: %v", table, columns)
	}

	if len(columns) == 0 {
		return 0, nil
	}

	rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	columnsList, err := rows.Columns()
	if err != nil {
		return 0, err
	}

	totalReplacements := 0
	rowCount := 0
	for rows.Next() {
		values := make([]interface{}, len(columnsList))
		valuePtrs := make([]interface{}, len(columnsList))
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
			for i, colName := range columnsList {
				if colName == col {
					if values[i] != nil {
						strValue := convertToString(values[i])
						newValue := strings.ReplaceAll(strValue, search, replace)
						if newValue != strValue {
							if verbose {
								log.Printf("    Found match in column %s: '%s' -> '%s'", col, strValue, newValue)
							}
							updates = append(updates, fmt.Sprintf("%s = ?", col))
							args = append(args, newValue)
							hasChanges = true
							totalReplacements++
						} else if verbose && rowCount < 3 {
							log.Printf("    No match in column %s: '%s' (searching for: '%s')", col, strValue, search)
						}
					}
					break
				}
			}
		}

		if hasChanges {
			if err := updateRow(db, table, updates, args, columnsList, values); err != nil {
				return 0, err
			}
		}
		rowCount++
	}

	if verbose {
		log.Printf("  Processed %d rows in table %s", rowCount, table)
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

func updateRow(db *sql.DB, table string, updates []string, args []interface{}, columnsList []string, values []interface{}) error {
	var whereClauses []string
	var whereArgs []interface{}

	for i, colName := range columnsList {
		if values[i] != nil {
			whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", colName))
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
