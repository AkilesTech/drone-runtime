package engine

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"time"
)

const metadataURL = "http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/token"

// For these urls, the parts of the host name can be glob, for example '*.gcr.io" will match
// "foo.gcr.io" and "bar.gcr.io".
var containerRegistryUrls = []string{"container.cloud.google.com", "gcr.io", "*.gcr.io"}

var metadataHeader = &http.Header{
	"Metadata-Flavor": []string{"Google"},
}

// tokenBlob is used to decode the JSON blob containing an access token
// that is returned by GCE metadata.
type tokenBlob struct {
	AccessToken string `json:"access_token"`
}

var httpClient *http.Client

func init() {
	metadataHTTPClientTimeout := time.Second * 10
	httpClient = &http.Client{
		Transport: &http.Transport{},
		Timeout:   metadataHTTPClientTimeout,
	}
}

// HttpError wraps a non-StatusOK error code as an error.
type HttpError struct {
	StatusCode int
	Url        string
}

// Error implements error
func (he *HttpError) Error() string {
	return fmt.Sprintf("http status code: %d while fetching url %s",
		he.StatusCode, he.Url)
}

func readUrl(url string, header *http.Header) (body []byte, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if header != nil {
		req.Header = *header
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("body of failing http response: %v", resp.Body)
		return nil, &HttpError{
			StatusCode: resp.StatusCode,
			Url:        url,
		}
	}

	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return contents, nil
}

func getCredentialsForGCR() (*DockerAuth, error) {
	tokenJsonBlob, err := readUrl(metadataURL, metadataHeader)
	if err != nil {
		return nil, fmt.Errorf("while querying metadata endpoint: %v", err)
	}

	var parsedBlob tokenBlob
	if err := json.Unmarshal([]byte(tokenJsonBlob), &parsedBlob); err != nil {
		return nil, fmt.Errorf("while parsing json blob %s: %v", tokenJsonBlob, err)
	}

	return &DockerAuth{
		Username: "oauth2accesstoken",
		Password: parsedBlob.AccessToken,
	}, nil
}

func isGCRHost(host string) bool {
	for _, r := range containerRegistryUrls {
		if match, err := filepath.Match(r, host); err == nil && match {
			return true
		}
	}
	return false
}
