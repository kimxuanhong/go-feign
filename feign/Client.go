package feign

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"reflect"
	"strings"

	"github.com/go-resty/resty/v2"
)

type Client interface {
	Create(target any)
}

type feignClient struct {
	baseURL     string
	headers     map[string]string
	restyClient *resty.Client
}

func NewClient(configs ...*Config) Client {
	cfg := GetConfig(configs...)
	return &feignClient{
		baseURL: cfg.Url,
		headers: cfg.Headers,
		restyClient: resty.New().
			SetTimeout(cfg.Timeout).
			SetRetryCount(cfg.RetryCount).
			SetRetryWaitTime(cfg.RetryWait),
	}
}

// HttpError giúp phân biệt lỗi HTTP như 401, 404, 500
type HttpError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s - %s", e.StatusCode, e.Status, e.Body)
}

// Nếu value bắt đầu bằng http/https thì dùng luôn, ngược lại tra từ Viper
func resolveUrl(value string) string {
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return viper.GetString(value) // nếu không có thì trả về ""
}

// Create gán các hàm vào struct target (ví dụ: *UserClient)
func (c *feignClient) Create(target any) {
	t := reflect.TypeOf(target).Elem()
	v := reflect.ValueOf(target).Elem()

	// Tìm field dummy có tag @Url
	baseUrl := c.baseURL
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type == reflect.TypeOf(struct{}{}) {
			tag := field.Tag.Get("feign")
			for _, line := range strings.Split(tag, "|") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "@Url") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						baseUrl = resolveUrl(parts[1])
						break
					}
				}
			}
			if baseUrl != c.baseURL {
				break
			}
		}
	}
	c.restyClient.SetBaseURL(baseUrl)

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Type.Kind() != reflect.Func {
			continue
		}

		methodType := field.Type

		if methodType.NumOut() != 2 || !methodType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			panic(fmt.Sprintf("method %s must return (*T, error)", field.Name))
		}

		doc := field.Tag.Get("feign")

		var httpMethod, path, bodyParam string
		var paths, headers, queries []string

		for _, line := range strings.Split(doc, "|") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "@") {
				parts := strings.Fields(line)
				if len(parts) < 2 {
					continue
				}
				tag := strings.TrimPrefix(parts[0], "@")
				value := parts[1]

				switch strings.ToUpper(tag) {
				case "GET", "POST", "PUT", "DELETE":
					httpMethod = strings.ToUpper(tag)
					path = value
				case "PATH":
					paths = append(paths, value)
				case "HEADER":
					headers = append(headers, value)
				case "BODY":
					bodyParam = value
				case "QUERY":
					queries = append(queries, value)
				}
			}
		}

		fn := reflect.MakeFunc(methodType, func(args []reflect.Value) []reflect.Value {
			j := 0
			var body interface{} = nil

			if bodyParam != "" {
				body = args[j].Interface()
				j++
			}

			// Thay thế {param} trong path
			pathProcessed := path
			for _, p := range paths {
				if j >= len(args) {
					break
				}
				placeholder := fmt.Sprintf("{%s}", p)
				pathProcessed = strings.ReplaceAll(pathProcessed, placeholder, fmt.Sprintf("%v", args[j].Interface()))
				j++
			}

			// Chuẩn bị query params
			queryParams := map[string]string{}
			for _, q := range queries {
				if j >= len(args) {
					break
				}
				queryParams[q] = fmt.Sprintf("%v", args[j].Interface())
				j++
			}

			// Chuẩn bị headers
			headersMap := map[string]string{}
			for _, h := range headers {
				if j >= len(args) {
					break
				}
				headersMap[h] = fmt.Sprintf("%v", args[j].Interface())
				j++
			}

			// Tạo request Resty
			r := c.restyClient.R()

			// Set headers
			if c.headers == nil {
				c.headers = make(map[string]string)
			}
			for k, v := range headersMap {
				c.headers[k] = v
			}

			// Set headers từ config
			for k, v := range c.headers {
				r.SetHeader(k, v)
			}

			if bodyParam != "" {
				r.SetHeader("Content-Type", "application/json")
				r.SetBody(body)
			}

			// Set query params
			if len(queryParams) > 0 {
				r.SetQueryParams(queryParams)
			}

			// Log request info
			fmt.Printf("➡️ Request: %s %s\n", httpMethod, baseUrl+pathProcessed)
			if bodyParam != "" {
				bodyJson, _ := json.Marshal(body)
				fmt.Println("📝 Body:", string(bodyJson))
			}
			for k, v := range c.headers {
				fmt.Printf("🔐 Header: %s = %s\n", k, v)
			}
			if len(queryParams) > 0 {
				fmt.Printf("🔍 Query: %+v\n", queryParams)
			}

			resp, err := r.Execute(httpMethod, pathProcessed)
			if err != nil {
				out0 := reflect.Zero(methodType.Out(0))
				httpErr := &HttpError{
					StatusCode: 0,
					Status:     "connection failed",
					Body:       err.Error(),
				}
				return []reflect.Value{out0, reflect.ValueOf(httpErr)}
			}

			respBytes := resp.Body()

			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				out0 := reflect.Zero(methodType.Out(0))
				httpErr := &HttpError{
					StatusCode: resp.StatusCode(),
					Status:     resp.Status(),
					Body:       string(respBytes),
				}
				return []reflect.Value{out0, reflect.ValueOf(httpErr)}
			}

			// Unmarshal
			out := reflect.New(methodType.Out(0).Elem())
			err = json.Unmarshal(respBytes, out.Interface())
			if err != nil {
				fmt.Println("❌ JSON Decode Error:", err)
				fmt.Println("📦 Raw Response:", string(respBytes))
				out0 := reflect.Zero(methodType.Out(0))
				out1 := reflect.ValueOf(fmt.Errorf("unmarshal failed: %w", err))
				return []reflect.Value{out0, out1}
			}
			fmt.Println("📦 Response:", string(respBytes))
			return []reflect.Value{out, reflect.Zero(methodType.Out(1))}
		})

		v.Field(i).Set(fn)
	}
}
