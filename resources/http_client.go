package resources

import (
	"bytes"
	"errors"
	"net/http"
	"time"
)

type HttpClient struct {
}

// GetStatusCode - sends GET request and returns status code
func (hc *HttpClient) GetStatusCode(url string) (int, error) {

	r, err := http.NewRequest("GET", url, bytes.NewReader([]byte("")))
	if err != nil {
		return 0, errors.New("Error creating request")
	}

	httpClient := &http.Client{Timeout: time.Second * 15}
	res, err := httpClient.Do(r)
	if err != nil {
		return 0, errors.New("Error making http request to GTG endpoint")
	}
	defer res.Body.Close()

	return res.StatusCode, nil
}
