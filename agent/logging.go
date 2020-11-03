package agent

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/agent/util"
	"github.com/evergreen-ci/evergreen/apimodels"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/rest/client"
	"github.com/evergreen-ci/pail"
	"github.com/mongodb/grip"
	"github.com/mongodb/grip/level"
	"github.com/mongodb/grip/send"
	"github.com/pkg/errors"
)

const (
	taskLogDirectory  = "evergreen-logs"
	agentLogFileName  = "agent.log"
	systemLogFileName = "system.log"
	taskLogFileName   = "task.log"
)

var (
	idSource chan int
)

func init() {
	idSource = make(chan int, 100)

	go func() {
		id := 0
		for {
			idSource <- id
			id++
		}
	}()
}

func getInc() int { return <-idSource }

// GetSender configures the agent's local logging to a file.
func (a *Agent) GetSender(ctx context.Context, prefix string) (send.Sender, error) {
	var (
		err     error
		sender  send.Sender
		senders []send.Sender
	)

	if os.Getenv(util.MarkerAgentPID) == "" { // this var is set if the agent is started via a command
		if token := a.opts.SetupData.SplunkClientToken; token != "" {
			info := send.SplunkConnectionInfo{
				ServerURL: a.opts.SetupData.SplunkServerURL,
				Token:     a.opts.SetupData.SplunkClientToken,
				Channel:   a.opts.SetupData.SplunkChannel,
			}
			grip.Info("configuring splunk sender")
			sender, err = send.NewSplunkLogger("evergreen.agent", info, send.LevelInfo{Default: level.Alert, Threshold: level.Alert})
			if err != nil {
				return nil, errors.Wrap(err, "problem creating the splunk logger")
			}
			senders = append(senders, sender)
		}
	} else {
		grip.Notice("agent started via command - not configuring external logger")
	}

	if prefix == "" {
		// pass
	} else if prefix == evergreen.LocalLoggingOverride || prefix == "--" || prefix == evergreen.StandardOutputLoggingOverride {
		sender, err = send.NewNativeLogger("evergreen.agent", send.LevelInfo{Default: level.Info, Threshold: level.Debug})
		if err != nil {
			return nil, errors.Wrap(err, "problem creating a native console logger")
		}

		senders = append(senders, sender)
	} else {
		sender, err = send.NewFileLogger("evergreen.agent",
			fmt.Sprintf("%s-%d-%d.log", prefix, os.Getpid(), getInc()), send.LevelInfo{Default: level.Info, Threshold: level.Debug})
		if err != nil {
			return nil, errors.Wrap(err, "problem creating a file logger")
		}

		senders = append(senders, sender)
	}

	return send.NewConfiguredMultiSender(senders...), nil
}

func (a *Agent) SetDefaultLogger(sender send.Sender) {
	a.defaultLogger = sender
}

func (a *Agent) makeLoggerProducer(ctx context.Context, tc *taskContext, c *model.LoggerConfig, commandName string) (client.LoggerProducer, error) {
	config := a.prepLogger(tc, c, commandName)

	logger, err := a.comm.GetLoggerProducer(ctx, tc.task, &config)
	if err != nil {
		return nil, err
	}
	loggerData := a.comm.GetLoggerMetadata()
	tc.logs = &apimodels.TaskLogs{}
	for _, agent := range loggerData.Agent {
		tc.logs.AgentLogURLs = append(tc.logs.AgentLogURLs, apimodels.LogInfo{
			Command: commandName,
			URL:     fmt.Sprintf("%s/build/%s/test/%s", a.opts.LogkeeperURL, agent.Build, agent.Test),
		})
	}
	for _, system := range loggerData.System {
		tc.logs.SystemLogURLs = append(tc.logs.SystemLogURLs, apimodels.LogInfo{
			Command: commandName,
			URL:     fmt.Sprintf("%s/build/%s/test/%s", a.opts.LogkeeperURL, system.Build, system.Test),
		})
	}
	for _, task := range loggerData.Task {
		tc.logs.TaskLogURLs = append(tc.logs.TaskLogURLs, apimodels.LogInfo{
			Command: commandName,
			URL:     fmt.Sprintf("%s/build/%s/test/%s", a.opts.LogkeeperURL, task.Build, task.Test),
		})
	}
	return logger, nil
}

func (a *Agent) prepLogger(tc *taskContext, c *model.LoggerConfig, commandName string) client.LoggerConfig {
	logDir := filepath.Join(a.opts.WorkingDirectory, taskLogDirectory)
	grip.Error(errors.Wrap(os.MkdirAll(logDir, os.ModeDir|os.ModePerm), "error making log directory"))
	// if this is a command-specific logger, create a dir for the command's logs separate from the overall task
	if commandName != "" {
		logDir = filepath.Join(logDir, commandName)
		grip.Error(errors.Wrapf(os.MkdirAll(logDir, os.ModeDir|os.ModePerm), "error making log directory for command %s", commandName))
	}
	config := client.LoggerConfig{}

	var defaultLogger string
	if tc.taskConfig != nil && tc.taskConfig.ProjectRef != nil {
		defaultLogger = tc.taskConfig.ProjectRef.DefaultLogger
	}
	if !model.IsValidDefaultLogger(defaultLogger) {
		grip.Warningf("default logger '%s' is not valid, setting Evergreen logger as default", defaultLogger)
		defaultLogger = model.EvergreenLogSender
	}
	if len(c.Agent) == 0 {
		c.Agent = []model.LogOpts{{Type: defaultLogger}}
	}
	if len(c.System) == 0 {
		c.System = []model.LogOpts{{Type: defaultLogger}}
	}
	if len(c.Task) == 0 {
		c.Task = []model.LogOpts{{Type: defaultLogger}}
	}

	for _, agentConfig := range c.Agent {
		config.Agent = append(config.Agent, a.prepSingleLogger(tc, agentConfig, logDir, agentLogFileName))
	}
	for _, systemConfig := range c.System {
		config.System = append(config.System, a.prepSingleLogger(tc, systemConfig, logDir, systemLogFileName))
	}
	for _, taskConfig := range c.Task {
		config.Task = append(config.Task, a.prepSingleLogger(tc, taskConfig, logDir, taskLogFileName))
	}

	return config
}

func (a *Agent) prepSingleLogger(tc *taskContext, in model.LogOpts, logDir, fileName string) client.LogOpts {
	splunkServer, err := tc.expansions.ExpandString(in.SplunkServer)
	if err != nil {
		grip.Error(errors.Wrap(err, "error expanding splunk server"))
	}
	splunkToken, err := tc.expansions.ExpandString(in.SplunkToken)
	if err != nil {
		grip.Error(errors.Wrap(err, "error expanding splunk token"))
	}
	if in.LogDirectory != "" {
		grip.Error(errors.Wrap(os.MkdirAll(in.LogDirectory, os.ModeDir|os.ModePerm), "error making log directory"))
		logDir = in.LogDirectory
	}
	if tc.logDirectories == nil {
		tc.logDirectories = map[string]interface{}{}
	}
	tc.logDirectories[logDir] = nil
	return client.LogOpts{
		LogkeeperURL:      a.opts.LogkeeperURL,
		LogkeeperBuildNum: tc.taskModel.Execution,
		BuilderID:         tc.taskModel.Id,
		Sender:            in.Type,
		SplunkServerURL:   splunkServer,
		SplunkToken:       splunkToken,
		Filepath:          filepath.Join(logDir, fileName),
	}
}

func (a *Agent) uploadToS3(ctx context.Context, tc *taskContext) error {
	if a.opts.S3Opts.Name == "" {
		return nil
	}
	bucket, err := pail.NewS3Bucket(a.opts.S3Opts)
	if err != nil {
		return errors.Wrap(err, "error creating pail")
	}

	catcher := grip.NewBasicCatcher()
	for logDir := range tc.logDirectories {
		catcher.Add(a.uploadLogDir(ctx, tc, bucket, logDir, ""))
	}

	return catcher.Resolve()
}

func (a *Agent) uploadLogDir(ctx context.Context, tc *taskContext, bucket pail.Bucket, directoryName, commandName string) error {
	if tc.taskConfig == nil || tc.taskConfig.Task == nil {
		return nil
	}
	catcher := grip.NewBasicCatcher()
	if commandName != "" {
		directoryName = filepath.Join(directoryName, commandName)
	}
	dir, err := ioutil.ReadDir(directoryName)
	if err != nil {
		catcher.Add(errors.Wrap(err, "error reading log directory"))
		return catcher.Resolve()
	}
	for _, f := range dir {
		if f.IsDir() {
			catcher.Add(a.uploadLogDir(ctx, tc, bucket, directoryName, f.Name()))
		} else {
			catcher.Add(a.uploadSingleFile(ctx, tc, bucket, f.Name(), tc.taskConfig.Task.Id, tc.taskConfig.Task.Execution, commandName))
		}
	}

	return catcher.Resolve()
}

func (a *Agent) uploadSingleFile(ctx context.Context, tc *taskContext, bucket pail.Bucket, file string, taskID string, execution int, cmd string) error {
	localDir := filepath.Join(a.opts.WorkingDirectory, taskLogDirectory)
	remotePath := fmt.Sprintf("logs/%s/%s", taskID, strconv.Itoa(execution))
	if cmd != "" {
		localDir = filepath.Join(localDir, cmd)
		remotePath = fmt.Sprintf("%s/%s", remotePath, cmd)
	}
	localPath := filepath.Join(localDir, file)
	_, err := os.Stat(localPath)
	if os.IsNotExist(err) {
		return nil
	}
	err = bucket.Upload(ctx, fmt.Sprintf("%s/%s", remotePath, file), localPath)
	if err != nil {
		return errors.Wrapf(err, "error uploading %s to S3", localPath)
	}
	remoteURL := util.S3DefaultURL(a.opts.S3Opts.Name, strings.Join([]string{remotePath, file}, "/"))
	tc.logger.Execution().Infof("uploaded file %s from %s to %s", file, localPath, remoteURL)
	switch file {
	case agentLogFileName:
		tc.logs.AgentLogURLs = append(tc.logs.AgentLogURLs, apimodels.LogInfo{
			Command: cmd,
			URL:     remoteURL,
		})
	case systemLogFileName:
		tc.logs.SystemLogURLs = append(tc.logs.SystemLogURLs, apimodels.LogInfo{
			Command: cmd,
			URL:     remoteURL,
		})
	case taskLogFileName:
		tc.logs.TaskLogURLs = append(tc.logs.TaskLogURLs, apimodels.LogInfo{
			Command: cmd,
			URL:     remoteURL,
		})
	}
	return nil
}
