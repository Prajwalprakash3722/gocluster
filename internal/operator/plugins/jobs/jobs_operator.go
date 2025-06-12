package jobs

import (
	"agent/internal/types"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// JobsOperator implements job execution operations
type JobsOperator struct {
	workingDir string
	timeout    time.Duration
	logDir     string
	executions map[string]*types.JobExecution
}

// New creates a new jobs operator instance
func New() *JobsOperator {
	return &JobsOperator{
		executions: make(map[string]*types.JobExecution),
	}
}

// Info returns operator information
func (j *JobsOperator) Info() types.OperatorInfo {
	return types.OperatorInfo{
		Name:        "jobs",
		Version:     "1.0.0",
		Description: "Job execution operator for running scripts and commands",
		Author:      "prajwal.p",
	}
}

// Init initializes the operator with configuration
func (j *JobsOperator) Init(config map[string]interface{}) error {
	if workingDir, ok := config["working_dir"].(string); ok {
		j.workingDir = workingDir
	} else {
		j.workingDir = "/tmp"
	}

	if timeoutStr, ok := config["timeout"].(string); ok {
		timeout, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return fmt.Errorf("invalid timeout format: %v", err)
		}
		j.timeout = timeout
	} else {
		j.timeout = 30 * time.Minute
	}

	if logDir, ok := config["log_dir"].(string); ok {
		j.logDir = logDir
	} else {
		j.logDir = "/tmp/gocluster-jobs"
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(j.logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	return nil
}

// GetOperations returns the list of supported operations
func (j *JobsOperator) GetOperations() []string {
	return []string{"execute_job", "validate_job", "list_executions", "get_execution", "list_logs", "get_log"}
}

// Execute performs the specified operation
func (j *JobsOperator) Execute(ctx context.Context, operation string, params map[string]interface{}) (*types.OperationResult, error) {
	result := &types.OperationResult{
		Timestamp: time.Now(),
		NodeID:    "local",
	}

	switch operation {
	case "execute_job":
		return j.executeJob(ctx, params, result)
	case "validate_job":
		return j.validateJob(params, result)
	case "list_executions":
		return j.listExecutions(result)
	case "get_execution":
		return j.getExecution(params, result)
	case "list_logs":
		return j.listLogs(params, result)
	case "get_log":
		return j.getLog(params, result)
	default:
		result.Success = false
		result.Error = fmt.Sprintf("unsupported operation: %s", operation)
		return result, fmt.Errorf("unsupported operation: %s", operation)
	}
}

// executeJob executes a job from YAML definition
func (j *JobsOperator) executeJob(ctx context.Context, params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	jobYAML, ok := params["job_yaml"].(string)
	if !ok || jobYAML == "" {
		result.Success = false
		result.Error = "job_yaml parameter is required"
		return result, fmt.Errorf("job_yaml parameter is required")
	}

	// Parse job YAML
	var job types.Job
	if err := yaml.Unmarshal([]byte(jobYAML), &job); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to parse job YAML: %v", err)
		return result, err
	}

	// Validate job
	if err := j.validateJobStruct(&job); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("job validation failed: %v", err)
		return result, err
	}

	// Create execution record
	execution := &types.JobExecution{
		ID:          uuid.New().String(),
		JobName:     job.Name,
		Status:      "running",
		StartTime:   time.Now(),
		Logs:        []string{},
		StepResults: make(map[string]interface{}),
	}

	j.executions[execution.ID] = execution

	// Execute job asynchronously
	go j.runJob(ctx, &job, execution, params)

	result.Success = true
	result.Message = fmt.Sprintf("Job %s started with execution ID %s", job.Name, execution.ID)
	result.Data = map[string]interface{}{
		"execution_id": execution.ID,
		"job_name":     job.Name,
		"status":       execution.Status,
		"start_time":   execution.StartTime,
	}

	return result, nil
}

// runJob executes the job steps
func (j *JobsOperator) runJob(ctx context.Context, job *types.Job, execution *types.JobExecution, params map[string]interface{}) {
	defer func() {
		endTime := time.Now()
		execution.EndTime = &endTime
		if execution.Status == "running" {
			execution.Status = "completed"
		}
	}()

	// Set up working directory
	workingDir := j.workingDir
	if job.WorkingDir != "" {
		workingDir = job.WorkingDir
	}

	// Set up timeout
	timeout := j.timeout
	if job.Timeout != "" {
		if t, err := time.ParseDuration(job.Timeout); err == nil {
			timeout = t
		}
	}

	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute each step
	for i, step := range job.Steps {
		stepName := step.Name
		if stepName == "" {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		execution.Logs = append(execution.Logs, fmt.Sprintf("Starting step: %s", stepName))

		stepResult, err := j.executeStep(jobCtx, &step, workingDir, job.Environment)

		execution.StepResults[stepName] = stepResult

		if err != nil {
			execution.Logs = append(execution.Logs, fmt.Sprintf("Step %s failed: %v", stepName, err))

			if !step.ContinueOnError && job.OnFailure != "continue" {
				execution.Status = "failed"
				execution.Result = &types.OperationResult{
					Success:   false,
					Message:   fmt.Sprintf("Job failed at step: %s", stepName),
					Error:     err.Error(),
					Timestamp: time.Now(),
				}
				return
			}
		} else {
			execution.Logs = append(execution.Logs, fmt.Sprintf("Step %s completed successfully", stepName))
		}
	}

	execution.Status = "completed"
	execution.Result = &types.OperationResult{
		Success:   true,
		Message:   fmt.Sprintf("Job %s completed successfully", job.Name),
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"execution_id": execution.ID,
			"job_name":     job.Name,
			"steps_count":  len(job.Steps),
		},
	}
}

// executeStep executes a single job step
func (j *JobsOperator) executeStep(ctx context.Context, step *types.JobStep, workingDir string, jobEnv map[string]string) (map[string]interface{}, error) {
	// Set up step working directory
	stepWorkingDir := workingDir
	if step.WorkingDir != "" {
		stepWorkingDir = step.WorkingDir
	}

	// Set up environment
	env := os.Environ()

	// Add job environment
	for k, v := range jobEnv {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Add step environment
	for k, v := range step.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Set up timeout
	stepCtx := ctx
	if step.Timeout != "" {
		if timeout, err := time.ParseDuration(step.Timeout); err == nil {
			var cancel context.CancelFunc
			stepCtx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
	}

	var cmd *exec.Cmd
	var output []byte
	var err error

	startTime := time.Now()

	if step.Script != "" {
		// Execute script
		scriptFile := filepath.Join(stepWorkingDir, fmt.Sprintf("step_%s.sh", uuid.New().String()))
		if err := ioutil.WriteFile(scriptFile, []byte(step.Script), 0755); err != nil {
			return nil, fmt.Errorf("failed to write script file: %v", err)
		}
		defer os.Remove(scriptFile)

		cmd = exec.CommandContext(stepCtx, "/bin/bash", scriptFile)
	} else if step.Command != "" {
		// Execute command
		if len(step.Args) > 0 {
			cmd = exec.CommandContext(stepCtx, step.Command, step.Args...)
		} else {
			cmd = exec.CommandContext(stepCtx, step.Command)
		}
	} else {
		return nil, fmt.Errorf("either script or command must be specified")
	}

	cmd.Dir = stepWorkingDir
	cmd.Env = env

	output, err = cmd.CombinedOutput()

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	result := map[string]interface{}{
		"start_time": startTime,
		"end_time":   endTime,
		"duration":   duration.String(),
		"output":     string(output),
		"success":    err == nil,
	}

	if err != nil {
		result["error"] = err.Error()
		if exitError, ok := err.(*exec.ExitError); ok {
			result["exit_code"] = exitError.ExitCode()
		}
	} else {
		result["exit_code"] = 0
	}

	return result, err
}

// validateJob validates a job YAML structure
func (j *JobsOperator) validateJob(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	jobYAML, ok := params["job_yaml"].(string)
	if !ok || jobYAML == "" {
		result.Success = false
		result.Error = "job_yaml parameter is required"
		return result, fmt.Errorf("job_yaml parameter is required")
	}

	var job types.Job
	if err := yaml.Unmarshal([]byte(jobYAML), &job); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to parse job YAML: %v", err)
		return result, err
	}

	if err := j.validateJobStruct(&job); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("job validation failed: %v", err)
		return result, err
	}

	result.Success = true
	result.Message = fmt.Sprintf("Job %s is valid", job.Name)
	result.Data = map[string]interface{}{
		"job_name":    job.Name,
		"description": job.Description,
		"steps_count": len(job.Steps),
		"working_dir": job.WorkingDir,
		"timeout":     job.Timeout,
	}

	return result, nil
}

// validateJobStruct validates the job structure
func (j *JobsOperator) validateJobStruct(job *types.Job) error {
	if job.Name == "" {
		return fmt.Errorf("job name is required")
	}

	if len(job.Steps) == 0 {
		return fmt.Errorf("at least one step is required")
	}

	for i, step := range job.Steps {
		if step.Script == "" && step.Command == "" {
			return fmt.Errorf("step %d: either script or command is required", i+1)
		}
		if step.Script != "" && step.Command != "" {
			return fmt.Errorf("step %d: cannot specify both script and command", i+1)
		}
	}

	return nil
}

// listExecutions lists all job executions
func (j *JobsOperator) listExecutions(result *types.OperationResult) (*types.OperationResult, error) {
	executions := make([]map[string]interface{}, 0, len(j.executions))

	for _, execution := range j.executions {
		execSummary := map[string]interface{}{
			"id":         execution.ID,
			"job_name":   execution.JobName,
			"status":     execution.Status,
			"start_time": execution.StartTime,
		}

		if execution.EndTime != nil {
			execSummary["end_time"] = *execution.EndTime
			execSummary["duration"] = execution.EndTime.Sub(execution.StartTime).String()
		}

		executions = append(executions, execSummary)
	}

	result.Success = true
	result.Message = fmt.Sprintf("Found %d job executions", len(executions))
	result.Data = map[string]interface{}{
		"executions": executions,
		"count":      len(executions),
	}

	return result, nil
}

// getExecution gets details of a specific job execution
func (j *JobsOperator) getExecution(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	executionID, ok := params["execution_id"].(string)
	if !ok || executionID == "" {
		result.Success = false
		result.Error = "execution_id parameter is required"
		return result, fmt.Errorf("execution_id parameter is required")
	}

	execution, exists := j.executions[executionID]
	if !exists {
		result.Success = false
		result.Error = fmt.Sprintf("execution %s not found", executionID)
		return result, fmt.Errorf("execution %s not found", executionID)
	}

	result.Success = true
	result.Message = fmt.Sprintf("Execution %s details", executionID)
	result.Data = map[string]interface{}{
		"execution": execution,
	}

	return result, nil
}

// listLogs lists available log files
func (j *JobsOperator) listLogs(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	files, err := ioutil.ReadDir(j.logDir)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to read log directory: %v", err)
		return result, err
	}

	var logFiles []map[string]interface{}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".log") {
			logFiles = append(logFiles, map[string]interface{}{
				"name":    file.Name(),
				"size":    file.Size(),
				"modtime": file.ModTime(),
			})
		}
	}

	// Apply limit if specified
	limit := len(logFiles)
	if l, ok := params["limit"].(int); ok && l > 0 && l < len(logFiles) {
		limit = l
		logFiles = logFiles[:limit]
	}

	result.Success = true
	result.Message = fmt.Sprintf("Found %d log files", len(logFiles))
	result.Data = map[string]interface{}{
		"log_files": logFiles,
		"count":     len(logFiles),
		"log_dir":   j.logDir,
	}

	return result, nil
}

// getLog gets the content of a specific log file
func (j *JobsOperator) getLog(params map[string]interface{}, result *types.OperationResult) (*types.OperationResult, error) {
	logFile, ok := params["log_file"].(string)
	if !ok || logFile == "" {
		result.Success = false
		result.Error = "log_file parameter is required"
		return result, fmt.Errorf("log_file parameter is required")
	}

	logPath := filepath.Join(j.logDir, logFile)
	if !strings.HasPrefix(logPath, j.logDir) {
		result.Success = false
		result.Error = "invalid log file path"
		return result, fmt.Errorf("invalid log file path")
	}

	content, err := ioutil.ReadFile(logPath)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to read log file: %v", err)
		return result, err
	}

	// Apply tail if specified
	lines := strings.Split(string(content), "\n")
	if tail, ok := params["tail"].(int); ok && tail > 0 && tail < len(lines) {
		lines = lines[len(lines)-tail:]
	}

	result.Success = true
	result.Message = fmt.Sprintf("Log file %s content", logFile)
	result.Data = map[string]interface{}{
		"log_file": logFile,
		"content":  strings.Join(lines, "\n"),
		"lines":    len(lines),
	}

	return result, nil
}

// GetOperationSchema returns the schema for all operations
func (j *JobsOperator) GetOperationSchema() types.OperatorSchema {
	info := j.Info()

	return types.OperatorSchema{
		Name:        info.Name,
		Version:     info.Version,
		Description: info.Description,
		Author:      info.Author,
		Operations: map[string]types.OperationSchema{
			"run": {
				Description: "Execute a job from YAML configuration",
				Parameters: map[string]types.ParameterSchema{
					"job_file": {
						Type:        "string",
						Required:    true,
						Description: "Path to job YAML file",
						Example:     "/path/to/job.yaml",
					},
				},
				Examples: []map[string]interface{}{
					{"job_file": "/jobs/backup.yaml"},
					{"job_file": "/jobs/deploy.yaml"},
				},
			},
			"execute": {
				Description: "Execute a single command directly",
				Parameters: map[string]types.ParameterSchema{
					"command": {
						Type:        "string",
						Required:    true,
						Description: "Command to execute",
						Example:     "ls -la /var/log",
					},
					"working_dir": {
						Type:        "string",
						Required:    false,
						Description: "Working directory for command execution",
						Example:     "/tmp",
					},
					"timeout": {
						Type:        "string",
						Required:    false,
						Default:     "30m",
						Description: "Timeout duration (e.g., '5m', '1h')",
						Example:     "10m",
					},
				},
				Examples: []map[string]interface{}{
					{"command": "ls -la"},
					{"command": "df -h", "timeout": "5m"},
					{"command": "ps aux", "working_dir": "/tmp"},
				},
			},
			"list": {
				Description: "List all job executions",
				Parameters:  map[string]types.ParameterSchema{},
			},
			"status": {
				Description: "Get status of a specific job execution",
				Parameters: map[string]types.ParameterSchema{
					"execution_id": {
						Type:        "string",
						Required:    true,
						Description: "Job execution ID",
						Example:     "550e8400-e29b-41d4-a716-446655440000",
					},
				},
				Examples: []map[string]interface{}{
					{"execution_id": "550e8400-e29b-41d4-a716-446655440000"},
				},
			},
			"logs": {
				Description: "Get logs for a specific job execution",
				Parameters: map[string]types.ParameterSchema{
					"execution_id": {
						Type:        "string",
						Required:    true,
						Description: "Job execution ID",
						Example:     "550e8400-e29b-41d4-a716-446655440000",
					},
				},
				Examples: []map[string]interface{}{
					{"execution_id": "550e8400-e29b-41d4-a716-446655440000"},
				},
			},
			"info": {
				Description: "Get operator information and configuration",
				Parameters:  map[string]types.ParameterSchema{},
			},
		},
	}
}

// Cleanup performs cleanup when the operator is being stopped
func (j *JobsOperator) Cleanup() error {
	// Clean up any running executions
	for _, execution := range j.executions {
		if execution.Status == "running" {
			execution.Status = "cancelled"
			endTime := time.Now()
			execution.EndTime = &endTime
		}
	}
	return nil
}
