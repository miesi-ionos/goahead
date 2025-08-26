package main

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type clusterCheck struct {
	Csetting         clusterSetting
	Fqdn             string
	RequestID        string
	Cluster          string
	RebootApprovedAt time.Time
	PanicCancel      context.CancelFunc
}

func startCheckForRebootedSystemWithOffset(cc clusterCheck, req request, cs clusterSetting) {
	checkerLogger.Info("Waiting for reboot_completion_check_offset: " + cc.Csetting.RebootCompletionCheckOffset.String() + " for FQDN: " + cc.Fqdn)

	// Start panic timer if panic threshold is configured
	if cc.Csetting.RebootCompletionPanicThreshold > 0 {
		panicTime := cc.RebootApprovedAt.Add(cc.Csetting.RebootCompletionCheckOffset).Add(cc.Csetting.RebootCompletionPanicThreshold)
		ctx, cancel := context.WithCancel(context.Background())
		cc.PanicCancel = cancel

		// Update the clusterCheck in sleepingClusterChecks with the cancel function
		mutex.Lock()
		sleepingClusterChecks[cc.Fqdn] = cc
		mutex.Unlock()

		go startRebootCompletionPanicTimer(ctx, cc, req, cs, panicTime)
	}

	// Sleep for the configured offset duration
	time.Sleep(cc.Csetting.RebootCompletionCheckOffset)

	// Remove from sleeping checks since we're about to start checking
	mutex.Lock()
	delete(sleepingClusterChecks, cc.Fqdn)
	mutex.Unlock()

	// Start the actual reboot completion checking
	startCheckForRebootedSystem(cc, req, cs)
}

func startRebootCompletionPanicTimer(ctx context.Context, cc clusterCheck, req request, cs clusterSetting, panicTime time.Time) {
	clusterLogger := clusterLoggers[cc.Cluster]

	// Calculate how long to wait until panic time
	waitDuration := time.Until(panicTime)
	if waitDuration <= 0 {
		// Panic time already passed, trigger immediately
		triggerRebootCompletionPanic(cc, req, cs, clusterLogger)
		return
	}

	clusterLogger.Info("Reboot completion panic timer started for " + cc.Fqdn + " - will trigger at " + panicTime.String())

	// Wait until panic time or context cancellation
	select {
	case <-time.After(waitDuration):
		// Panic time reached
	case <-ctx.Done():
		// Context cancelled, panic timer cancelled
		clusterLogger.Info("Reboot completion panic timer cancelled for " + cc.Fqdn + " - server completed reboot successfully")
		return
	}

	// Check if the server has already completed reboot (not in CurrentRestartingServers anymore)
	mutex.Lock()
	defer mutex.Unlock()

	clusterFile := config.SaveStateDir + cc.Cluster + ".json"
	if fileExists(clusterFile) {
		currentCs := readClusterStateFile(clusterFile, cc.Cluster, clusterLogger)
		if _, stillRestarting := currentCs.CurrentRestartingServers[cc.Fqdn]; stillRestarting {
			// Server is still restarting, trigger panic
			triggerRebootCompletionPanic(cc, req, cs, clusterLogger)
		} else {
			clusterLogger.Info("Reboot completion panic timer cancelled for " + cc.Fqdn + " - server completed reboot successfully")
		}
	}
}

func triggerRebootCompletionPanic(cc clusterCheck, req request, cs clusterSetting, clusterLogger *logrus.Entry) {
	clusterLogger.Error("Reboot completion panic threshold reached for " + cc.Fqdn + " in cluster " + cc.Cluster + "!")

	// Update cluster state with panic timestamp
	mutex.Lock()
	defer mutex.Unlock()

	clusterFile := config.SaveStateDir + cc.Cluster + ".json"
	var currentCs clusterState
	if fileExists(clusterFile) {
		currentCs = readClusterStateFile(clusterFile, cc.Cluster, clusterLogger)
	}

	currentCs.LastRestartPanicTimestamp = time.Now()
	if err := writeStructJSONFile(clusterFile, currentCs); err != nil {
		clusterLogger.Error("Could not save cluster state file after panic: " + clusterFile + " " + err.Error())
	}

	// Trigger panic actions
	triggerRebootCompletionPanicActions(cc.Fqdn, cc.Cluster, req.Uptime, clusterLogger)
}

func startCheckForRebootedSystem(cc clusterCheck, req request, cs clusterSetting) {
	checkerLogger.Info("Starting check for rebooted system in cluster " + cc.Cluster + " with fqdn: " + cc.Fqdn)
	successfulChecks := 0
	for {
		command := strings.Replace(cc.Csetting.RebootCompletionCheck, "{:%fqdn%:}", cc.Fqdn, -1)
		command = strings.Replace(command, "{:%hostname%:}", cc.Fqdn, -1)
		command = strings.Replace(command, "{:%cluster%:}", cc.Fqdn, -1)
		er := executeCommand(command, 5, !cs.RaiseErrors, checkerLogger)
		checkerLogger.Info("Check result of "+command+" is ", er.returnCode)

		if er.returnCode == 0 {
			successfulChecks++
			checkerLogger.Info("Increasing successful check counter to " + strconv.Itoa(successfulChecks) + " of " + strconv.Itoa(cc.Csetting.RebootCompletionCheckConsecutiveSuccesses) + " for rebooted system in cluster " + cc.Cluster + " with fqdn: " + cc.Fqdn)
			if successfulChecks >= cc.Csetting.RebootCompletionCheckConsecutiveSuccesses {
				break
			}
		} else {
			successfulChecks = 0
		}
		//checkerLogger.Info("Sleeping for reboot_completion_check_interval: " + cc.Csetting.RebootCompletionCheckInterval.String())
		time.Sleep(cc.Csetting.RebootCompletionCheckInterval)
	}
	checkerLogger.Info("fqdn: " + cc.Fqdn + " seems to have successfully rebooted in cluster " + cc.Cluster)
	clusterLogger := clusterLoggers[cc.Cluster]
	clusterLogger.Info("fqdn: " + cc.Fqdn + " seems to have successfully rebooted in cluster " + cc.Cluster)

	// Cancel panic timer if it's running
	if cc.PanicCancel != nil {
		cc.PanicCancel()
	}

	triggerRebootCompletionActions(cc.Fqdn, cc.Cluster, req.Uptime, clusterLogger)
	//deleteAckFile(cc.Fqdn, cc.Cluster)
	// decrement current restarts for cluster
	modifyClusterState(cc.Cluster, cc.Fqdn, "remove", clusterLogger)
	res := response{}
	res.Timestamp = time.Now()
	res.RequestingFqdn = cc.Fqdn
	res.ReportedUptime = req.Uptime
	res.FoundCluster = cc.Cluster
	res.Message = "fqdn: " + cc.Fqdn + " seems to have successfully rebooted in cluster " + cc.Cluster + " at " + res.Timestamp.String()
	saveAckFile(res, clusterLogger)
}
