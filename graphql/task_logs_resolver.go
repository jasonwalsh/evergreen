package graphql

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.31

import (
	"context"
	"fmt"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/apimodels"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/model/event"
	"github.com/evergreen-ci/evergreen/model/task"
	restModel "github.com/evergreen-ci/evergreen/rest/model"
	"github.com/evergreen-ci/utility"
)

// AgentLogs is the resolver for the agentLogs field.
func (r *taskLogsResolver) AgentLogs(ctx context.Context, obj *TaskLogs) ([]*apimodels.LogMessage, error) {
	const logMessageCount = 100
	task, taskErr := task.FindOneIdAndExecution(obj.TaskID, obj.Execution)
	if taskErr != nil {
		return nil, InternalServerError.Send(ctx, fmt.Sprintf("Finding task %s: %s", obj.TaskID, taskErr.Error()))
	}
	if evergreen.IsUnstartedTaskStatus(task.Status) {
		return []*apimodels.LogMessage{}, nil
	}
	var agentLogs []apimodels.LogMessage
	// get logs from cedar
	if obj.DefaultLogger == model.BuildloggerLogSender {
		opts := apimodels.GetBuildloggerLogsOptions{
			BaseURL:       evergreen.GetEnvironment().Settings().Cedar.BaseURL,
			TaskID:        obj.TaskID,
			Execution:     utility.ToIntPtr(obj.Execution),
			PrintPriority: true,
			Tail:          logMessageCount,
			LogType:       apimodels.AgentLogPrefix,
		}
		// agent logs
		agentLogReader, err := apimodels.GetBuildloggerLogs(ctx, opts)
		if err != nil {
			return nil, InternalServerError.Send(ctx, err.Error())
		}
		agentLogs = apimodels.ReadBuildloggerToSlice(ctx, obj.TaskID, agentLogReader)
	} else {
		var err error
		// agent logs
		agentLogs, err = model.FindMostRecentLogMessages(obj.TaskID, obj.Execution, logMessageCount, []string{},
			[]string{apimodels.AgentLogPrefix})
		if err != nil {
			return nil, InternalServerError.Send(ctx, fmt.Sprintf("Finding agent logs for task %s: %s", obj.TaskID, err.Error()))
		}
	}

	agentLogPointers := []*apimodels.LogMessage{}

	for i := range agentLogs {
		agentLogPointers = append(agentLogPointers, &agentLogs[i])
	}
	return agentLogPointers, nil
}

// AllLogs is the resolver for the allLogs field.
func (r *taskLogsResolver) AllLogs(ctx context.Context, obj *TaskLogs) ([]*apimodels.LogMessage, error) {
	const logMessageCount = 100
	task, taskErr := task.FindOneIdAndExecutionWithDisplayStatus(obj.TaskID, &obj.Execution)
	if taskErr != nil {
		return nil, InternalServerError.Send(ctx, fmt.Sprintf("Finding task %s: %s", obj.TaskID, taskErr.Error()))
	}
	if evergreen.IsUnstartedTaskStatus(task.Status) {
		return []*apimodels.LogMessage{}, nil
	}
	var allLogs []apimodels.LogMessage

	// get logs from cedar
	if obj.DefaultLogger == model.BuildloggerLogSender {

		opts := apimodels.GetBuildloggerLogsOptions{
			BaseURL:       evergreen.GetEnvironment().Settings().Cedar.BaseURL,
			TaskID:        obj.TaskID,
			Execution:     utility.ToIntPtr(obj.Execution),
			PrintPriority: true,
			Tail:          logMessageCount,
			LogType:       apimodels.AllTaskLevelLogs,
		}

		// all logs
		allLogReader, err := apimodels.GetBuildloggerLogs(ctx, opts)
		if err != nil {
			return nil, InternalServerError.Send(ctx, err.Error())
		}

		allLogs = apimodels.ReadBuildloggerToSlice(ctx, obj.TaskID, allLogReader)

	} else {
		var err error
		// all logs
		allLogs, err = model.FindMostRecentLogMessages(obj.TaskID, obj.Execution, logMessageCount, []string{}, []string{})
		if err != nil {
			return nil, InternalServerError.Send(ctx, fmt.Sprintf("Finding all logs for task %s: %s", obj.TaskID, err.Error()))
		}
	}

	allLogPointers := []*apimodels.LogMessage{}
	for i := range allLogs {
		allLogPointers = append(allLogPointers, &allLogs[i])
	}
	return allLogPointers, nil
}

// EventLogs is the resolver for the eventLogs field.
func (r *taskLogsResolver) EventLogs(ctx context.Context, obj *TaskLogs) ([]*restModel.TaskAPIEventLogEntry, error) {
	const logMessageCount = 100
	// loggedEvents is ordered ts descending
	loggedEvents, err := event.Find(event.MostRecentTaskEvents(obj.TaskID, logMessageCount))
	if err != nil {
		return nil, InternalServerError.Send(ctx, fmt.Sprintf("Unable to find EventLogs for task %s: %s", obj.TaskID, err.Error()))
	}

	// reverse order so it is ascending
	for i := len(loggedEvents)/2 - 1; i >= 0; i-- {
		opp := len(loggedEvents) - 1 - i
		loggedEvents[i], loggedEvents[opp] = loggedEvents[opp], loggedEvents[i]
	}

	// populate eventlogs pointer arrays
	apiEventLogPointers := []*restModel.TaskAPIEventLogEntry{}
	for _, e := range loggedEvents {
		apiEventLog := restModel.TaskAPIEventLogEntry{}
		err = apiEventLog.BuildFromService(e)
		if err != nil {
			return nil, InternalServerError.Send(ctx, fmt.Sprintf("Unable to build APIEventLogEntry from EventLog: %s", err.Error()))
		}
		apiEventLogPointers = append(apiEventLogPointers, &apiEventLog)
	}
	return apiEventLogPointers, nil
}

// SystemLogs is the resolver for the systemLogs field.
func (r *taskLogsResolver) SystemLogs(ctx context.Context, obj *TaskLogs) ([]*apimodels.LogMessage, error) {
	const logMessageCount = 100

	var systemLogs []apimodels.LogMessage
	task, taskErr := task.FindOneIdAndExecutionWithDisplayStatus(obj.TaskID, &obj.Execution)
	if taskErr != nil {
		return nil, InternalServerError.Send(ctx, fmt.Sprintf("Finding task %s: %s", obj.TaskID, taskErr.Error()))
	}
	if evergreen.IsUnstartedTaskStatus(task.Status) {
		return []*apimodels.LogMessage{}, nil
	}
	// get logs from cedar
	if obj.DefaultLogger == model.BuildloggerLogSender {
		opts := apimodels.GetBuildloggerLogsOptions{
			BaseURL:       evergreen.GetEnvironment().Settings().Cedar.BaseURL,
			TaskID:        obj.TaskID,
			Execution:     utility.ToIntPtr(obj.Execution),
			PrintPriority: true,
			Tail:          logMessageCount,
			LogType:       apimodels.TaskLogPrefix,
		}

		// system logs
		opts.LogType = apimodels.SystemLogPrefix
		systemLogReader, err := apimodels.GetBuildloggerLogs(ctx, opts)
		if err != nil {
			return nil, InternalServerError.Send(ctx, err.Error())
		}
		systemLogs = apimodels.ReadBuildloggerToSlice(ctx, obj.TaskID, systemLogReader)
	} else {
		var err error

		systemLogs, err = model.FindMostRecentLogMessages(obj.TaskID, obj.Execution, logMessageCount, []string{},
			[]string{apimodels.SystemLogPrefix})
		if err != nil {
			return nil, InternalServerError.Send(ctx, fmt.Sprintf("Finding system logs for task %s: %s", obj.TaskID, err.Error()))
		}
	}
	systemLogPointers := []*apimodels.LogMessage{}
	for i := range systemLogs {
		systemLogPointers = append(systemLogPointers, &systemLogs[i])
	}

	return systemLogPointers, nil
}

// TaskLogs is the resolver for the taskLogs field.
func (r *taskLogsResolver) TaskLogs(ctx context.Context, obj *TaskLogs) ([]*apimodels.LogMessage, error) {
	const logMessageCount = 100
	task, taskErr := task.FindOneIdAndExecutionWithDisplayStatus(obj.TaskID, &obj.Execution)
	if taskErr != nil {
		return nil, InternalServerError.Send(ctx, fmt.Sprintf("Finding task %s: %s", obj.TaskID, taskErr.Error()))
	}
	if evergreen.IsUnstartedTaskStatus(task.Status) {
		return []*apimodels.LogMessage{}, nil
	}

	var taskLogs []apimodels.LogMessage
	// get logs from cedar
	if obj.DefaultLogger == model.BuildloggerLogSender {

		opts := apimodels.GetBuildloggerLogsOptions{
			BaseURL:       evergreen.GetEnvironment().Settings().Cedar.BaseURL,
			TaskID:        obj.TaskID,
			Execution:     utility.ToIntPtr(obj.Execution),
			PrintPriority: true,
			Tail:          logMessageCount,
			LogType:       apimodels.TaskLogPrefix,
		}
		// task logs
		taskLogReader, err := apimodels.GetBuildloggerLogs(ctx, opts)

		if err != nil {
			return nil, InternalServerError.Send(ctx, fmt.Sprintf("Encountered an error while fetching build logger logs: %s", err.Error()))
		}

		taskLogs = apimodels.ReadBuildloggerToSlice(ctx, obj.TaskID, taskLogReader)

	} else {
		var err error

		// task logs
		taskLogs, err = model.FindMostRecentLogMessages(obj.TaskID, obj.Execution, logMessageCount, []string{},
			[]string{apimodels.TaskLogPrefix})
		if err != nil {
			return nil, InternalServerError.Send(ctx, fmt.Sprintf("Finding task logs for task %s: %s", obj.TaskID, err.Error()))
		}
	}

	taskLogPointers := []*apimodels.LogMessage{}
	for i := range taskLogs {
		taskLogPointers = append(taskLogPointers, &taskLogs[i])
	}

	return taskLogPointers, nil
}

// TaskLogs returns TaskLogsResolver implementation.
func (r *Resolver) TaskLogs() TaskLogsResolver { return &taskLogsResolver{r} }

type taskLogsResolver struct{ *Resolver }
