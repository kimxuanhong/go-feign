package feign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
)

type Client struct {
	BaseURL string
}

func NewClient(baseURL string) *Client {
	return &Client{BaseURL: baseURL}
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

// Create gán các hàm vào struct target (ví dụ: *UserClient)
func (c *Client) Create(target any) {
	t := reflect.TypeOf(target).Elem()
	v := reflect.ValueOf(target).Elem()

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
			var body io.ReadCloser = http.NoBody

			if bodyParam != "" {
				jsonBytes, err := json.Marshal(args[j].Interface())
				if err != nil {
					out0 := reflect.Zero(methodType.Out(0))
					out1 := reflect.ValueOf(fmt.Errorf("marshal body failed: %w", err))
					return []reflect.Value{out0, out1}
				}
				body = io.NopCloser(bytes.NewReader(jsonBytes))
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

			// Encode query
			query := make([]string, 0)
			for _, q := range queries {
				if j >= len(args) {
					break
				}
				query = append(query, fmt.Sprintf("%s=%s", q, url.QueryEscape(fmt.Sprintf("%v", args[j].Interface()))))
				j++
			}
			if len(query) > 0 {
				pathProcessed += "?" + strings.Join(query, "&")
			}

			req, err := http.NewRequest(httpMethod, c.BaseURL+pathProcessed, body)
			if err != nil {
				out0 := reflect.Zero(methodType.Out(0))
				out1 := reflect.ValueOf(fmt.Errorf("create request failed: %w", err))
				return []reflect.Value{out0, out1}
			}

			if bodyParam != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			for _, h := range headers {
				if j >= len(args) {
					break
				}
				req.Header.Set(h, fmt.Sprintf("%v", args[j].Interface()))
				j++
			}

			// Log request
			fmt.Printf("➡️ Request: %s %s\n", req.Method, req.URL.String())
			if bodyParam != "" {
				b, _ := io.ReadAll(body)
				fmt.Println("📝 Body:", string(b))
				body = io.NopCloser(bytes.NewReader(b)) // reset body
				req.Body = body
			}
			for k, v := range req.Header {
				fmt.Printf("🔐 Header: %s = %s\n", k, strings.Join(v, ", "))
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				// Lỗi kết nối: không có server phản hồi
				out0 := reflect.Zero(methodType.Out(0))
				httpErr := &HttpError{
					StatusCode: 0, // không phải HTTP
					Status:     "connection failed",
					Body:       err.Error(),
				}
				return []reflect.Value{out0, reflect.ValueOf(httpErr)}
			}
			defer func(Body io.ReadCloser) {
				_ = Body.Close()
			}(resp.Body)

			respBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				out0 := reflect.Zero(methodType.Out(0))
				out1 := reflect.ValueOf(fmt.Errorf("read response failed: %w", err))
				return []reflect.Value{out0, out1}
			}

			// Trả lỗi nếu không phải 2xx
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				out0 := reflect.Zero(methodType.Out(0))
				httpErr := &HttpError{
					StatusCode: resp.StatusCode,
					Status:     resp.Status,
					Body:       string(respBytes),
				}
				out1 := reflect.ValueOf(httpErr)
				return []reflect.Value{out0, out1}
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
