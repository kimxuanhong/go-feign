package feign

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"reflect"
	"runtime"
	"strings"
)

type MethodMeta struct {
	Method    string
	Path      string
	PathVars  []string
	Headers   []string
	Queries   []string
	BodyParam string
}

type Client struct {
	*resty.Client
	baseURL string
	headers map[string]string
	Config  *Config
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

type HttpError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HttpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s - %s", e.StatusCode, e.Status, e.Body)
}

func resolveUrl(value string) string {
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return viper.GetString(value)
}

type BaseClient interface {
	BaseUrl() string
}

func (c *Client) Create(target BaseClient) {
	t := reflect.TypeOf(target).Elem()
	v := reflect.ValueOf(target).Elem()

	filePath, err := getFilePathOfStruct(target)
	if err != nil {
		log.Fatalf("failed to get file path of struct %T: %v", target, err)
	}

	metaMap, err := parseStructFuncTags(filePath, t.Name())
	if err != nil {
		log.Fatalf("failed to parse tags for %T: %v", target, err)
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if field.Type.Kind() != reflect.Func {
			continue
		}

		methodType := field.Type

		if methodType.NumIn() < 1 {
			panic(fmt.Sprintf("method %s must have context.Context as first parameter", field.Name))
		}
		if !methodType.In(0).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
			panic(fmt.Sprintf("method %s must take context.Context as first parameter", field.Name))
		}
		if methodType.NumOut() != 2 || !methodType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			panic(fmt.Sprintf("method %s must return (T, error)", field.Name))
		}

		meta := metaMap[field.Name]
		if meta.Method == "" || meta.Path == "" {
			panic(fmt.Sprintf("missing HTTP method or path in %s", field.Name))
		}

		fn := reflect.MakeFunc(methodType, func(args []reflect.Value) []reflect.Value {
			ctx := args[0].Interface().(context.Context)

			j := 1
			var body any
			if meta.BodyParam != "" {
				body = args[j].Interface()
				j++
			}

			pathProcessed := meta.Path
			for _, p := range meta.PathVars {
				pathProcessed = strings.ReplaceAll(pathProcessed, fmt.Sprintf("{%s}", p), fmt.Sprintf("%v", args[j].Interface()))
				j++
			}

			queryParams := map[string]string{}
			for _, q := range meta.Queries {
				queryParams[q] = fmt.Sprintf("%v", args[j].Interface())
				j++
			}

			headersMap := map[string]string{}
			for _, h := range meta.Headers {
				headersMap[h] = fmt.Sprintf("%v", args[j].Interface())
				j++
			}

			r := c.R().SetContext(ctx)
			for k, v := range c.headers {
				r.SetHeader(k, v)
			}
			for k, v := range headersMap {
				r.SetHeader(k, v)
			}
			if body != nil {
				r.SetHeader("Content-Type", "application/json")
				r.SetBody(body)
			}
			if len(queryParams) > 0 {
				r.SetQueryParams(queryParams)
			}

			resp, err := r.Execute(meta.Method, pathProcessed)
			if err != nil {
				return []reflect.Value{reflect.Zero(methodType.Out(0)), reflect.ValueOf(&HttpError{0, "connection error", err.Error()})}
			}

			if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
				return []reflect.Value{reflect.Zero(methodType.Out(0)), reflect.ValueOf(&HttpError{
					StatusCode: resp.StatusCode(),
					Status:     resp.Status(),
					Body:       string(resp.Body()),
				})}
			}

			out := reflect.New(methodType.Out(0).Elem())
			err = json.Unmarshal(resp.Body(), out.Interface())
			if err != nil {
				return []reflect.Value{reflect.Zero(methodType.Out(0)), reflect.ValueOf(fmt.Errorf("unmarshal failed: %w", err))}
			}
			return []reflect.Value{out, reflect.Zero(methodType.Out(1))}
		})

		v.Field(i).Set(fn)
	}
}

func getFilePathOfStruct(i interface{}) (string, error) {
	typ := reflect.TypeOf(i)
	if typ.NumMethod() == 0 {
		return "", fmt.Errorf("struct %T has no methods", i)
	}
	pc := typ.Method(0).Func.Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "", fmt.Errorf("cannot find function for struct")
	}
	file, _ := fn.FileLine(pc)
	return file, nil
}

// Refactored parser
func parseStructFuncTags(filePath, structName string) (map[string]MethodMeta, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	result := map[string]MethodMeta{}
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}

		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != structName {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			for _, field := range structType.Fields.List {
				_, ok := field.Type.(*ast.FuncType)
				if !ok || len(field.Names) == 0 {
					continue
				}

				methodName := field.Names[0].Name
				meta := MethodMeta{}

				if field.Doc != nil {
					if err := parseComment(field.Doc.List, &meta); err != nil {
						return nil, fmt.Errorf("invalid comment in %s: %w", methodName, err)
					}
				}
				result[methodName] = meta
			}
		}
	}
	return result, nil
}

// Parse and validate tags
func parseComment(comments []*ast.Comment, meta *MethodMeta) error {
	seen := map[string]bool{}
	for _, comment := range comments {
		if !strings.HasPrefix(comment.Text, "// @") {
			continue
		}
		parts := strings.Fields(strings.TrimPrefix(comment.Text, "// "))
		if len(parts) < 2 {
			continue
		}
		tag, value := strings.ToUpper(parts[0][1:]), parts[1]

		if seen[tag] {
			return fmt.Errorf("duplicate tag: @%s", tag)
		}
		seen[tag] = true

		switch tag {
		case "GET", "POST", "PUT", "DELETE":
			meta.Method = tag
			meta.Path = value
		case "PATH":
			meta.PathVars = append(meta.PathVars, value)
		case "QUERY":
			meta.Queries = append(meta.Queries, value)
		case "HEADER":
			meta.Headers = append(meta.Headers, value)
		case "BODY":
			meta.BodyParam = value
		default:
			log.Printf("⚠️ Unknown tag @%s", tag)
		}
	}
	if meta.Method == "" || meta.Path == "" {
		return fmt.Errorf("missing HTTP method or path")
	}
	return nil
}
