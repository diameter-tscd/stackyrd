package modules

import (
	"strconv"
	"sync"

	"stackyrd/config"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type UsersService struct {
	enabled bool
	logger  *logger.Logger
}

type User struct {
	ID       int    `json:"id" uri:"id"`
	Name     string `json:"name" validate:"required"`
	Email    string `json:"email" validate:"required,email"`
	Phone    string `json:"phone" validate:"phone"`
	Username string `json:"username" validate:"username"`
	Age      int    `json:"age" validate:"gte=0,lte=130"`
}

func NewUsersService(enabled bool, logger *logger.Logger) *UsersService {
	return &UsersService{
		enabled: enabled,
		logger:  logger,
	}
}

func (s *UsersService) Name() string {
	return "Users Service"
}

func (s *UsersService) WireName() string {
	return "users"
}

func (s *UsersService) Enabled() bool {
	return s.enabled
}

func (s *UsersService) Endpoints() []string {
	return []string{
		"/users",
		"/users/:id",
	}
}

func (s *UsersService) Get() interface{} {
	return s
}

func (s *UsersService) RegisterRoutes(g *echo.Group) {
	sub := g.Group("/users")
	{
		sub.GET("", s.listUsers)
		sub.GET("/:id", s.getUser)
		sub.POST("", s.createUser)
		sub.PUT("/:id", s.updateUser)
	}
}

var (
	usersMu   sync.RWMutex
	usersList = []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com", Phone: "+1234567890", Username: "alice123", Age: 30},
		{ID: 2, Name: "Bob", Email: "bob@example.com", Phone: "+0987654321", Username: "bob456", Age: 25},
	}
	usersIdx = map[int]*User{
		1: &usersList[0],
		2: &usersList[1],
	}
)

func (s *UsersService) listUsers(c echo.Context) error {
	pageStr := c.QueryParam("page")
	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil {
			page = p
		}
	}

	perPageStr := c.QueryParam("per_page")
	perPage := 10
	if perPageStr != "" {
		if pp, err := strconv.Atoi(perPageStr); err == nil {
			perPage = pp
		}
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	totalUsers := func() int {
		usersMu.RLock()
		defer usersMu.RUnlock()
		return len(usersList)
	}()
	if page > totalUsers {
		return response.BadRequest(c, "Invalid pagination parameters")
	}

	usersMu.RLock()
	start := (page - 1) * perPage
	end := start + perPage
	if end > len(usersList) {
		end = len(usersList)
	}
	usersPage := make([]User, end-start)
	copy(usersPage, usersList[start:end])
	usersMu.RUnlock()

	meta := response.CalculateMeta(page, perPage, int64(len(usersList)))
	return response.SuccessWithMeta(c, usersPage, meta, "Users retrieved successfully")
}

func (s *UsersService) getUser(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))

	usersMu.RLock()
	u, ok := usersIdx[id]
	usersMu.RUnlock()
	if ok {
		return response.Success(c, *u, "User retrieved successfully")
	}

	return response.NotFound(c, "User not found")
}

func (s *UsersService) createUser(c echo.Context) error {
	var user User
	if err := request.Bind(c, &user); err != nil {
		if validationErr, ok := err.(*request.ValidationError); ok {
			return response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
		} else {
			return response.BadRequest(c, err.Error())
		}
	}

	usersMu.Lock()
	if len(usersList) >= maxUsers {
		usersMu.Unlock()
		return response.Error(c, 503, "SERVICE_UNAVAILABLE", "User limit reached", nil)
	}
	user.ID = len(usersList) + 1
	usersList = append(usersList, user)
	usersIdx[user.ID] = &usersList[len(usersList)-1]
	usersMu.Unlock()

	return response.Created(c, user, "User created successfully")
}

const maxUsers = 10000

func (s *UsersService) updateUser(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))

	var user User
	if err := request.Bind(c, &user); err != nil {
		if validationErr, ok := err.(*request.ValidationError); ok {
			return response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
		} else {
			return response.BadRequest(c, err.Error())
		}
	}

	usersMu.Lock()
	defer usersMu.Unlock()
	u, ok := usersIdx[id]
	if ok {
		user.ID = id
		*u = user
		return response.Success(c, user, "User updated successfully")
	}

	return response.NotFound(c, "User not found")
}

func init() {
	registry.RegisterService("users_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		return NewUsersService(config.Services.IsEnabled("users_service"), logger)
	})
}

func ValidateAge(fl validator.FieldLevel) bool {
	age, ok := fl.Field().Interface().(int)
	if !ok {
		return false
	}
	return age >= 0 && age <= 130
}
