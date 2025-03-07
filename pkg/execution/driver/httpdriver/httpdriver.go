package httpdriver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/inngest/inngest-cli/inngest"
	"github.com/inngest/inngest-cli/pkg/execution/driver"
	"github.com/inngest/inngest-cli/pkg/execution/state"
)

var (
	DefaultExecutor = executor{
		client: &http.Client{
			Timeout: 15 * time.Minute,
		},
	}
)

func Execute(ctx context.Context, state state.State, action inngest.ActionVersion, step inngest.Step) (*driver.Response, error) {
	return DefaultExecutor.Execute(ctx, state, action, step)
}

type executor struct {
	client *http.Client
}

func (e executor) Execute(ctx context.Context, state state.State, action inngest.ActionVersion, step inngest.Step) (*driver.Response, error) {
	rt, ok := action.Runtime.Runtime.(inngest.RuntimeHTTP)
	if !ok {
		return nil, fmt.Errorf("Unable to use HTTP executor for non-HTTP runtime")
	}

	input, err := driver.MarshalV1(state)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, rt.URL, bytes.NewBuffer(input))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}

	byt, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, err
	}

	var body interface{}
	body = string(byt)

	// Is the response valid JSON?  If so, ensure that we don't re-marshal the
	// JSON string.
	respjson := map[string]interface{}{}
	if err := json.Unmarshal(byt, &respjson); err == nil {
		body = respjson
	}

	// Add an error to driver.Response if the status code isn't 2XX.
	err = nil
	if resp.StatusCode <= 200 && resp.StatusCode >= 299 {
		err = fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}

	return &driver.Response{
		Output: map[string]interface{}{
			"status": resp.StatusCode,
			"body":   body,
		},
		Err:           err,
		ActionVersion: action.Version,
	}, nil
}
