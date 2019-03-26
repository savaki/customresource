package customresource

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/aws/aws-lambda-go/lambda"
)

type transportFunc func(req *http.Request) (*http.Response, error)

func (fn transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func ExampleHandler() {
	fn := func(ctx context.Context, req *Request) (*Response, error) {
		switch req.RequestType {
		case RequestTypeCreate:
			// create the resource ...
		case RequestTypeUpdate:
			// update the resource ...
		case RequestTypeDelete:
			// delete the resource ...
		}

		return &Response{
			PhysicalResourceId: "blah",
		}, nil
	}

	handler := New(fn)
	lambda.StartHandler(handler)
}

func TestHandler_Invoke(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		var (
			ctx  = context.Background()
			resp = Response{
				PhysicalResourceId: "blah",
			}
			reply []byte
			rt    = func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusOK)
				reply, _ = ioutil.ReadAll(req.Body)
				return w.Result(), nil
			}
			req = Request{
				LogicalResourceId: "Resource",
				RequestType:       RequestTypeCreate,
				ResponseURL:       "http://localhost",
			}
			got Request
			fn  = func(ctx context.Context, req *Request) (*Response, error) {
				got = *req
				return &resp, nil
			}
		)

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}

		handler := New(fn, WithTransport(transportFunc(rt)), WithOutput(os.Stdout))
		v, err := handler.Invoke(ctx, data)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		if v != nil {
			t.Fatalf("got %v; want nil", v)
		}
		if want := req; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v; want %v", got, want)
		}

		var input replyInput
		if err := json.Unmarshal(reply, &input); err != nil {
			t.Fatalf("got %v; want nil", v)
		}
		if got, want := input.Status, StatusSuccess; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}
	})

	t.Run("err", func(t *testing.T) {
		var (
			ctx    = context.Background()
			reason = "boom"
			reply  []byte
			rt     = func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusOK)
				reply, _ = ioutil.ReadAll(req.Body)
				return w.Result(), nil
			}
			req = Request{
				RequestType: RequestTypeCreate,
				ResponseURL: "http://localhost",
			}
			fn = func(ctx context.Context, req *Request) (*Response, error) {
				return nil, fmt.Errorf(reason)
			}
		)

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}

		handler := New(fn, WithTransport(transportFunc(rt)))
		v, err := handler.Invoke(ctx, data)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		if v != nil {
			t.Fatalf("got %v; want nil", v)
		}

		var input replyInput
		if err := json.Unmarshal(reply, &input); err != nil {
			t.Fatalf("got %v; want nil", v)
		}
		if got, want := input.Status, StatusFailed; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}
		if got, want := input.Reason, reason; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}
	})

	t.Run("panic", func(t *testing.T) {
		var (
			ctx   = context.Background()
			reply []byte
			rt    = func(req *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				w.WriteHeader(http.StatusOK)
				reply, _ = ioutil.ReadAll(req.Body)
				return w.Result(), nil
			}
			req = Request{
				RequestType: RequestTypeCreate,
				ResponseURL: "http://localhost",
			}
			fn = func(ctx context.Context, req *Request) (*Response, error) {
				var m map[string]string
				m["hello"] = "world"
				return &Response{}, nil
			}
		)

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}

		handler := New(fn, WithTransport(transportFunc(rt)))
		v, err := handler.Invoke(ctx, data)
		if err != nil {
			t.Fatalf("got %v; want nil", err)
		}
		if v != nil {
			t.Fatalf("got %v; want nil", v)
		}

		var input replyInput
		if err := json.Unmarshal(reply, &input); err != nil {
			t.Fatalf("got %v; want nil", v)
		}
		if got, want := input.Status, StatusFailed; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}
		if got, want := input.Reason, "assignment to entry in nil map"; got != want {
			t.Fatalf("got %v; want %v", got, want)
		}
	})
}
