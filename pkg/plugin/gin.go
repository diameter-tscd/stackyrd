package plugin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/afero"
)

func RegisterManagementRoutes(rg *gin.RouterGroup) {
	rg.GET("", handleList)
	rg.GET("/:name", handleGet)
	rg.POST("/:name/execute", handleExecute)
	rg.PUT("/:name/scripts/:file", handleUploadScript)
	rg.GET("/:name/scripts", handleListScripts)
	rg.GET("/:name/scripts/:file", handleGetScript)
	rg.DELETE("/:name", handleUnload)
}

func handleList(c *gin.Context) {
	reg := GetGlobalPluginRegistry()
	metas := reg.GetAllMetas()
	result := make([]gin.H, 0, len(metas))
	for name, meta := range metas {
		_, loaded := reg.Get(name)
		status := "registered"
		if loaded {
			status = "loaded"
		}
		result = append(result, gin.H{
			"name":        name,
			"version":     meta.Version,
			"description": meta.Description,
			"status":      status,
		})
	}
	c.JSON(http.StatusOK, gin.H{"plugins": result})
}

func handleGet(c *gin.Context) {
	name := c.Param("name")
	reg := GetGlobalPluginRegistry()

	meta, ok := reg.GetMeta(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}

	_, loaded := reg.Get(name)
	status := "registered"
	if loaded {
		status = "loaded"
	}

	typeInfo := "go"
	if len(meta.Entrypoint) > 2 && meta.Entrypoint[:3] == "ts:" {
		typeInfo = "typescript"
	}

	c.JSON(http.StatusOK, gin.H{
		"name":        name,
		"version":     meta.Version,
		"description": meta.Description,
		"author":      meta.Author,
		"entrypoint":  meta.Entrypoint,
		"type":        typeInfo,
		"depends_on":  meta.DependsOn,
		"limits": gin.H{
			"max_timeout_ms":   meta.Limits.MaxTimeoutMs,
			"max_memory_bytes": meta.Limits.MaxMemoryBytes,
		},
		"status": status,
	})
}

type executeRequest struct {
	Args map[string]interface{} `json:"args"`
}

func handleExecute(c *gin.Context) {
	name := c.Param("name")
	reg := GetGlobalPluginRegistry()

	p, ok := reg.Get(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not loaded"})
		return
	}

	var req executeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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

	result, err := p.Execute(ctx, req.Args)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

type uploadScriptRequest struct {
	Content string `json:"content" binding:"required"`
}

func handleUploadScript(c *gin.Context) {
	name := c.Param("name")
	fileName := c.Param("file")

	reg := GetGlobalPluginRegistry()
	fsys, ok := reg.GetFilesystem(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}

	var req uploadScriptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content field required"})
		return
	}

	scriptPath := "scripts/" + fileName
	if err := afero.WriteFile(fsys, scriptPath, []byte(req.Content), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write script: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "script uploaded", "path": scriptPath})
}

func handleListScripts(c *gin.Context) {
	name := c.Param("name")
	reg := GetGlobalPluginRegistry()
	fsys, ok := reg.GetFilesystem(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}

	entries, err := afero.ReadDir(fsys, "scripts")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"scripts": []string{}})
		return
	}

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	c.JSON(http.StatusOK, gin.H{"scripts": files})
}

func handleGetScript(c *gin.Context) {
	name := c.Param("name")
	fileName := c.Param("file")

	reg := GetGlobalPluginRegistry()
	fsys, ok := reg.GetFilesystem(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not found"})
		return
	}

	scriptPath := "scripts/" + fileName
	content, err := afero.ReadFile(fsys, scriptPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "script not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"name":    fileName,
		"content": string(content),
	})
}

func handleUnload(c *gin.Context) {
	name := c.Param("name")
	reg := GetGlobalPluginRegistry()

	p, ok := reg.Get(name)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "plugin not loaded"})
		return
	}

	if err := p.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to close plugin: " + err.Error()})
		return
	}

	reg.Remove(name)
	c.JSON(http.StatusOK, gin.H{"message": "plugin unloaded", "name": name})
}
