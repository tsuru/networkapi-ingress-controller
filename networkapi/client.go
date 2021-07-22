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
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	logger := log.FromContext(ctx).V(2).WithValues("method", method)
	var body io.Reader
	if bodyData != nil {
		body = bytes.NewReader(bodyData)
		logger = logger.WithValues("body", string(bodyData))
	}

	fullURL := fmt.Sprintf("%s/%s", strings.TrimSuffix(c.baseURL, "/"), strings.TrimPrefix(u, "/"))
	if qs != nil {
		fullURL += "?" + qs.Encode()
	}

	logger = logger.WithValues("url", fullURL)
	logger.Info("NetworkAPI request")
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
		return nil, errors.Wrapf(err, "unable to request %s %s with body %s", method, fullURL, string(bodyData))
	}
	defer resp.Body.Close()

	rspData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to read response %d for %s %s with body %s", resp.StatusCode, method, fullURL, string(bodyData))
	}

	logger = logger.WithValues("status", resp.StatusCode, "response_body", string(rspData))
	logger.Info("NetworkAPI response")

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, errors.Errorf("invalid response %d for %s %s with body %s: %s", resp.StatusCode, method, fullURL, string(bodyData), string(rspData))
	}

	return rspData, nil
}

func (n *baseClient) doPost(ctx context.Context, url, reqName string, obj interface{}) (int, error) {
	body, err := marshalField(reqName, []interface{}{obj})
	if err != nil {
		return 0, err
	}
	data, err := n.doRequest(ctx, http.MethodPost, url, nil, body)
	if err != nil {
		return 0, err
	}
	ids, err := unmarshalIDs(data)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, errors.Errorf("no %s created: %s", reqName, string(data))
	}
	if len(ids) > 1 {
		return 0, errors.Errorf("multiple %s created: %s", reqName, string(data))
	}
	return ids[0], nil
}
