# mysqlreplace

A CLI tool for performing bulk search-and-replace operations across all tables in a MySQL database. Connects to your database, scans all tables and their text columns, and replaces specified strings throughout your data.

## Features

- Scans all tables in a MySQL database
- Automatically identifies text-based columns (CHAR, VARCHAR, TEXT types)
- Performs row-by-row replacements with detailed logging
- Reports total replacements made per table
- Safe handling of NULL values

## Requirements

- Go 1.20 or higher
- MySQL database access

## Installation

### Build from source

```bash
git clone https://github.com/wltechblog/mysqlreplace.git
cd mysqlreplace
go build -o mysqlreplace main.go
```

## Usage

```bash
./mysqlreplace [OPTIONS]
```

### Required Flags

- `-user string` - MySQL username
- `-database string` - Database name
- `-search string` - String to search for

### Optional Flags

- `-host string` - MySQL host (default: "localhost")
- `-port int` - MySQL port (default: 3306)
- `-password string` - MySQL password (default: empty)
- `-replace string` - String to replace with (default: empty)

## Examples

Basic usage with password:

```bash
./mysqlreplace -user root -password secret -database myapp -search "old.domain.com" -replace "new.domain.com"
```

With custom host and port:

```bash
./mysqlreplace -user admin -password pass123 -host db.example.com -port 3307 -database production -search "http://" -replace "https://"
```

Delete a specific string (replace with empty):

```bash
./mysqlreplace -user root -database myapp -search "deprecated phrase" -replace ""
```

## How It Works

1. Connects to the specified MySQL database
2. Retrieves a list of all tables
3. For each table:
   - Analyzes table structure to identify text columns
   - Iterates through all rows
   - Checks each text column for the search string
   - Updates rows where replacements are needed
4. Reports total replacements made per table and overall

## Safety Notes

- Always backup your database before running bulk replacements
- Test with a subset of data first if possible
- The tool performs updates row-by-row with WHERE clauses matching original values
- NULL values are preserved and not modified

## License

GPL-2.0
