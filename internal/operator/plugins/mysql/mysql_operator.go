package mysql

import (
	"agent/internal/types"
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLOperator implements MySQL database operations
type MySQLOperator struct {
	dsn       string
	backupDir string
	db        *sql.DB
}

// New creates a new MySQL operator instance
func New() *MySQLOperator {
	return &MySQLOperator{}
}

// Info returns operator information
func (m *MySQLOperator) Info() types.OperatorInfo {
	return types.OperatorInfo{
		Name:        "mysql",
		Version:     "1.0.0",
		Description: "MySQL database operations including backup, restore, and queries",
		Author:      "prajwal.p",
	}
}

// Init initializes the operator with configuration
func (m *MySQLOperator) Init(config map[string]interface{}) error {
	if dsn, ok := config["dsn"].(string); ok {
		m.dsn = dsn
	} else {
		return fmt.Errorf("dsn is required for MySQL operator")
	}

	if backupDir, ok := config["backup_dir"].(string); ok {
		m.backupDir = backupDir
	} else {
		m.backupDir = "/tmp/mysql-backups"
	}

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(m.backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Test database connection
	db, err := sql.Open("mysql", m.dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %v", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping MySQL: %v", err)
	}

	m.db = db
	return nil
}

// GetOperations returns the list of supported operations
func (m *MySQLOperator) GetOperations() []string {
	return []string{"backup", "restore", "query", "status", "list_databases", "list_tables"}
}

// Execute performs the specified operation
func (m *MySQLOperator) Execute(ctx context.Context, operation string, params map[string]interface{}) (*types.OperationResult, error) {
	result := &types.OperationResult{
		Timestamp: time.Now(),
		NodeID:    "local",
	}

	switch operation {
	case "backup":
		return m.backup(ctx, params, result)
	case "restore":
		return m.restore(ctx, params, result)
	case "query":
		return m.query(ctx, params, result)
	case "status":
		return m.status(ctx, result)
	case "list_databases":
		return m.listDatabases(ctx, result)
	case "list_tables":
		return m.listTables(ctx, params, result)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unsupported operation: %s", operation)
		return result, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// backup creates a backup of the specified database
func (m *MySQLOperator) backup(ctx context.Context, params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	database, ok := params["database"].(string)
	if !ok || database == "" {
		result.Success = false
		result.Error = "database parameter is required"
		return result, fmt.Errorf("database parameter is required")
	}

	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(m.backupDir, fmt.Sprintf("%s_%s.sql", database, timestamp))

	// Extract connection details from DSN for mysqldump
	parts := strings.Split(m.dsn, "@")
	if len(parts) != 2 {
		result.Success = false
		result.Error = "invalid DSN format"
		return result, fmt.Errorf("invalid DSN format")
	}

	userPass := strings.Split(parts[0], ":")
	if len(userPass) != 2 {
		result.Success = false
		result.Error = "invalid user:password format in DSN"
		return result, fmt.Errorf("invalid user:password format in DSN")
	}

	hostPort := strings.Split(strings.TrimPrefix(parts[1], "tcp("), ")")
	if len(hostPort) == 0 {
		result.Success = false
		result.Error = "invalid host:port format in DSN"
		return result, fmt.Errorf("invalid host:port format in DSN")
	}

	host := "localhost"
	port := "3306"
	if strings.Contains(hostPort[0], ":") {
		hostPortParts := strings.Split(hostPort[0], ":")
		host = hostPortParts[0]
		port = hostPortParts[1]
	}

	// Execute mysqldump
	cmd := exec.CommandContext(ctx, "mysqldump",
		"-h", host,
		"-P", port,
		"-u", userPass[0],
		fmt.Sprintf("-p%s", userPass[1]),
		"--single-transaction",
		"--routines",
		"--triggers",
		database,
	)

	output, err := cmd.Output()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("mysqldump failed: %v", err)
		return result, err
	}

	if err := os.WriteFile(backupFile, output, 0644); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to write backup file: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("Database %s backed up successfully", database)
	result.Data = map[string]interface{}{
		"database":    database,
		"backup_file": backupFile,
		"size":        len(output),
		"timestamp":   timestamp,
	}

	return result, nil
}

// restore restores a database from backup
func (m *MySQLOperator) restore(ctx context.Context, params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	database, ok := params["database"].(string)
	if !ok || database == "" {
		result.Success = false
		result.Error = "database parameter is required"
		return result, fmt.Errorf("database parameter is required")
	}

	backupFile, ok := params["backup_file"].(string)
	if !ok || backupFile == "" {
		result.Success = false
		result.Error = "backup_file parameter is required"
		return result, fmt.Errorf("backup_file parameter is required")
	}

	if !filepath.IsAbs(backupFile) {
		backupFile = filepath.Join(m.backupDir, backupFile)
	}

	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		result.Success = false
		result.Error = fmt.Sprintf("backup file %s does not exist", backupFile)
		return result, fmt.Errorf("backup file %s does not exist", backupFile)
	}

	// Read backup file
	content, err := os.ReadFile(backupFile)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to read backup file: %v", err)
		return result, err
	}

	// Execute restore
	_, err = m.db.ExecContext(ctx, string(content))
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to restore database: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("Database %s restored successfully from %s", database, backupFile)
	result.Data = map[string]interface{}{
		"database":    database,
		"backup_file": backupFile,
		"size":        len(content),
	}

	return result, nil
}

// query executes a SQL query
func (m *MySQLOperator) query(ctx context.Context, params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	sql, ok := params["sql"].(string)
	if !ok || sql == "" {
		result.Success = false
		result.Error = "sql parameter is required"
		return result, fmt.Errorf("sql parameter is required")
	}

	rows, err := m.db.QueryContext(ctx, sql)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("query failed: %v", err)
		return result, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to get columns: %v", err)
		return result, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to scan row: %v", err)
			return result, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	result.Success = true
	result.Message = fmt.Sprintf("Query executed successfully, returned %d rows", len(results))
	result.Data = map[string]interface{}{
		"columns": columns,
		"rows":    results,
		"count":   len(results),
	}

	return result, nil
}

// status returns MySQL server status
func (m *MySQLOperator) status(ctx context.Context, result *types.OperationResult) (*types.OperationResult, error) {
	var version string
	err := m.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to get MySQL version: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = "MySQL server is accessible"
	result.Data = map[string]interface{}{
		"version":    version,
		"status":     "connected",
		"backup_dir": m.backupDir,
	}

	return result, nil
}

// listDatabases lists all databases
func (m *MySQLOperator) listDatabases(ctx context.Context, result *types.OperationResult) (*types.OperationResult, error) {
	rows, err := m.db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to list databases: %v", err)
		return result, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var db string
		if err := rows.Scan(&db); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to scan database name: %v", err)
			return result, err
		}
		databases = append(databases, db)
	}

	result.Success = true
	result.Message = fmt.Sprintf("Found %d databases", len(databases))
	result.Data = map[string]interface{}{
		"databases": databases,
		"count":     len(databases),
	}

	return result, nil
}

// listTables lists tables in a database
func (m *MySQLOperator) listTables(ctx context.Context, params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	database, ok := params["database"].(string)
	if !ok || database == "" {
		result.Success = false
		result.Error = "database parameter is required"
		return result, fmt.Errorf("database parameter is required")
	}

	query := fmt.Sprintf("SHOW TABLES FROM %s", database)
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to list tables: %v", err)
		return result, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to scan table name: %v", err)
			return result, err
		}
		tables = append(tables, table)
	}

	result.Success = true
	result.Message = fmt.Sprintf("Found %d tables in database %s", len(tables), database)
	result.Data = map[string]interface{}{
		"database": database,
		"tables":   tables,
		"count":    len(tables),
	}

	return result, nil
}

// GetOperationSchema returns the schema for all operations
func (m *MySQLOperator) GetOperationSchema() types.OperatorSchema {
	info := m.Info()

	return types.OperatorSchema{
		Name:        info.Name,
		Version:     info.Version,
		Description: info.Description,
		Author:      info.Author,
		Operations: map[string]types.OperationSchema{
			"backup": {
				Description: "Create a backup of the specified database",
				Parameters: map[string]types.ParameterSchema{
					"database": {
						Type:        "string",
						Required:    true,
						Description: "Name of the database to backup",
						Example:     "myapp_production",
					},
				},
				Examples: []map[string]interface{}{
					{"database": "myapp_production"},
					{"database": "analytics_db"},
				},
			},
			"restore": {
				Description: "Restore a database from backup",
				Parameters: map[string]types.ParameterSchema{
					"database": {
						Type:        "string",
						Required:    true,
						Description: "Name of the database to restore to",
						Example:     "myapp_production",
					},
					"backup_file": {
						Type:        "string",
						Required:    true,
						Description: "Path to backup file (relative to backup_dir or absolute)",
						Example:     "myapp_production_20240612_120000.sql",
					},
				},
				Examples: []map[string]interface{}{
					{
						"database":    "myapp_production",
						"backup_file": "myapp_production_20240612_120000.sql",
					},
				},
			},
			"query": {
				Description: "Execute a SQL query",
				Parameters: map[string]types.ParameterSchema{
					"sql": {
						Type:        "string",
						Required:    true,
						Description: "SQL query to execute",
						Example:     "SELECT COUNT(*) FROM users",
					},
				},
				Examples: []map[string]interface{}{
					{"sql": "SELECT COUNT(*) FROM users"},
					{"sql": "SHOW TABLES"},
					{"sql": "SELECT * FROM orders WHERE created_at > '2024-01-01' LIMIT 10"},
				},
			},
			"status": {
				Description: "Get MySQL server status and connection info",
				Parameters:  map[string]types.ParameterSchema{},
			},
			"list_databases": {
				Description: "List all databases on the MySQL server",
				Parameters:  map[string]types.ParameterSchema{},
			},
			"list_tables": {
				Description: "List tables in a specific database",
				Parameters: map[string]types.ParameterSchema{
					"database": {
						Type:        "string",
						Required:    true,
						Description: "Name of the database to list tables from",
						Example:     "myapp_production",
					},
				},
				Examples: []map[string]interface{}{
					{"database": "myapp_production"},
				},
			},
		},
	}
}

// Cleanup performs cleanup when the operator is being stopped
func (m *MySQLOperator) Cleanup() error {
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}
