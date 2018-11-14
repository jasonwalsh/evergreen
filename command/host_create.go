package command

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/evergreen-ci/evergreen/apimodels"
	"github.com/evergreen-ci/evergreen/model"
	"github.com/evergreen-ci/evergreen/rest/client"
	"github.com/evergreen-ci/evergreen/util"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

type createHost struct {
	CreateHost *apimodels.CreateHost
	base
}

func createHostFactory() Command { return &createHost{} }

func (c *createHost) Name() string { return "host.create" }

func (c *createHost) ParseParams(params map[string]interface{}) error {
	c.CreateHost = &apimodels.CreateHost{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           c.CreateHost,
	})
	if err != nil {
		return errors.WithStack(err)
	}
	if err := decoder.Decode(params); err != nil {
		return errors.Wrapf(err, "error decoding %s params", c.Name())
	}
	return nil
}

func (c *createHost) expandAndValidate(conf *model.TaskConfig) error {
	var err error
	if err = util.ExpandValues(c.CreateHost, conf.Expansions); err != nil {
		return errors.Wrap(err, "error expanding params")
	}
	var numHosts int
	numHosts, err = c.CreateHost.NumHosts.Int()
	if err != nil {
		return errors.WithStack(err)
	}
	if numHosts != 0 {
		c.CreateHost.NumHostsInt = numHosts
	}
	if err = c.CreateHost.Validate(); err != nil {
		return errors.Wrap(err, "command is invalid")
	}
	return nil
}

func (c *createHost) Execute(ctx context.Context, comm client.Communicator,
	logger client.LoggerProducer, conf *model.TaskConfig) error {

	if err := c.expandAndValidate(conf); err != nil {
		return err
	}

	taskData := client.TaskData{
		ID:     conf.Task.Id,
		Secret: conf.Task.Secret,
	}

	if c.CreateHost.UserdataFile != "" {
		if err := c.populateUserdata(); err != nil {
			return err
		}
	}

	return comm.CreateHost(ctx, taskData, *c.CreateHost)
}

func (c *createHost) populateUserdata() error {
	file, err := os.Open(c.CreateHost.UserdataFile)
	if err != nil {
		return errors.Wrap(err, "error opening UserData file")
	}
	defer file.Close()
	fileData, err := ioutil.ReadAll(file)
	if err != nil {
		return errors.Wrap(err, "error reading UserData file")
	}
	c.CreateHost.UserdataCommand = string(fileData)

	return nil
}
