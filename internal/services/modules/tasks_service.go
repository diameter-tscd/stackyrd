package modules

import (
	"errors"
	"strconv"

	"stackyrd/config"
	"stackyrd/pkg/infrastructure"
	"stackyrd/pkg/interfaces"
	"stackyrd/pkg/logger"
	"stackyrd/pkg/registry"
	"stackyrd/pkg/request"
	"stackyrd/pkg/response"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type Task struct {
	gorm.Model
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
}

type TasksService struct {
	db      *infrastructure.PostgresManager
	logger  *logger.Logger
	enabled bool
}

func NewTasksService(db *infrastructure.PostgresManager, enabled bool, logger *logger.Logger) *TasksService {
	if enabled && db != nil && db.ORM != nil {
		if err := db.ORM.AutoMigrate(&Task{}); err != nil {
			logger.Error("Error migrating Task model", err)
		}
	}
	return &TasksService{
		db:      db,
		logger:  logger,
		enabled: enabled,
	}
}

func (s *TasksService) Name() string     { return "Tasks Service" }
func (s *TasksService) WireName() string { return "tasks-service" }

func (s *TasksService) Enabled() bool {
	return s.enabled && s.db != nil && s.db.ORM != nil
}

func (s *TasksService) Get() any { return s }

func (s *TasksService) Endpoints() []string { return []string{"/tasks"} }

func (s *TasksService) RegisterRoutes(g *echo.Group) {
	sub := g.Group("/tasks")
	sub.GET("", s.listTasks)
	sub.POST("", s.createTask)
	sub.PUT("/:id", s.updateTask)
	sub.DELETE("/:id", s.deleteTask)
}

func (s *TasksService) listTasks(c echo.Context) error {
	var req response.PaginationRequest
	if err := request.Bind(c, &req); err != nil {
		return response.BadRequest(c, "Invalid pagination parameters")
	}

	ctx := c.Request().Context()

	var total int64
	if err := s.db.ORM.WithContext(ctx).Model(&Task{}).Count(&total).Error; err != nil {
		s.logger.Error("Failed to count tasks", err)
		return response.InternalServerError(c, "Failed to list tasks")
	}

	var tasks []Task
	result := s.db.ORM.WithContext(ctx).
		Order("id desc").
		Limit(req.GetPerPage()).
		Offset(req.GetOffset()).
		Find(&tasks)
	if result.Error != nil {
		s.logger.Error("Failed to list tasks", result.Error)
		return response.InternalServerError(c, "Failed to list tasks")
	}

	perPage := req.GetPerPage()
	meta := &response.Meta{
		Page:       req.GetPage(),
		PerPage:    perPage,
		Total:      total,
		TotalPages: int((total + int64(perPage) - 1) / int64(perPage)),
	}

	return response.SuccessWithMeta(c, tasks, meta)
}

func (s *TasksService) createTask(c echo.Context) error {
	task := new(Task)
	if err := request.Bind(c, task); err != nil {
		return response.BadRequest(c, "Invalid input")
	}

	result := s.db.ORM.WithContext(c.Request().Context()).Create(task)
	if result.Error != nil {
		s.logger.Error("Failed to create task", result.Error)
		return response.InternalServerError(c, "Failed to create task")
	}

	return response.Created(c, task)
}

func (s *TasksService) updateTask(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid task ID")
	}
	var task Task

	result := s.db.ORM.WithContext(c.Request().Context()).First(&task, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return response.NotFound(c, "Task not found")
		}
		s.logger.Error("Failed to fetch task", result.Error, "id", id)
		return response.InternalServerError(c, "Failed to fetch task")
	}

	// Bind into a fresh struct: the body must never override the ID resolved
	// from the URL (a body-supplied "id" would otherwise retarget the update).
	var payload Task
	if err := request.Bind(c, &payload); err != nil {
		return response.BadRequest(c, "Invalid input")
	}

	// Map-based update so zero values (e.g. Completed=false) are applied
	// instead of being skipped by Updates(struct).
	result = s.db.ORM.WithContext(c.Request().Context()).Model(&task).Updates(map[string]any{
		"title":       payload.Title,
		"description": payload.Description,
		"completed":   payload.Completed,
	})
	if result.Error != nil {
		s.logger.Error("Failed to update task", result.Error, "id", id)
		return response.InternalServerError(c, "Failed to update task")
	}

	task.Title = payload.Title
	task.Description = payload.Description
	task.Completed = payload.Completed

	return response.Success(c, task)
}

func (s *TasksService) deleteTask(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return response.BadRequest(c, "Invalid task ID")
	}

	result := s.db.ORM.WithContext(c.Request().Context()).Delete(&Task{}, "id = ?", id)
	if result.Error != nil {
		s.logger.Error("Failed to delete task", result.Error, "id", id)
		return response.InternalServerError(c, "Failed to delete task")
	}

	return response.Success(c, nil, "Task deleted")
}

func init() {
	registry.RegisterServiceWithDeps("tasks_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		helper := registry.NewServiceHelper(config, logger, deps)

		if !helper.IsServiceEnabled("tasks_service") {
			return nil
		}

		pgConnManager := deps.Postgres()
		if !helper.RequireDependency("Postgres Connection Manager", pgConnManager != nil) {
			return nil
		}

		db, ok := pgConnManager.GetDefaultConnection()
		if !ok {
			logger.Warn("Postgres default connection not available")
			return nil
		}

		return NewTasksService(db, true, logger)
	})
}
