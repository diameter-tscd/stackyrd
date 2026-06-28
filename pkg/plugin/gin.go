package plugin

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/spf13/afero"
)

func RegisterManagementRoutes(rg *echo.Group) {
	rg.GET("", handleList)
	rg.GET("/:name", handleGet)
	rg.POST("/:name/execute", handleExecute)
	rg.PUT("/:name/scripts/:file", handleUploadScript)
	rg.GET("/:name/scripts", handleListScripts)
	rg.GET("/:name/scripts/:file", handleGetScript)
	rg.DELETE("/:name", handleUnload)
	rg.GET("/manager/status", handleManagerStatus)
}

func handleManagerStatus(c echo.Context) error {
	reg := GetGlobalPluginRegistry()
	metrics := CollectMetrics(reg)
	return c.JSON(http.StatusOK, metrics)
}

func handleList(c echo.Context) error {
	reg := GetGlobalPluginRegistry()
	metas := reg.GetAllMetas()
	allStats := reg.GetAllStats()

	result := make([]map[string]interface{}, 0, len(metas))
	for name, meta := range metas {
		_, loaded := reg.Get(name)
		status := "registered"
		if loaded {
			status = "loaded"
		}

		entry := map[string]interface{}{
			"name":        name,
			"version":     meta.Version,
			"description": meta.Description,
			"status":      status,
			"type":        entrypointType(meta.Entrypoint),
		}

		if s, ok := allStats[name]; ok {
			entry["execute_count"] = s.ExecuteCount
			entry["load_time_ms"] = s.LoadTimeMs
			entry["file_size"] = s.EmbeddedFileSize
			if s.LastExecuteMs > 0 {
				entry["last_execution_ms"] = s.LastExecuteMs
			}
		}

		result = append(result, entry)
	}

	metrics := CollectMetrics(reg)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"plugins":        result,
		"total":          metrics.TotalPlugins,
		"loaded":         metrics.LoadedPlugins,
		"active_execs":   metrics.ActiveExecutions,
		"goroutines":     metrics.GoroutineCount,
		"memory_bytes":   metrics.MemoryUsageBytes,
		"memory_limit":   metrics.MemoryLimitBytes,
		"memory_percent": metrics.MemoryPercent,
		"uptime_seconds": metrics.UptimeSeconds,
	})
}

func handleGet(c echo.Context) error {
	name := c.Param("name")
	reg := GetGlobalPluginRegistry()

	meta, ok := reg.GetMeta(name)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "plugin not found"})
	}

	_, loaded := reg.Get(name)
	status := "registered"
	if loaded {
		status = "loaded"
	}

	stats, _ := reg.GetStats(name)

	response := map[string]interface{}{
		"name":        name,
		"version":     meta.Version,
		"description": meta.Description,
		"author":      meta.Author,
		"entrypoint":  meta.Entrypoint,
		"type":        entrypointType(meta.Entrypoint),
		"depends_on":  meta.DependsOn,
		"limits": map[string]interface{}{
			"max_timeout_ms":   meta.Limits.MaxTimeoutMs,
			"max_memory_bytes": meta.Limits.MaxMemoryBytes,
		},
		"status": status,
	}

	if stats != nil {
		response["load_time_ms"] = stats.LoadTimeMs
		response["embedded_file_size"] = stats.EmbeddedFileSize
		response["execute_count"] = stats.ExecuteCount
		if stats.LastExecuteMs > 0 {
			response["last_execution_ms"] = stats.LastExecuteMs
		}
		if stats.TotalExecuteMs > 0 {
			response["total_execution_ms"] = stats.TotalExecuteMs
		}
	}

	return c.JSON(http.StatusOK, response)
}

type executeRequest struct {
	Args map[string]interface{} `json:"args"`
}

func handleExecute(c echo.Context) error {
	name := c.Param("name")
	reg := GetGlobalPluginRegistry()

	p, ok := reg.Get(name)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "plugin not loaded"})
	}

	var req executeRequest
	if err := c.Bind(&req); err != nil {
		req.Args = make(map[string]interface{})
	}
	if req.Args == nil {
		req.Args = make(map[string]interface{})
	}

	meta, _ := reg.GetMeta(name)
	ctx := Context{
		ID:       uuid.New().String(),
		Logger:   globalLogger,
		Registry: globalInfraRegistry,
		Limits:   meta.Limits,
	}

	reg.AcquireExecution()
	start := time.Now()
	result, err := p.Execute(ctx, req.Args)
	elapsed := time.Since(start).Seconds() * 1000
	reg.ReleaseExecution()

	if err == nil {
		reg.IncrementExecuteCount(name, elapsed)
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, result)
}

type uploadScriptRequest struct {
	Content string `json:"content"`
}

func handleUploadScript(c echo.Context) error {
	name := c.Param("name")
	fileName := c.Param("file")

	reg := GetGlobalPluginRegistry()
	fsys, ok := reg.GetFilesystem(name)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "plugin not found"})
	}

	var req uploadScriptRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": "content field required"})
	}

	scriptPath := "scripts/" + fileName
	if err := afero.WriteFile(fsys, scriptPath, []byte(req.Content), 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "failed to write script: " + err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"message": "script uploaded", "path": scriptPath})
}

func handleListScripts(c echo.Context) error {
	name := c.Param("name")
	reg := GetGlobalPluginRegistry()
	fsys, ok := reg.GetFilesystem(name)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "plugin not found"})
	}

	entries, err := afero.ReadDir(fsys, "scripts")
	if err != nil {
		return c.JSON(http.StatusOK, map[string]interface{}{"scripts": []string{}})
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if isCompiledArtifact(name) {
			continue
		}
		files = append(files, name)
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"scripts": files})
}

func handleGetScript(c echo.Context) error {
	name := c.Param("name")
	fileName := c.Param("file")

	reg := GetGlobalPluginRegistry()
	fsys, ok := reg.GetFilesystem(name)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "plugin not found"})
	}

	scriptPath := "scripts/" + fileName
	content, err := afero.ReadFile(fsys, scriptPath)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "script not found"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"name":    fileName,
		"content": string(content),
	})
}

func isCompiledArtifact(name string) bool {
	switch {
	case len(name) > 5 && name[len(name)-5:] == ".luac":
		return true
	case len(name) > 4 && name[len(name)-4:] == ".pyc":
		return true
	case len(name) > 7 && name[len(name)-7:] == ".min.js":
		return true
	case len(name) > 5 && name[len(name)-5:] == ".wasm":
		return true
	}
	return false
}

func handleUnload(c echo.Context) error {
	name := c.Param("name")
	reg := GetGlobalPluginRegistry()

	p, ok := reg.Get(name)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]interface{}{"error": "plugin not loaded"})
	}

	if err := p.Close(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": "failed to close plugin: " + err.Error()})
	}

	reg.Remove(name)
	return c.JSON(http.StatusOK, map[string]interface{}{"message": "plugin unloaded", "name": name})
}
