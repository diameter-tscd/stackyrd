package modules

import (
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

func (s *TasksService) Get() interface{} { return s }

func (s *TasksService) Endpoints() []string { return []string{"/tasks"} }

func (s *TasksService) RegisterRoutes(g *echo.Group) {
	sub := g.Group("/tasks")
	sub.GET("", s.listTasks)
	sub.POST("", s.createTask)
	sub.PUT("/:id", s.updateTask)
	sub.DELETE("/:id", s.deleteTask)
}

func (s *TasksService) listTasks(c echo.Context) error {
	var tasks []Task

	result := s.db.ORM.WithContext(c.Request().Context()).Find(&tasks)
	if result.Error != nil {
		return response.InternalServerError(c, result.Error.Error())
	}

	return response.Success(c, tasks)
}

func (s *TasksService) createTask(c echo.Context) error {
	task := new(Task)
	if err := request.Bind(c, task); err != nil {
		return response.BadRequest(c, "Invalid input")
	}

	result := s.db.ORM.WithContext(c.Request().Context()).Create(task)
	if result.Error != nil {
		return response.InternalServerError(c, result.Error.Error())
	}

	return response.Created(c, task)
}

func (s *TasksService) updateTask(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var task Task

	result := s.db.ORM.WithContext(c.Request().Context()).First(&task, id)
	if result.Error != nil {
		return response.NotFound(c, "Task not found")
	}

	if err := request.Bind(c, &task); err != nil {
		return response.BadRequest(c, "Invalid input")
	}

	result = s.db.ORM.WithContext(c.Request().Context()).Model(&task).Updates(task)
	if result.Error != nil {
		return response.InternalServerError(c, result.Error.Error())
	}

	return response.Success(c, task)
}

func (s *TasksService) deleteTask(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))

	result := s.db.ORM.WithContext(c.Request().Context()).Delete(&Task{}, "id = ?", id)
	if result.Error != nil {
		return response.InternalServerError(c, result.Error.Error())
	}

	return response.Success(c, nil, "Task deleted")
}

func init() {
	registry.RegisterService("tasks_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		helper := registry.NewServiceHelper(config, logger, deps)

		if !helper.IsServiceEnabled("tasks_service") {
			return nil
		}

		postgresManager, ok := registry.GetTyped[infrastructure.PostgresManager](deps, "postgres")
		if !helper.RequireDependency("PostgresManager", ok) {
			return nil
		}

		return NewTasksService(&postgresManager, true, logger)
	})
}
