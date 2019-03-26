// Copyright 2019 Matt Ho
// 
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// 
// http://www.apache.org/licenses/LICENSE-2.0
// 
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package customresource

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

const (
	RequestTypeCreate = "Create"
	RequestTypeUpdate = "Update"
	RequestTypeDelete = "Delete"
)

const (
	StatusSuccess = "SUCCESS"
	StatusFailed  = "FAILED"
)

// Request that arrives from AWS
type Request struct {
	RequestType           string
	ResponseURL           string
	StackId               string
	RequestId             string
	ResourceType          string
	LogicalResourceId     string
	PhysicalResourceId    string
	ResourceProperties    json.RawMessage `json:",omitempty"`
	OldResourceProperties json.RawMessage `json:",omitempty"`
}

// Response contains the successful response to our request
type Response struct {
	// Data to return as output
	Data map[string]interface{}
	// PhysicalResourceId that uniquely identifies the resource that was created
	PhysicalResourceId string
	// NoEcho prevents Data from being returned by !GetAtt
	NoEcho bool
}

// Func to encapsulate custom resource logic
type Func func(ctx context.Context, req *Request) (*Response, error)

// Handler provides a lambda wrapper to manage the lifecycle of a custom resource
type Handler struct {
	fn        Func
	output    io.Writer
	transport http.RoundTripper
}

type replyInput struct {
	Status             string
	Reason             string
	PhysicalResourceId string
	StackId            string
	RequestId          string
	LogicalResourceId  string
	Data               interface{}
}

func (h *Handler) reply(ctx context.Context, req *Request, input *replyInput) error {
	data, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("unable to marshal reply")
	}

	httpReq, err := http.NewRequest(http.MethodPut, req.ResponseURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq = httpReq.WithContext(ctx)

	httpResp, err := h.transport.RoundTrip(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	fmt.Fprintln(h.output, httpResp.Status)
	io.Copy(h.output, httpResp.Body)

	return nil
}

func (h *Handler) replySuccess(ctx context.Context, req *Request, resp *Response) error {
	fmt.Fprintf(h.output, "%v: %v succeeded. PhysicalResourceId=%v\n", req.LogicalResourceId, req.RequestType, resp.PhysicalResourceId)
	input := replyInput{
		Status:             StatusSuccess,
		PhysicalResourceId: resp.PhysicalResourceId,
		StackId:            req.StackId,
		RequestId:          req.RequestId,
		LogicalResourceId:  req.LogicalResourceId,
		Data:               resp.Data,
	}
	return h.reply(ctx, req, &input)
}

func (h *Handler) replyFailure(ctx context.Context, req *Request, reason string) error {
	fmt.Fprintf(h.output, "%v: %v failed - %v\n", req.LogicalResourceId, req.RequestType, reason)
	input := replyInput{
		Status: StatusFailed,
		Reason: reason,
	}
	return h.reply(ctx, req, &input)
}

func (h *Handler) safeInvoke(ctx context.Context, req *Request) (resp *Response, err error) {
	defer func() {
		if r := recover(); r != nil {
			if v, ok := r.(error); ok {
				err = v
				return
			}

			err = fmt.Errorf("recovered from %v", r)
		}
	}()

	return h.fn(ctx, req)
}

// Invoke implements lambda.Handler
func (h *Handler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	var req Request
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}

	resp, err := h.safeInvoke(ctx, &req)
	if err != nil {
		reason := err.Error()
		return nil, h.replyFailure(ctx, &req, reason)
	}

	return nil, h.replySuccess(ctx, &req, resp)
}

type options struct {
	output    io.Writer
	transport http.RoundTripper
}

// Option functional option for the Handler
type Option func(*options)

// WithOutput displays output to the specified Writer
func WithOutput(w io.Writer) Option {
	return func(o *options) {
		if w != nil {
			o.output = w
		}
	}
}

// WithTransport allows the transport to be customized
func WithTransport(transport http.RoundTripper) Option {
	return func(o *options) {
		if transport != nil {
			o.transport = transport
		}
	}
}

// New returns a new custom response handler
func New(fn Func, opts ...Option) *Handler {
	options := options{
		output:    ioutil.Discard,
		transport: http.DefaultTransport,
	}
	for _, opt := range opts {
		opt(&options)
	}

	return &Handler{
		fn:        fn,
		output:    options.output,
		transport: options.transport,
	}
}
