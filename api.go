package form3api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	DefaultRetryCount uint = 3
)

const (
	// Base URL of the Form3 API.
	BaseURL = "http://accountapi:8080"
)

type API interface {
	// Create a new bank account or register an existing bank account with Form3.
	// See https://www.api-docs.form3.tech/api/schemes/fps-direct/accounts/accounts/create-an-account
	Create(ctx context.Context, data AccountData) (AccountData, error)

	// Fetch a single Account resource using the accountID.
	Fetch(ctx context.Context, accountID string) (AccountData, error)

	// Delete an Account resource using the accountID and the current version number.
	Delete(ctx context.Context, accountID string, version int64) error
}

type api struct {
	client     *http.Client
	retryCount uint
}

func drainAndCloseHttpResponse(resp *http.Response) {
	// Needed for keepalive connection reusage.
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

func (a *api) httpDoRetry(req *http.Request, count uint) (*http.Response, error) {
	for i := uint(0); i < count; i++ {
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, err
		}

		switch resp.StatusCode {
		case 429, 500, 503, 504:
			drainAndCloseHttpResponse(resp)
		default:
			return resp, nil
		}

		if err := backOff(req.Context(), i); err != nil {
			return nil, err
		}
	}
	return nil, new(ErrTooManyRetries)
}

func parse400or409(resp *http.Response) error {
	var ret GenericError
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return err
	}
	if resp.StatusCode == 400 {
		return newErrBadRequest(ret)
	}
	return newErrConflict(ret)
}

func parse403(resp *http.Response) error {
	var ret ForbiddenError
	if err := json.NewDecoder(resp.Body).Decode(&ret); err != nil {
		return err
	}
	return newErrForbiden(ret)
}

func parseError(resp *http.Response) error {
	switch resp.StatusCode {
	case 400, 409:
		return parse400or409(resp)
	case 403:
		return parse403(resp)
	case 404:
		return new(ErrNotFound)
	default:
		return newErrHttp(resp.StatusCode)
	}
}

func (a *api) httpDo(ctx context.Context, method, url string, body any, res any) error {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(body); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, &b)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/vnd.api+json")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Content-Type", "application/vnd.api+json")
	// No need to set Content-Length, stdlib is aware that we passed bytes.Buffer.

	resp, err := a.httpDoRetry(req, a.retryCount)
	if err != nil {
		return err
	}
	defer drainAndCloseHttpResponse(resp)

	switch resp.StatusCode {
	case 200, 201, 204:
	default:
		return parseError(resp)
	}

	if res != nil && resp.StatusCode != 204 {
		if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
			return err
		}
	}
	return nil
}

func (a *api) Create(ctx context.Context, data AccountData) (AccountData, error) {
	var ret struct {
		Data AccountData
	}

	if err := a.httpDo(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/v1/organisation/accounts", BaseURL),
		&struct{ Data AccountData }{Data: data},
		&ret,
	); err != nil {
		return AccountData{}, err
	}

	return ret.Data, nil
}

func (a *api) Fetch(ctx context.Context, accountID string) (AccountData, error) {
	var ret struct {
		Data AccountData
	}

	if err := a.httpDo(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/v1/organisation/accounts/%s", BaseURL, accountID),
		nil,
		&ret,
	); err != nil {
		return AccountData{}, err
	}

	return ret.Data, nil
}

func (a *api) Delete(ctx context.Context, accountID string, version int64) error {
	return a.httpDo(
		ctx,
		http.MethodDelete,
		fmt.Sprintf(
			"%s/v1/organisation/accounts/%s?version=%d",
			BaseURL,
			accountID,
			version,
		),
		nil,
		nil,
	)
}

// WithHttpClient provides http.Client to be used by an API instance.
func WithHttpClient(client *http.Client) func(*api) {
	return func(a *api) {
		a.client = client
	}
}

// WithRetryCount overrides default retry count used by an API instance.
func WithRetryCount(n uint) func(*api) {
	return func(a *api) {
		a.retryCount = n
	}
}

// NewAPI creates an API object that uses http.DefaultClient and default
// retry count (when throttled).
func NewAPI(options ...func(*api)) API {
	ret := &api{
		client:     http.DefaultClient,
		retryCount: DefaultRetryCount,
	}
	for _, f := range options {
		f(ret)
	}
	return ret
}
