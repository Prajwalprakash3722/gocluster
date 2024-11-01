package mysql_operator

import (
	"agent/internal/types"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLOperator struct {
	dsn       string
	backupDir string
	db        *sql.DB
}

func New() types.Operator {
	return &MySQLOperator{}
}

// Info provides metadata about this operator.
func (o *MySQLOperator) Info() types.OperatorInfo {
	return types.OperatorInfo{
		Name:        "mysql",
		Version:     "1.0.0",
		Description: "MySQL database management operator",
		Author:      "mysql-author",
		Operations: map[string]types.OperationSchema{
			"health_check": {
				Description: "Checks the health of the MySQL node",
				Parameters: map[string]types.ParamSchema{
					"timeout": {
						Type:        "int",
						Required:    false,
						Default:     5,
						Description: "Timeout in seconds for health check",
					},
				},
			},
			"backup": {
				Description: "Creates a database backup",
				Parameters: map[string]types.ParamSchema{
					"database": {
						Type:        "string",
						Required:    true,
						Description: "Name of the database to back up",
					},
					"compress": {
						Type:        "bool",
						Required:    false,
						Default:     true,
						Description: "Whether to compress the backup",
					},
				},
				Config: map[string]types.ParamSchema{
					"retention_days": {
						Type:        "int",
						Required:    false,
						Default:     7,
						Description: "Number of days to retain backups",
					},
				},
			},
			"create_database": {
				Description: "Creates a new database",
				Parameters: map[string]types.ParamSchema{
					"name": {
						Type:        "string",
						Required:    true,
						Description: "Name of the database to create",
					},
					"charset": {
						Type:        "string",
						Required:    false,
						Default:     "utf8mb4",
						Description: "Character set for the database",
					},
					"collation": {
						Type:        "string",
						Required:    false,
						Default:     "utf8mb4_general_ci",
						Description: "Collation for the database",
					},
				},
			},
			"show_databases": {
				Description: "Lists all databases",
				Parameters: map[string]types.ParamSchema{
					"pattern": {
						Type:        "string",
						Required:    false,
						Description: "Pattern to filter database names",
					},
				},
			},
			"show_variables": {
				Description: "Shows MySQL system variables",
				Parameters: map[string]types.ParamSchema{
					"like": {
						Type:        "string",
						Required:    false,
						Description: "Filter variables using LIKE pattern",
					},
				},
			},
		},
	}
}

// Init initializes the operator with necessary configurations.
func (o *MySQLOperator) Init(config map[string]interface{}) error {
	// Get DSN from config
	dsn, ok := config["dsn"].(string)
	if !ok {
		return fmt.Errorf("DSN not provided in config")
	}
	o.dsn = dsn

	// Get backup directory from config or use default
	if backupDir, ok := config["backup_dir"].(string); ok {
		o.backupDir = backupDir
	} else {
		o.backupDir = "/var/lib/mysql/backups"
	}

	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(o.backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Initialize database connection
	db, err := sql.Open("mysql", o.dsn)
	if err != nil {
		return fmt.Errorf("failed to initialize database connection: %v", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect to database: %v", err)
	}

	o.db = db
	return nil
}

// Execute runs the specified MySQL operation.
func (o *MySQLOperator) Execute(ctx context.Context, params map[string]interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		operation, ok := params["operation"].(string)
		if !ok {
			return fmt.Errorf("operation not specified")
		}

		operationParams, _ := params["params"].(map[string]interface{})
		configParams, _ := params["config"].(map[string]interface{})

		switch operation {
		case "health_check":
			timeout := getIntParam(operationParams, "timeout", 5)
			return o.healthCheck(timeout)

		case "backup":
			database := getStringParam(operationParams, "database", "")
			if database == "" {
				return fmt.Errorf("database parameter is required")
			}
			compress := getBoolParam(operationParams, "compress", true)
			retentionDays := getIntParam(configParams, "retention_days", 7)
			return o.backupDatabase(database, compress, retentionDays)

		case "create_database":
			name := getStringParam(operationParams, "name", "")
			if name == "" {
				return fmt.Errorf("database name is required")
			}
			charset := getStringParam(operationParams, "charset", "utf8mb4")
			collation := getStringParam(operationParams, "collation", "utf8mb4_general_ci")
			return o.createDatabase(name, charset, collation)

		case "show_databases":
			pattern := getStringParam(operationParams, "pattern", "")
			return o.showDatabases(pattern)

		case "show_variables":
			like := getStringParam(operationParams, "like", "")
			return o.showVariables(like)

		default:
			return fmt.Errorf("unknown operation: %s", operation)
		}
	}
}

func (o *MySQLOperator) healthCheck(timeout int) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	if err := o.db.PingContext(ctx); err != nil {
		return fmt.Errorf("health check failed: %v", err)
	}

	var version string
	if err := o.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return fmt.Errorf("failed to get version: %v", err)
	}

	fmt.Printf("MySQL is healthy (Version: %s)\n", version)
	return nil
}

func (o *MySQLOperator) backupDatabase(database string, compress bool, retentionDays int) error {
	// Create timestamp-based backup filename
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(o.backupDir, fmt.Sprintf("%s_%s.sql", database, timestamp))
	if compress {
		backupFile += ".gz"
	}

	// Simulate backup for demonstration
	fmt.Printf("Backing up database '%s' to '%s'\n", database, backupFile)

	// Cleanup old backups
	return o.cleanupOldBackups(retentionDays)
}

func (o *MySQLOperator) createDatabase(name, charset, collation string) error {
	query := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET %s COLLATE %s",
		name, charset, collation,
	)

	if _, err := o.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create database: %v", err)
	}

	fmt.Printf("Database '%s' created successfully\n", name)
	return nil
}

func (o *MySQLOperator) showDatabases(pattern string) error {
	query := "SHOW DATABASES"
	if pattern != "" {
		query += fmt.Sprintf(" LIKE '%s'", pattern)
	}

	rows, err := o.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to list databases: %v", err)
	}
	defer rows.Close()

	fmt.Println("\nDatabases:")
	fmt.Println("-------------------")
	for rows.Next() {
		var dbName string
		if err := rows.Scan(&dbName); err != nil {
			return fmt.Errorf("failed to scan database name: %v", err)
		}
		fmt.Println(dbName)
	}
	return nil
}

func (o *MySQLOperator) showVariables(like string) error {
	query := "SHOW VARIABLES"
	if like != "" {
		query += fmt.Sprintf(" LIKE '%s'", like)
	}

	rows, err := o.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to get variables: %v", err)
	}
	defer rows.Close()

	fmt.Println("\nMySQL Variables:")
	fmt.Println("-------------------")
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return fmt.Errorf("failed to scan variable: %v", err)
		}
		fmt.Printf("%s = %s\n", name, value)
	}
	return nil
}

func (o *MySQLOperator) cleanupOldBackups(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	entries, err := os.ReadDir(o.backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				backupPath := filepath.Join(o.backupDir, entry.Name())
				if err := os.Remove(backupPath); err != nil {
					fmt.Printf("Warning: Failed to remove old backup %s: %v\n", entry.Name(), err)
				} else {
					fmt.Printf("Removed old backup: %s\n", entry.Name())
				}
			}
		}
	}
	return nil
}

// Rollback handles failure scenarios
func (o *MySQLOperator) Rollback(ctx context.Context) error {
	// Implementation depends on the operation being rolled back
	return nil
}

// Cleanup performs necessary cleanup
func (o *MySQLOperator) Cleanup() error {
	if o.db != nil {
		return o.db.Close()
	}
	return nil
}

// Helper functions
func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key].(string); ok {
		return val
	}
	return defaultValue
}

func getIntParam(params map[string]interface{}, key string, defaultValue int) int {
	if val, ok := params[key].(float64); ok {
		return int(val)
	}
	return defaultValue
}

func getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key].(bool); ok {
		return val
	}
	return defaultValue
}
