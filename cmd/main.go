package main

import (
	"errors"
	"fmt"
	"github.com/kimxuanhong/go-feign/feign"
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
	GetUser     func(id string, auth string) (*User, error)              `feign:"@GET /api/v1/users/{id} | @Path id | @Header Authorization"`
	GetUserById func(user string, id string, auth string) (*User, error) `feign:"@GET /api/v1/users/{user} | @Path user | @Query id | @Header Authorization"`
	CreateUser  func(user User, auth string) (*User, error)              `feign:"@POST /api/v1/users | @Body user | @Header Authorization"`
}

func main() {
	client := &UserClient{} // KHỞI TẠO
	feignClient := &feign.Client{BaseURL: "http://localhost:8081"}
	feignClient.Create(client) // OK

	//user, err := client.GetUser("123", "token") // gọi được, vì func đã được gán
	//fmt.Println(user, err)

	user2, err := client.GetUserById("123", "hong kim", "token") // gọi được, vì func đã được gán
	fmt.Println(user2, err)

	newUser := User{UserName: "Alice"}
	createdUser, err := client.CreateUser(newUser, "Bearer xyz")
	if err != nil {
		var httpErr *feign.HttpError
		if errors.As(err, &httpErr) {
			fmt.Println("📛 HTTP Error:", httpErr.StatusCode)
			fmt.Println("📄 Body:", httpErr.Body)
		} else {
			fmt.Println("❗️Other Error:", err)
		}
	}

	fmt.Println(createdUser, err)
}
