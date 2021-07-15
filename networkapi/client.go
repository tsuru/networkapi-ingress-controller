package networkapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

func getClient() *http.Client {
	return http.DefaultClient
}

type baseClient struct {
	baseURL  string
	username string
	password string
}

func (c *baseClient) doRequest(ctx context.Context, method string, u string, qs url.Values, bodyData []byte) ([]byte, error) {
	var body io.Reader
	if bodyData != nil {
		body = bytes.NewReader(bodyData)
	}

	fullURL := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.baseURL, "/"), strings.TrimPrefix(u, "/"))
	if qs != nil {
		fullURL += "?" + qs.Encode()
	}

	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if c.username != "" && c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := getClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rspData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, errors.Errorf("invalid response %d: %s", resp.StatusCode, string(rspData))
	}

	return rspData, nil

}
