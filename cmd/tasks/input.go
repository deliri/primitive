package main

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/secretstore"
	"github.com/deliri/primitive/v2026/taskmanager"
)

const (
	defaultJobFileName   = "task_job.json"
	standardInputPath    = "-"
	schemaArgument       = "schema"
	commandArgumentCount = 1
)

type invocationMode uint8

const (
	invocationModeUnknown invocationMode = iota
	invocationModeExecute
	invocationModeSchema
	invocationModeLimit
)

type invocation struct {
	JobPath string
	Mode    invocationMode
}

type commandInputRequest struct {
	StandardInput    io.Reader
	WorkingDirectory core.AbsolutePath
	JobPath          string
}

func (r commandInputRequest) Validate() error {
	if r.JobPath == "" || r.StandardInput == nil {
		return commandError("command input request is incomplete", nil)
	}
	return r.WorkingDirectory.Validate()
}

func (i invocation) Validate() error {
	switch i.Mode {
	case invocationModeExecute:
		if i.JobPath == "" {
			return commandError("job path is empty", nil)
		}
		return nil
	case invocationModeSchema:
		if i.JobPath != "" {
			return commandError("schema invocation must not carry a job path", nil)
		}
		return nil
	case invocationModeUnknown, invocationModeLimit:
		return commandError(invocationModeOutsidePublishedDomainDetail, nil)
	default:
		return commandError(invocationModeOutsidePublishedDomainDetail, nil)
	}
}

func parseInvocation(values []string) (invocation, error) {
	arguments, err := process.ParseArguments(values)
	if err != nil {
		return invocation{}, commandError("command arguments are invalid", err)
	}
	if len(arguments) == 0 {
		return invocation{Mode: invocationModeExecute, JobPath: defaultJobFileName}, nil
	}
	if len(arguments) != commandArgumentCount {
		return invocation{}, commandError("usage: tasks [job.json|-] or tasks schema", nil)
	}
	value, err := arguments[0].Value()
	if err != nil {
		return invocation{}, commandError("command argument cannot be projected", err)
	}
	if value == schemaArgument {
		return invocation{Mode: invocationModeSchema}, nil
	}
	result := invocation{Mode: invocationModeExecute, JobPath: value}
	if err := result.Validate(); err != nil {
		return invocation{}, err
	}
	return result, nil
}

func loadInputs(
	ctx context.Context,
	request commandInputRequest,
) (configurationDocument, jobDocument, error) {
	if ctx == nil {
		return configurationDocument{}, jobDocument{}, commandError("command input context is nil", core.ErrNilContext)
	}
	if err := request.Validate(); err != nil {
		return configurationDocument{}, jobDocument{}, err
	}
	configurationPath, err := request.WorkingDirectory.Resolve(configurationFileName)
	if err != nil {
		return configurationDocument{}, jobDocument{}, commandError("configuration path is invalid", err)
	}
	configuration, err := readDocument[configurationDocument](ctx, configurationPath, configurationMaxBytes)
	if err != nil {
		return configurationDocument{}, jobDocument{}, commandError("task_config.json cannot be loaded", err)
	}
	job, err := loadJob(ctx, request)
	if err != nil {
		return configurationDocument{}, jobDocument{}, err
	}
	return configuration, job, nil
}

func loadJob(
	ctx context.Context,
	request commandInputRequest,
) (jobDocument, error) {
	if ctx == nil {
		return jobDocument{}, commandError("job input context is nil", core.ErrNilContext)
	}
	if err := request.Validate(); err != nil {
		return jobDocument{}, err
	}
	if request.JobPath == standardInputPath {
		limits, err := documentLimits(commandDocumentMaxBytes)
		if err != nil {
			return jobDocument{}, err
		}
		job, err := core.DecodeStrictJSON[jobDocument](request.StandardInput, limits)
		if err != nil {
			return jobDocument{}, commandError("stdin job document is invalid", err)
		}
		return job, nil
	}
	absolute, err := request.WorkingDirectory.ResolveText(request.JobPath)
	if err != nil {
		return jobDocument{}, commandError("job path is invalid", err)
	}
	job, err := readDocument[jobDocument](ctx, absolute, commandDocumentMaxBytes)
	if err != nil {
		return jobDocument{}, commandError("job document cannot be loaded", err)
	}
	return job, nil
}

func readDocument[Document core.Validatable](
	ctx context.Context,
	path core.AbsolutePath,
	maximum uint64,
) (Document, error) {
	var zero Document
	location, err := filestore.OpenParent(ctx, path)
	if err != nil {
		return zero, err
	}
	file, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: location})
	if err != nil {
		return zero, errors.Join(err, location.Root.Close())
	}
	limits, err := documentLimits(maximum)
	if err != nil {
		return zero, errors.Join(err, file.Close(), location.Root.Close())
	}
	document, decodeErr := core.DecodeStrictJSON[Document](file, limits)
	return document, errors.Join(decodeErr, file.Close(), location.Root.Close())
}

func documentLimits(maximum uint64) (core.StrictJSONLimits, error) {
	count, err := core.NewByteCount(maximum)
	if err != nil {
		return core.StrictJSONLimits{}, commandError("document byte limit is invalid", err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = count
	if err := limits.Validate(); err != nil {
		return core.StrictJSONLimits{}, commandError("document limits are invalid", err)
	}
	return limits, nil
}

func configuredClient(ctx context.Context, configuration configurationDocument) (taskmanager.Client, error) {
	if err := configuration.Validate(); err != nil {
		return taskmanager.Client{}, err
	}
	request, err := configuration.PasswordSecret.accessRequest()
	if err != nil {
		return taskmanager.Client{}, err
	}
	reader, err := secretstore.NewGoogleReader(ctx)
	if err != nil {
		return taskmanager.Client{}, commandError("Google Secret Manager client construction failed", err)
	}
	result, accessErr := reader.Access(ctx, request)
	closeErr := reader.Close()
	if err := errors.Join(accessErr, closeErr); err != nil {
		return taskmanager.Client{}, commandError("password secret access failed", err)
	}
	password, copyErr := result.Value.CopyBytes()
	destroyErr := result.Value.Destroy()
	if err := errors.Join(copyErr, destroyErr); err != nil {
		clear(password)
		return taskmanager.Client{}, commandError("password secret projection failed", err)
	}
	defer clear(password)
	httpClient, err := exchange.NewStandardClient()
	if err != nil {
		return taskmanager.Client{}, commandError("HTTP client construction failed", err)
	}
	return taskManagerClient(configuration, password, httpClient)
}

func taskManagerClient(
	configuration configurationDocument,
	password []byte,
	httpClient exchange.Client,
) (taskmanager.Client, error) {
	if err := errors.Join(configuration.Validate(), httpClient.Validate()); err != nil {
		return taskmanager.Client{}, commandError("task-manager client input is invalid", err)
	}
	header, err := exchange.NewBasicAuthorizationHeader(exchange.BasicAuthorizationRequest{
		Identity: configuration.Username,
		Secret:   password,
	})
	if err != nil {
		return taskmanager.Client{}, commandError("authorization header construction failed", err)
	}
	headers := exchange.Headers{Values: []exchange.Header{header}}
	client, err := taskmanager.NewClient(taskmanager.ClientConfiguration{
		HTTP: httpClient, Authority: configuration.Authority, Headers: headers,
	})
	if err != nil {
		return taskmanager.Client{}, commandError("task-manager client construction failed", err)
	}
	return client, nil
}

var (
	_ core.Validatable = invocation{}
	_ core.Validatable = commandInputRequest{}
)
