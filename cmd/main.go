package main

import (
	"context"
	"errors"
	"fmt"
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
	//_ struct{} `feign:"@Url http://localhost:8081/api/v1"`
	//GetUser     func(ctx context.Context, id string, auth string) (*User, error)              `feign:"@GET /users/{id} | @Path id | @Header Authorization"`

	// @GET http://localhost:8081/api/v1/users/{user}
	// @Path user
	// @Query id
	// @Header auth
	GetUserById func(ctx context.Context, user string, id string, auth string) (*User, error) `feign:"@GET /users/{user} | @Path user | @Query id | @Header Authorization"`
	//CreateUser  func(ctx context.Context, user User, auth string) (*User, error)              `feign:"@POST /users | @Body user | @Header Authorization"`
	//UpdateUser  func(ctx context.Context, user User, auth string) (*User, error)              `feign:"@POST /users | @Body user | @Header Authorization"`
	//GetAllUser  func(ctx context.Context, auth string) ([]User, error)                        `feign:"@POST /users | @Header Authorization"`
}

func (c *UserClient) BaseUrl() string {
	return "Nam"
}

func main() {
	config.LoadConfigFile()
	client := &UserClient{} // KHỞI TẠO
	feignClient := feign.NewClient()
	feignClient.Create(client) // OK

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	//user, err := client.GetUser(ctx, "123", "token") // gọi được, vì func đã được gán
	//fmt.Println(user, err)

	user2, err := client.GetUserById(ctx, "123", "hong kim", "token") // gọi được, vì func đã được gán
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
