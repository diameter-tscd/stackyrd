package modules

import (
	"context"
	"strconv"

	"stackyard/config"
	"stackyard/pkg/infrastructure"
	"stackyard/pkg/interfaces"
	"stackyard/pkg/logger"
	"stackyard/pkg/registry"
	"stackyard/pkg/response"

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
		// Auto-migrate the schema
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
	// Service is enabled only if configured AND DB is available
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

// listTasks godoc
// @Summary List all tasks
// @Description Retrieve all tasks from the database
// @Tags tasks
// @Accept json
// @Produce json
// @Success 200 {object} response.Response "Tasks retrieved successfully"
// @Failure 500 {object} response.Response "Failed to retrieve tasks"
// @Router /tasks [get]
func (s *TasksService) listTasks(c echo.Context) error {
	var tasks []Task

	// Use async GORM operation to avoid blocking main thread
	result := s.db.GORMFindAsync(context.Background(), &tasks)

	// Wait for the async operation to complete
	_, err := result.Wait()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Success(c, tasks)
}

// createTask godoc
// @Summary Create a new task
// @Description Create a new task in the database
// @Tags tasks
// @Accept json
// @Produce json
// @Param request body Task true "Task to create"
// @Success 201 {object} response.Response "Task created successfully"
// @Failure 400 {object} response.Response "Invalid input"
// @Failure 500 {object} response.Response "Failed to create task"
// @Router /tasks [post]
func (s *TasksService) createTask(c echo.Context) error {
	task := new(Task)
	if err := c.Bind(task); err != nil {
		return response.BadRequest(c, "Invalid input")
	}

	// Use async GORM operation to avoid blocking main thread
	result := s.db.GORMCreateAsync(context.Background(), task)

	// Wait for the async operation to complete
	_, err := result.Wait()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Created(c, task)
}

// updateTask godoc
// @Summary Update a task
// @Description Update an existing task by ID
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path int true "Task ID"
// @Param request body Task true "Task data to update"
// @Success 200 {object} response.Response "Task updated successfully"
// @Failure 400 {object} response.Response "Invalid input"
// @Failure 404 {object} response.Response "Task not found"
// @Failure 500 {object} response.Response "Failed to update task"
// @Router /tasks/{id} [put]
func (s *TasksService) updateTask(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var task Task

	// First check if task exists using async operation
	findResult := s.db.GORMFirstAsync(context.Background(), &task, id)
	_, err := findResult.Wait()
	if err != nil {
		return response.NotFound(c, "Task not found")
	}

	if err := c.Bind(&task); err != nil {
		return response.BadRequest(c, "Invalid input")
	}

	// Use async GORM update operation
	updateResult := s.db.GORMUpdateAsync(context.Background(), &task, task, "id = ?", id)
	_, err = updateResult.Wait()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Success(c, task)
}

// deleteTask godoc
// @Summary Delete a task
// @Description Delete a task by ID
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path int true "Task ID"
// @Success 200 {object} response.Response "Task deleted successfully"
// @Failure 500 {object} response.Response "Failed to delete task"
// @Router /tasks/{id} [delete]
func (s *TasksService) deleteTask(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var task Task

	// Use async GORM delete operation
	result := s.db.GORMDeleteAsync(context.Background(), &task, "id = ?", id)

	// Wait for the async operation to complete
	_, err := result.Wait()
	if err != nil {
		return response.InternalServerError(c, err.Error())
	}

	return response.Success(c, nil, "Task deleted")
}

// Auto-registration function - called when package is imported
func init() {
	registry.RegisterService("tasks_service", func(config *config.Config, logger *logger.Logger, deps *registry.Dependencies) interfaces.Service {
		helper := registry.NewServiceHelper(config, logger, deps)

		if !helper.IsServiceEnabled("tasks_service") {
			return nil
		}

		postgresManager, ok := helper.GetPostgres()
		if !helper.RequireDependency("PostgresManager", ok) {
			return nil
		}

		return NewTasksService(postgresManager, true, logger)
	})
}
