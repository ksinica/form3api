//go:build integration
// +build integration

package form3api_test

import (
	"context"
	"errors"
	"net"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/ksinica/form3api"
)

func waitForURL(remoteUrl string, timeout time.Duration) bool {
	u, err := url.Parse(remoteUrl)
	if err != nil {
		panic(err)
	}

	ctx, cf := context.WithTimeout(context.Background(), timeout)
	defer cf()

	var d net.Dialer
	for {
		conn, err := d.DialContext(ctx, "tcp", u.Host)
		if err == nil {
			conn.Close()
			return true
		}

		if errors.Is(err, context.DeadlineExceeded) {
			return false
		}

		// We could put some back off here, because of unnecessry spinning...
	}
}

type accountAPITestState struct {
	data form3api.AccountData
}

func AccountAPICreateIntegrationTest(state *accountAPITestState) func(*testing.T) {
	return func(t *testing.T) {
		const (
			timeout = 15 * time.Second
		)

		api := form3api.NewAPI()

		ctx, cf := context.WithTimeout(context.Background(), timeout)
		data, err := api.Create(ctx, form3api.AccountData{
			ID:             uuid.Must(uuid.NewV4()).String(),
			OrganisationID: uuid.Must(uuid.NewV4()).String(),
			Type:           "accounts",
			Attributes: &form3api.AccountAttributes{
				Country:       form3api.String("GB"),
				BankID:        "400300",
				Bic:           "NWBKGB22",
				AccountNumber: "41426819",
				Iban:          "GB11NWBK40030041426819",
				Name:          []string{"John Doe"},
			},
		})
		cf()
		if err != nil {
			t.Error("expected no error, got:", err)
		}

		state.data = data
	}
}

func AccountAPIFetchIntgerationTest(state *accountAPITestState) func(*testing.T) {
	return func(t *testing.T) {
		const (
			timeout = 15 * time.Second
		)

		api := form3api.NewAPI()

		ctx, cf := context.WithTimeout(context.Background(), timeout)
		data, err := api.Fetch(ctx, state.data.ID)
		cf()
		if err != nil {
			t.Error("expected no error, got:", err)
		}

		if !reflect.DeepEqual(data, state.data) {
			t.Errorf("invalid fetch data, expected %v, got %v", state.data, data)
		}
	}
}

func AccountAPIDeleteIntegrationTest(state *accountAPITestState) func(*testing.T) {
	return func(t *testing.T) {
		const (
			timeout = 15 * time.Second
		)

		api := form3api.NewAPI()

		var version int64
		if state.data.Version != nil {
			version = *state.data.Version
		}

		ctx, cf := context.WithTimeout(context.Background(), timeout)
		err := api.Delete(ctx, state.data.ID, version)
		cf()
		if err != nil {
			t.Error("expected no error, got:", err)
		}
	}
}

func TestAccountAPIIntegration(t *testing.T) {
	if !waitForURL(form3api.BaseURL, 15*time.Second) {
		t.Error("could not connect to the service")
		t.FailNow()
	}

	var state accountAPITestState

	if t.Run("Create", AccountAPICreateIntegrationTest(&state)) {
		t.Run("Fetch", AccountAPIFetchIntgerationTest(&state))
		t.Run("Delete", AccountAPIDeleteIntegrationTest(&state))
	}
}
