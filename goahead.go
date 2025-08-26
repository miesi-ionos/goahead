package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/sirupsen/logrus"
)

var (
	debug                 bool
	verbose               bool
	info                  bool
	quiet                 bool
	buildtime             string
	config                configSettings
	clusterSettings       map[string]clusterSetting
	sleepingClusterChecks map[string]clusterCheck
	checkCluster          chan clusterCheck
	startCheckerChannel   chan request
	mutex                 sync.Mutex
	clusterLoggers        map[string]*logrus.Entry
	mainLogger            *logrus.Entry
	unknownLogger         *logrus.Entry
	checkerLogger         *logrus.Entry
)

func main() {

	var (
		configFileFlag = flag.String("config", "./config.yml", "which config file to use, defaults to ./config.yml")
		versionFlag    = flag.Bool("version", false, "show build time and version number")
	)
	flag.BoolVar(&debug, "debug", false, "log debug output, defaults to false")
	flag.Parse()

	configFile := *configFileFlag
	version := *versionFlag

	if version {
		fmt.Println("goahead version 0.0.7 Build time:", buildtime, "UTC")
		os.Exit(0)
	}

	config = readConfigfile(configFile)

	clusterLoggers = make(map[string]*logrus.Entry)
	clusterSettings = make(map[string]clusterSetting)
	checkCluster = make(chan clusterCheck)
	sleepingClusterChecks = make(map[string]clusterCheck)
	startCheckerChannel = make(chan request)

	mainLogger = initLogger("goahead")
	unknownLogger = initLogger("unknown")
	checkerLogger = initLogger("checker")

	if len(config.IncludeDir) > 0 {
		if isDir(config.IncludeDir) {
			mainLogger.Debugf("Glob'ing with %s**/*.yml", config.IncludeDir)
			files := []string{}
			err := filepath.Walk(config.IncludeDir, func(path string, f os.FileInfo, err error) error {
				mainLogger.Infof("Checking for file extension on file: %s", path)
				if filepath.Ext(path) == ".yml" {
					files = append(files, path)
				}
				return nil
			})

			if len(files) == 0 {
				mainLogger.Fatalf("Could not find any cluster settings matching %s**/*.yml", config.IncludeDir)
			}
			Debugf("found potential module versions: " + strings.Join(files, " "))
			if err != nil {
				mainLogger.Fatalf("Failed to glob cluster settings include_dir with glob path %s**/*.yml Error: %v", config.IncludeDir, err)
			}
			for _, f := range files {
				readClusterSetting(f)
			}
		}

	}

	mainLogger.Info("Found following cluster settings:")
	mainLogger.Infof("%+v\n", clusterSettings)

	// check for previously create cluster state files and check if I need to restart checker
	go checkCurrentClusterStates()

	go serve()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	mainLogger.Info("GoAhead server started. Press Ctrl+C to stop.")
	
	// Wait for shutdown signal
	sig := <-sigChan
	mainLogger.Infof("Received signal %v, shutting down gracefully...", sig)
	
	// Cleanup resources
	logFileManager.CloseLogFiles()
	mainLogger.Info("GoAhead server stopped")

}
