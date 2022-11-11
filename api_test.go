package form3api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"
)

type testRoundTripper struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (r *testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if r.roundTrip != nil {
		return r.roundTrip(req)
	}
	return nil, errors.New("nil roundtripper")
}

type bufferCloseWrapper struct {
	buf    *bytes.Buffer
	closed bool
}

func (rc *bufferCloseWrapper) Read(p []byte) (int, error) {
	if rc.buf == nil {
		return 0, io.EOF
	}
	return rc.buf.Read(p)
}

func (rc *bufferCloseWrapper) Close() error {
	rc.closed = true
	return nil
}

func (rc *bufferCloseWrapper) isDrainedAndClosed() bool {
	return rc.buf.Len() == 0 && rc.closed
}

func newBufferCloseWrapper(buf *bytes.Buffer) *bufferCloseWrapper {
	return &bufferCloseWrapper{
		buf: buf,
	}
}

func newClientReturningStatusCodeAndBuffer(statusCode int, rc io.ReadCloser) *http.Client {
	return &http.Client{
		Transport: &testRoundTripper{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					Status: fmt.Sprintf(
						"%d %s",
						statusCode,
						http.StatusText(statusCode),
					),
					StatusCode:    statusCode,
					Proto:         "HTTP/1.1",
					ProtoMajor:    1,
					ProtoMinor:    1,
					Body:          rc,
					ContentLength: -1,
					Close:         true,
					Request:       req,
				}, nil
			},
		},
	}
}

func newClientReturningStatusCode(statusCode int) *http.Client {
	return newClientReturningStatusCodeAndBuffer(statusCode, nil)
}

func newContextWithImmediateTimer() context.Context {
	return withNewTimer(context.Background(), func(d time.Duration) timer {
		return new(immediateTimer)
	})
}

func TestApiCreateFetchDeleteRetry(t *testing.T) {
	api := NewAPI(
		WithHttpClient(newClientReturningStatusCode(429)),
		WithRetryCount(100),
	)

	_, err := api.Create(newContextWithImmediateTimer(), AccountData{})
	if !errors.Is(err, new(ErrTooManyRetries)) {
		t.Error("Create error type not expected:", reflect.TypeOf(err).String())
	}

	_, err = api.Fetch(newContextWithImmediateTimer(), "foo")
	if !errors.Is(err, new(ErrTooManyRetries)) {
		t.Error("Fetch error type not expected:", reflect.TypeOf(err).String())
	}

	err = api.Delete(newContextWithImmediateTimer(), "bar", 123)
	if !errors.Is(err, new(ErrTooManyRetries)) {
		t.Error("Delete error type not expected:", reflect.TypeOf(err).String())
	}
}

func TestApiCreateBadRequestError(t *testing.T) {
	const message = `{
		"error_message": "Message parsing failed: Unexpected character",
		"error_code": "d0a17902-63ed-4cb6-a8e8-fac5ca31b0b7"
	}`

	buf := newBufferCloseWrapper(bytes.NewBufferString(message))

	api := NewAPI(
		WithHttpClient(
			newClientReturningStatusCodeAndBuffer(
				400,
				buf,
			),
		),
	)

	_, err := api.Create(newContextWithImmediateTimer(), AccountData{})
	if !errors.Is(err, new(ErrBadRequest)) {
		t.Error("error type not expected:", reflect.TypeOf(err).String())
	}

	if !buf.isDrainedAndClosed() {
		t.Error("buffer was not drained and closed")
	}
}

func TestApiCreateConflictError(t *testing.T) {
	const message = `{
		"error_message": "Duplicate id f72c5098-bf0f-4526-a215-54e5c1e2e687",
		"error_code": "4bc0fa5d-231e-43f3-af79-8fc371d95a31"
	}`

	buf := newBufferCloseWrapper(bytes.NewBufferString(message))

	api := NewAPI(
		WithHttpClient(
			newClientReturningStatusCodeAndBuffer(
				409,
				buf,
			),
		),
	)

	_, err := api.Create(newContextWithImmediateTimer(), AccountData{})
	if !errors.Is(err, new(ErrConflict)) {
		t.Error("error type not expected:", reflect.TypeOf(err).String())
	}

	if !buf.isDrainedAndClosed() {
		t.Error("buffer was not drained and closed")
	}
}

func TestApiFetchNotFound(t *testing.T) {
	const message = `<html>
	<head>
	  <title>404 Not Found</title>
	</head>
	<body bgcolor="white">
	  <center>
		<h1>404 Not Found</h1>
	  </center>
	  <hr>
	  <center>openresty</center>
	</body>
	</html>`

	buf := newBufferCloseWrapper(bytes.NewBufferString(message))

	api := NewAPI(
		WithHttpClient(
			newClientReturningStatusCodeAndBuffer(
				404,
				buf,
			),
		),
	)

	_, err := api.Fetch(newContextWithImmediateTimer(), "baz")
	if !errors.Is(err, new(ErrNotFound)) {
		t.Error("error type not expected:", reflect.TypeOf(err).String())
	}

	if !buf.isDrainedAndClosed() {
		t.Error("buffer was not drained and closed")
	}
}

func TestApiDeleteForbiddenError(t *testing.T) {
	const message = `{
		"error": "invalid_grant",
		"error_description": "Wrong email or password."
	}`

	buf := newBufferCloseWrapper(bytes.NewBufferString(message))

	api := NewAPI(
		WithHttpClient(
			newClientReturningStatusCodeAndBuffer(
				403,
				buf,
			),
		),
	)

	err := api.Delete(newContextWithImmediateTimer(), "quux", 321)
	if !errors.Is(err, new(ErrForbidden)) {
		t.Error("error type not expected:", reflect.TypeOf(err).String())
	}

	if !buf.isDrainedAndClosed() {
		t.Error("buffer was not drained and closed")
	}
}

func TestApiCreateSuccessfulRoundTrip(t *testing.T) {
	const message = `{
		"data": {
		  "id": "0d209d7f-d07a-4542-947f-5885fddddae2",
		  "organisation_id": "ba61483c-d5c5-4f50-ae81-6b8c039bea43",
		  "type": "accounts",
		  "attributes": {
			"bank_id": "400300",
			"bank_id_code": "GBDSC",
			"bic": "NWBKGB22",
			"country": "GB",
			"base_currency": "GBP",
			"iban": "GB11NWBK40030041426819",
			"account_number": "41426819",
			"customer_id": "123"
		  }
		}
	}`

	buf := newBufferCloseWrapper(bytes.NewBufferString(message))

	api := NewAPI(
		WithHttpClient(
			newClientReturningStatusCodeAndBuffer(
				200,
				buf,
			),
		),
	)

	data, err := api.Create(context.Background(), AccountData{})
	if err != nil {
		t.Error("no error expected, got:", err)
	}

	if !buf.isDrainedAndClosed() {
		t.Error("buffer was not drained and closed")
	}

	if data.ID != "0d209d7f-d07a-4542-947f-5885fddddae2" {
		t.Error("unexpected id:", data.ID)
	}
	if data.OrganisationID != "ba61483c-d5c5-4f50-ae81-6b8c039bea43" {
		t.Error("unexpected organisation id:", data.OrganisationID)
	}
	if data.Type != "accounts" {
		t.Error("unexpected type:", data.Type)
	}
	if data.Attributes.BankID != "400300" {
		t.Error("unexpected bank id:", data.Attributes.BankID)
	}
	if data.Attributes.BankIDCode != "GBDSC" {
		t.Error("unexpected bank id code:", data.Attributes.BankIDCode)
	}
	if data.Attributes.Bic != "NWBKGB22" {
		t.Error("unexpected bic:", data.Attributes.Bic)
	}
	if *data.Attributes.Country != "GB" {
		t.Error("unexpected country:", *data.Attributes.Country)
	}
	if data.Attributes.BaseCurrency != "GBP" {
		t.Error("unexpected base currency:", data.Attributes.BaseCurrency)
	}
	if data.Attributes.Iban != "GB11NWBK40030041426819" {
		t.Error("unexpected iban:", data.Attributes.Iban)
	}
	if data.Attributes.AccountNumber != "41426819" {
		t.Error("unexpected account number:", data.Attributes.AccountNumber)
	}
}

func TestApiCreateInvalidJsonResponse(t *testing.T) {
	const message = `{
		"data": {
		  "id": "0d209d7f-d07a-4542-947f-5885fddddae2",
		  "organisa
	}`

	buf := newBufferCloseWrapper(bytes.NewBufferString(message))

	api := NewAPI(
		WithHttpClient(
			newClientReturningStatusCodeAndBuffer(
				200,
				buf,
			),
		),
	)

	_, err := api.Create(context.Background(), AccountData{})
	if err == nil {
		t.Error("expected an error")
	}

	if !buf.isDrainedAndClosed() {
		t.Error("buffer was not drained and closed")
	}
}
