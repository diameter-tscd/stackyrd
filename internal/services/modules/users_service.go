package modules

import (
	"strconv"

	"stackyrd/config"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
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

func (s *UsersService) RegisterRoutes(g *gin.RouterGroup) {
	sub := g.Group("/users")
	{
		// @Summary List users with pagination
		// @Description Get a paginated list of users
		// @Tags users
		// @Accept json
		// @Produce json
		// @Param page query int false "Page number" default(1)
		// @Param per_page query int false "Items per page" default(10)
		// @Success 200 {object} response.Response{data=[]User} "Success"
		// @Failure 400 {object} response.Response "Bad request"
		// @Router /users [get]
		sub.GET("", s.listUsers)

		// @Summary Get user by ID
		// @Description Get a specific user by ID
		// @Tags users
		// @Accept json
		// @Produce json
		// @Param id path int true "User ID"
		// @Success 200 {object} response.Response{data=User} "Success"
		// @Failure 404 {object} response.Response "User not found"
		// @Router /users/{id} [get]
		sub.GET("/:id", s.getUser)

		// @Summary Create user
		// @Description Create a new user
		// @Tags users
		// @Accept json
		// @Produce json
		// @Param user body User true "User data"
		// @Success 201 {object} response.Response{data=User} "User created"
		// @Failure 400 {object} response.Response "Invalid input"
		// @Failure 422 {object} response.Response "Validation error"
		// @Router /users [post]
		sub.POST("", s.createUser)

		// @Summary Update user
		// @Description Update an existing user
		// @Tags users
		// @Accept json
		// @Produce json
		// @Param id path int true "User ID"
		// @Param user body User true "User data"
		// @Success 200 {object} response.Response{data=User} "User updated"
		// @Failure 400 {object} response.Response "Invalid input"
		// @Failure 404 {object} response.Response "User not found"
		// @Router /users/{id} [put]
		sub.PUT("/:id", s.updateUser)

		// DELETE is blocked by PermissionCheck middleware
	}
}

// Mock database
var users = []User{
	{ID: 1, Name: "Alice", Email: "alice@example.com", Phone: "+1234567890", Username: "alice123", Age: 30},
	{ID: 2, Name: "Bob", Email: "bob@example.com", Phone: "+0987654321", Username: "bob456", Age: 25},
}

func (s *UsersService) listUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	if page > len(users) {
		response.BadRequest(c, "Invalid pagination parameters")
		return
	}

	start := (page - 1) * perPage
	end := start + perPage
	if end > len(users) {
		end = len(users)
	}

	usersPage := users[start:end]
	meta := response.CalculateMeta(page, perPage, int64(len(users)))
	response.SuccessWithMeta(c, usersPage, meta, "Users retrieved successfully")
}

func (s *UsersService) getUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	for _, user := range users {
		if user.ID == id {
			response.Success(c, user, "User retrieved successfully")
			return
		}
	}

	response.NotFound(c, "User not found")
}

func (s *UsersService) createUser(c *gin.Context) {
	var user User
	if err := request.Bind(c, &user); err != nil {
		if validationErr, ok := err.(*request.ValidationError); ok {
			response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
		} else {
			response.BadRequest(c, err.Error())
		}
		return
	}

	// Assign new ID
	user.ID = len(users) + 1
	users = append(users, user)

	response.Created(c, user, "User created successfully")
}

func (s *UsersService) updateUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var user User
	if err := request.Bind(c, &user); err != nil {
		if validationErr, ok := err.(*request.ValidationError); ok {
			response.ValidationError(c, "Validation failed", validationErr.GetFieldErrors())
		} else {
			response.BadRequest(c, err.Error())
		}
		return
	}

	for i, u := range users {
		if u.ID == id {
			user.ID = id
			users[i] = user
			response.Success(c, user, "User updated successfully")
			return
		}
	}

	response.NotFound(c, "User not found")
}

// Auto-registration function - called when package is imported
func init() {
	// Service registration is handled by the registry package
	registry.RegisterService("users_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		return NewUsersService(config.Services.IsEnabled("users_service"), logger)
	})
}

// ValidateAge is a custom validator for age
func ValidateAge(fl validator.FieldLevel) bool {
	age, ok := fl.Field().Interface().(int)
	if !ok {
		return false
	}
	return age >= 0 && age <= 130
}
