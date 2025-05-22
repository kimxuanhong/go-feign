package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/kimxuanhong/go-feign/feign"
	"github.com/kimxuanhong/go-utils/config"
	"time"
)

type User struct {
	ID        string    `json:"id"`
	PartnerId string    `json:"partner_id"`
	Total     int       `json:"total"`
	UserName  string    `json:"user_name"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserClient struct {
	_            struct{}                                                                                                               `feign:"@Url http://localhost:8081/api/v1"`
	GetUser      func(ctx context.Context, id string, auth string) (*User, error)                                                       `feign:"@GET /users/{id} | @Path id | @Header Authorization"`
	GetUserById  func(ctx context.Context, user string, id string, auth string) (*User, error)                                          `feign:"@GET /users/{user} | @Path user | @Query id | @Header Authorization"`
	GetUserByIds func(ctx context.Context, user string, queries map[string]string, headers map[string]string, id string) (*User, error) `feign:"@GET /users/{user} | @Path user | @Queries queries | @Headers headers | @Query id"`
	CreateUser   func(ctx context.Context, user User, auth string) (*User, error)                                                       `feign:"@POST /users | @Body user | @Header Authorization"`
	UpdateUser   func(ctx context.Context, user User, auth string) (*User, error)                                                       `feign:"@POST /users | @Body user | @Header Authorization"`
	GetAllUser   func(ctx context.Context, auth string) ([]User, error)                                                                 `feign:"@POST /users | @Header Authorization"`
}

func main() {
	config.LoadConfigFile()
	client := &UserClient{} // KHỞI TẠO
	feignClient := feign.NewClient()
	feignClient.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
		fmt.Println("Request:", r.Method, r.URL)
		// Thêm header chung
		r.SetHeader("X-Request-ID", "some-id")
		return nil
	})

	// Thêm interceptor sau response
	feignClient.OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
		fmt.Println("Response status:", r.Status())
		return nil
	})
	feignClient.Create(client) // OK

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	//user, err := client.GetUser(ctx, "123", "token") // gọi được, vì func đã được gán
	//fmt.Println(user, err)
	headers := map[string]string{
		"Authorization": "Bearer token456",
		"X-Custom":      "custom-value",
		"ApiKey":        "apikey-value",
	}

	queries := map[string]string{
		"Hong": "hong",
		"Kim":  "kim",
		"Xuan": "xuan",
	}
	user2, err := client.GetUserByIds(ctx, "123", queries, headers, "hong kim") // gọi được, vì func đã được gán
	fmt.Println(user2, err)

	//newUser := User{UserName: "Alice"}
	//createdUser, err := client.CreateUser(newUser, "Bearer xyz")
	//fmt.Println(createdUser, err)

	if err != nil {
		var httpErr *feign.HttpError
		if errors.As(err, &httpErr) {
			fmt.Println("📛 HTTP Error:", httpErr.StatusCode)
			fmt.Println("📄 Body:", httpErr.Body)
		} else {
			fmt.Println("❗️Other Error:", err)
		}
	}
}
