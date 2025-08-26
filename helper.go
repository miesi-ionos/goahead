package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/kballard/go-shellquote"
	log "github.com/sirupsen/logrus"
)

// ExecResult contains the exit code and output of an external command (e.g. git)
type ExecResult struct {
	returnCode int
	output     string
}

// Debugf is a helper function for debug logging if global variable debug is set to true
func Debugf(s string) {
	if debug != false {
		pc, _, _, _ := runtime.Caller(1)
		callingFunctionName := strings.Split(runtime.FuncForPC(pc).Name(), ".")[len(strings.Split(runtime.FuncForPC(pc).Name(), "."))-1]
		if strings.HasPrefix(callingFunctionName, "func") {
			// check for anonymous function names
			log.Printf("DEBUG %v", s)
		} else {
			log.Printf("DEBUG %s(): %v", callingFunctionName, s)
		}
	}
}

// Verbosef is a helper function for verbose logging if global variable verbose is set to true
func Verbosef(s string) {
	if debug != false || verbose != false {
		log.Printf("%v", s)
	}
}

// Infof is a helper function for info logging if global variable info is set to true
func Infof(s string) {
	if debug != false || verbose != false || info != false {
		color.Green(s)
	}
}

// Warnf is a helper function for warning logging
func Warnf(s string) {
	pc, _, _, _ := runtime.Caller(1)
	callingFunctionName := strings.Split(runtime.FuncForPC(pc).Name(), ".")[len(strings.Split(runtime.FuncForPC(pc).Name(), "."))-1]
	color.Set(color.FgYellow)
	log.Printf("WARN %s(): %v", callingFunctionName, s)
	color.Unset()
}

// Fatalf is a helper function for fatal logging
func Fatalf(s string) {
	color.New(color.FgRed).Fprintln(os.Stderr, s)
	os.Exit(1)
}

// fileExists checks if the given file exists and returns a bool
func fileExists(file string) bool {
	//Debugf("checking for file existence " + file)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return false
	}
	return true
}

// isDir checks if the given dir exists and returns a bool
func isDir(dir string) bool {
	fi, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false
	}
	if fi.Mode().IsDir() {
		return true
	}
	return false
}

// normalizeDir removes from the given directory path multiple redundant slashes and adds a trailing slash
func normalizeDir(dir string) string {
	if strings.Count(dir, "//") > 0 {
		dir = normalizeDir(strings.Replace(dir, "//", "/", -1))
	} else {
		if !strings.HasSuffix(dir, "/") {
			dir = dir + "/"
		}
	}
	return dir
}

// checkDirAndCreate tests if the given directory exists and tries to create it
func checkDirAndCreate(dir string, name string) string {
	if len(dir) != 0 {
		if !fileExists(dir) {
			//log.Printf("checkDirAndCreate(): trying to create dir '%s' as %s", dir, name){
			if err := os.MkdirAll(dir, 0777); err != nil {
				Fatalf("checkDirAndCreate(): Error: failed to create directory: " + dir)
			}
		} else {
			if !isDir(dir) {
				Fatalf("checkDirAndCreate(): Error: " + dir + " exists, but is not a directory! Exiting!")
			}
		}
	} else {
		// TODO make dir optional
		Fatalf("checkDirAndCreate(): Error: dir setting '" + name + "' missing! Exiting!")
	}
	dir = normalizeDir(dir)
	return dir
}

func createOrPurgeDir(dir string, callingFunction string) {
	if !fileExists(dir) {
		Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
		os.MkdirAll(dir, 0777)
	} else {
		Debugf("Trying to remove: " + dir + " called from " + callingFunction)
		if err := os.RemoveAll(dir); err != nil {
			log.Print("createOrPurgeDir(): error: removing dir failed", err)
		}
		Debugf("Trying to create dir: " + dir + " called from " + callingFunction)
		os.MkdirAll(dir, 0777)
	}
}

func purgeDir(dir string, callingFunction string) {
	if fileExists(dir) {
		if err := os.RemoveAll(dir); err != nil {
			log.Print("purgeDir(): os.RemoveAll() error: removing dir failed: ", err)
			if err = syscall.Unlink(dir); err != nil {
				log.Print("purgeDir(): syscall.Unlink() error: removing link failed: ", err)
			}
		}
	}
}

func executeCommand(command string, timeout int, allowFail bool, logger *log.Entry) ExecResult {
	logger.Info("Executing " + command)
	parts := strings.SplitN(command, " ", 2)
	cmd := parts[0]
	cmdArgs := []string{}
	if len(parts) > 1 {
		args, err := shellquote.Split(parts[1])
		if err != nil {
			logger.Warn("err: " + fmt.Sprint(err))
		} else {
			cmdArgs = args
		}
	}

	before := time.Now()
	out, err := exec.Command(cmd, cmdArgs...).CombinedOutput()
	duration := time.Since(before).Seconds()
	er := ExecResult{0, string(out)}
	if msg, ok := err.(*exec.ExitError); ok { // there is error code
		er.returnCode = msg.Sys().(syscall.WaitStatus).ExitStatus()
	}
	logger.Debug("Executing " + command + " took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
	if err != nil {
		if !allowFail {
			logger.Warn("executeCommand(): command failed: " + command + " " + err.Error() + "\nOutput: " + string(out))
		} else {
			er.returnCode = 1
			er.output = fmt.Sprint(err)
		}
	}
	return er
}

// funcName return the function name as a string
func funcName() string {
	pc, _, _, _ := runtime.Caller(1)
	completeFuncname := runtime.FuncForPC(pc).Name()
	return strings.Split(completeFuncname, ".")[len(strings.Split(completeFuncname, "."))-1]
}

func timeTrack(start time.Time, name string) {
	duration := time.Since(start).Seconds()
	Debugf(name + "() took " + strconv.FormatFloat(duration, 'f', 5, 64) + "s")
}

// getSha256sumFile return the SHA256 hash sum of the given file
func getSha256sumFile(file string) string {
	// https://golang.org/pkg/crypto/sha256/#New
	f, err := os.Open(file)
	if err != nil {
		Fatalf("failed to open file " + file + " to calculate SHA256 sum. Error: " + err.Error())
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		Fatalf("failed to calculate SHA256 sum of file " + file + " Error: " + err.Error())
	}

	return string(h.Sum(nil))
}

// randSeq returns a fixed length random string to identify each request in the log
// http://stackoverflow.com/a/22892986/682847
func randSeq() string {
	b := make([]rune, 8)
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	rand.Seed(time.Now().UTC().UnixNano())
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func writeStructJSONFile(file string, v interface{}) error {
	f, err := os.Create(file)
	if err != nil {
		Warnf("Could not write JSON file " + file + " " + err.Error())
		return err
	}
	defer f.Close()
	json, err := json.Marshal(v)
	if err != nil {
		Warnf("Could not encode JSON file " + file + " " + err.Error())
		return err
	}
	f.Write(json)
	return nil
}

func readClusterStateFile(file string, cluster string, clusterLogger *log.Entry) clusterState {
	clusterLogger.Debug("Trying to read json file: " + file)
	data, err := os.ReadFile(file)
	if err != nil {
		clusterLogger.Warn("readStructJSONFile(): There was an error parsing the json file " + file + ": " + err.Error())
	}

	var cs clusterState
	err = json.Unmarshal([]byte(data), &cs)
	if err != nil {
		clusterLogger.Warn("In json file " + file + ": JSON unmarshal error: " + err.Error())
	}
	return cs
}

func readAckFile(file string, res response, cluster string, clusterLogger *log.Entry) response {
	clusterLogger.Debug("Trying to read json file: " + file)
	data, err := os.ReadFile(file)
	if err != nil {
		clusterLogger.Warn("readStructJSONFile(): There was an error parsing the json file " + file + ": " + err.Error())
	}

	err = json.Unmarshal([]byte(data), &res)
	if err != nil {
		clusterLogger.Warn("In json file " + file + ": JSON unmarshal error: " + err.Error())
	}
	return res
}

func keysString(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// LogFileManager manages log files and their lifecycle
type LogFileManager struct {
	files map[string]*os.File
}

var logFileManager = &LogFileManager{
	files: make(map[string]*os.File),
}

// CloseLogFiles closes all open log files - call during shutdown
func (lfm *LogFileManager) CloseLogFiles() {
	for name, file := range lfm.files {
		if err := file.Close(); err != nil {
			log.Errorf("Error closing log file %s: %v", name, err)
		}
	}
	lfm.files = make(map[string]*os.File)
}

// parseLogSize converts size strings like "100M", "1G" to bytes
func parseLogSize(sizeStr string) (int64, error) {
	re := regexp.MustCompile(`^(\d+)([KMGT]?)[Bb]?$`)
	matches := re.FindStringSubmatch(strings.ToUpper(sizeStr))
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid size format: %s (expected format: 100M, 1G, etc.)", sizeStr)
	}

	size, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size number: %s", matches[1])
	}

	unit := matches[2]
	switch unit {
	case "K":
		return size * 1024, nil
	case "M":
		return size * 1024 * 1024, nil
	case "G":
		return size * 1024 * 1024 * 1024, nil
	case "T":
		return size * 1024 * 1024 * 1024 * 1024, nil
	case "":
		return size, nil // bytes
	default:
		return 0, fmt.Errorf("unknown size unit: %s", unit)
	}
}

// rotateLogFile performs log rotation with timestamp-based naming and retention management
func rotateLogFile(logFilePath string) error {
	maxSizeBytes, err := parseLogSize(config.LogMaxSize)
	if err != nil {
		log.Errorf("Invalid log_max_size configuration: %v", err)
		return err
	}

	// Check if rotation is needed
	stat, err := os.Stat(logFilePath)
	if err != nil {
		return nil // File doesn't exist, no rotation needed
	}

	if stat.Size() < maxSizeBytes {
		return nil // File not large enough for rotation
	}

	// Create timestamp for rotated file
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	rotatedPath := fmt.Sprintf("%s.%s", logFilePath, timestamp)

	// Rotate the current log file
	if err := os.Rename(logFilePath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rotate log file %s: %v", logFilePath, err)
	}

	log.Infof("Rotated log file %s to %s", logFilePath, rotatedPath)

	// Clean up old log files if enabled
	if config.DeleteOldLogFiles {
		if err := cleanupOldLogFiles(logFilePath); err != nil {
			log.Warnf("Failed to cleanup old log files: %v", err)
		}
	}

	return nil
}

// cleanupOldLogFiles removes rotated log files beyond the configured retention count
func cleanupOldLogFiles(baseLogPath string) error {
	logDir := filepath.Dir(baseLogPath)
	logBaseName := filepath.Base(baseLogPath)
	
	// Find all rotated log files for this base log
	pattern := fmt.Sprintf("%s.*", logBaseName)
	files, err := filepath.Glob(filepath.Join(logDir, pattern))
	if err != nil {
		return fmt.Errorf("failed to glob log files: %v", err)
	}

	// Filter out the current log file and extract only rotated files with timestamps
	var rotatedFiles []string
	timestampPattern := regexp.MustCompile(`\.\d{4}-\d{2}-\d{2}_\d{2}-\d{2}-\d{2}$`)
	
	for _, file := range files {
		if file != baseLogPath && timestampPattern.MatchString(file) {
			rotatedFiles = append(rotatedFiles, file)
		}
	}

	// Sort by filename (which includes timestamp) - newest first
	sort.Sort(sort.Reverse(sort.StringSlice(rotatedFiles)))

	// Keep only the configured number of rotated files
	if len(rotatedFiles) > config.LogRotationCount {
		filesToDelete := rotatedFiles[config.LogRotationCount:]
		for _, file := range filesToDelete {
			if err := os.Remove(file); err != nil {
				log.Warnf("Failed to delete old log file %s: %v", file, err)
			} else {
				log.Infof("Deleted old log file: %s", file)
			}
		}
	}

	return nil
}

func initLogger(fileName string) *log.Entry {
	logFilePath := filepath.Join(config.LogBaseDir, fileName+".log")
	log.Debugf("Setting up log file: %s", logFilePath)
	
	// Perform log rotation if needed based on configured parameters
	if err := rotateLogFile(logFilePath); err != nil {
		log.Errorf("Log rotation failed: %v", err)
	}
	
	var logrusLog = log.New()
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to log to file %s: %v", logFilePath, err)
	}
	
	// Store file handle for proper cleanup later
	logFileManager.files[fileName] = file
	logrusLog.Out = file
	
	if debug {
		logrusLog.SetLevel(log.DebugLevel)
	}
	logger := logrusLog.WithFields(log.Fields{})
	return logger
}

func compareDurationString(a string, b string) string {
	da, err := time.ParseDuration(a)
	if err != nil {
		Warnf("Can not convert value " + a + " of your uptime to a golang Duration. Valid time units are 300ms, 1.5h or 2h45m.")
	}
	db, err := time.ParseDuration(b)
	if err != nil {
		Warnf("Can not convert value " + b + " of your uptime to a golang Duration. Valid time units are 300ms, 1.5h or 2h45m.")
	}

	if da.Seconds() < db.Seconds() {
		return "shorter"
	}
	return "longer"
}
