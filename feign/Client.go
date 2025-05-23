package feign

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-resty/resty/v2"
	"io"
	"net/http"
)

type Client struct {
	*resty.Client
	Config  *Config
	baseURL string
	headers map[string]string
}

func NewClient(configs ...*Config) *Client {
	cfg := GetConfig(configs...)
	return &Client{
		baseURL: cfg.Url,
		headers: cfg.Headers,
		Config:  cfg,
		Client: resty.New().
			SetTimeout(cfg.Timeout).
			SetRetryCount(cfg.RetryCount).
			SetRetryWaitTime(cfg.RetryWait).
			SetDebug(cfg.Debug),
	}
}

// Get sends a GET request and unmarshals the response.
//
// Example:
//
//	var result YourStruct
//	err := cli.Get(ctx, "/users/1", &result)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	resp, err := c.R().
		SetContext(ctx).
		Get(path)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return errors.New("request failed with status: " + resp.Status())
	}

	return json.Unmarshal(resp.Body(), result)
}

// Post sends a POST request with body and unmarshals the response.
//
// Example:
//
//	body := CreateUserRequest{ Name: "John" }
//	var result CreateUserResponse
//	err := cli.Post(ctx, "/users", body, &result)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) Post(ctx context.Context, path string, body interface{}, result interface{}) error {
	resp, err := c.R().
		SetContext(ctx).
		SetBody(body).
		Post(path)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusCreated {
		return errors.New("request failed with status: " + resp.Status())
	}

	return json.Unmarshal(resp.Body(), result)
}

// Put sends a PUT request with body and unmarshals the response.
//
// Example:
//
//	body := UpdateUserRequest{ Age: 30 }
//	var result UpdateUserResponse
//	err := cli.Put(ctx, "/users/1", body, &result)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) Put(ctx context.Context, path string, body interface{}, result interface{}) error {
	resp, err := c.R().
		SetContext(ctx).
		SetBody(body).
		Put(path)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return errors.New("request failed with status: " + resp.Status())
	}

	return json.Unmarshal(resp.Body(), result)
}

// Delete sends a DELETE request.
//
// Example:
//
//	err := cli.Delete(ctx, "/users/1")
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) Delete(ctx context.Context, path string) error {
	resp, err := c.R().
		SetContext(ctx).
		Delete(path)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusNoContent {
		return errors.New("request failed with status: " + resp.Status())
	}

	return nil
}

// Download downloads a file and writes it to the provided writer.
//
// Example:
//
//	file, _ := os.Create("downloaded_file.zip")
//	defer file.Close()
//	err := cli.Download(ctx, "/files/file.zip", file)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) Download(ctx context.Context, path string, writer io.Writer) error {
	resp, err := c.R().
		SetContext(ctx).
		SetDoNotParseResponse(true).
		Get(path)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return errors.New("request failed with status: " + resp.Status())
	}

	_, err = io.Copy(writer, resp.RawBody())
	return err
}
