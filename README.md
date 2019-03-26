customresource
----------------------------------------

`customresource` provides a library that simplifies writing AWS Custom Resources in Go.

```QuickStart
func main() {
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
``` 