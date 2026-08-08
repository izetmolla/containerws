package configapi

import (
	"fmt"
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/packages/kubeclient"
)

type createFileBody struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type updateFileBody struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type activateBody struct {
	Context string `json:"context"`
}

func (cc *controller) ListFilesAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	list, activeID, err := ensureSeededRegistry(cc.app)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"files":     toRows(list, activeID),
			"active_id": activeID,
		},
	}))
}

func (cc *controller) GetFileAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	list, activeID, err := ensureSeededRegistry(cc.app)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	entry, ok := findFile(list, id)
	if !ok {
		return r.Api(c, r.WithError(fmt.Errorf("kubeconfig %q not found", id)), r.WithStatus(fiber.StatusNotFound))
	}
	content, err := readFileContent(entry)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	_ = materializeFile(entry)
	contexts, current, _ := listContextsForEntry(entry, content)
	row := toRows([]fileMeta{entry}, activeID)[0]
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"file":     row,
			"content":  content,
			"contexts": contexts,
			"current":  current,
		},
	}))
}

func (cc *controller) CreateFileAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	var body createFileBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(fmt.Errorf("invalid body")), r.WithStatus(fiber.StatusBadRequest))
	}
	entry, err := createManagedFile(cc.app.DB(), body.Name, body.Content)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	list, activeID, _ := ensureSeededRegistry(cc.app)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"file":      toRows([]fileMeta{entry}, activeID)[0],
			"files":     toRows(list, activeID),
			"active_id": activeID,
		},
		"message": "Kubeconfig added",
	}))
}

func (cc *controller) UpdateFileAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	var body updateFileBody
	if err := c.Bind().Body(&body); err != nil {
		return r.Api(c, r.WithError(fmt.Errorf("invalid body")), r.WithStatus(fiber.StatusBadRequest))
	}
	if strings.TrimSpace(body.Name) == "" && body.Content == "" {
		return r.Api(c, r.WithError(fmt.Errorf("name or content is required")), r.WithStatus(fiber.StatusBadRequest))
	}
	entry, err := updateManagedContent(cc.app.DB(), id, body.Name, body.Content)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	list, activeID, _ := ensureSeededRegistry(cc.app)
	contexts, current, _ := listContextsForEntry(entry, body.Content)
	content, _ := readFileContent(entry)
	if content == "" {
		content = body.Content
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"file":     toRows([]fileMeta{entry}, activeID)[0],
			"content":  content,
			"contexts": contexts,
			"current":  current,
			"files":    toRows(list, activeID),
		},
		"message": "Kubeconfig saved",
	}))
}

func (cc *controller) DeleteFileAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	if err := deleteFile(cc.app.DB(), id); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
	}
	list, activeID, _ := ensureSeededRegistry(cc.app)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"files":     toRows(list, activeID),
			"active_id": activeID,
		},
		"message": "Kubeconfig removed",
	}))
}

func (cc *controller) ActivateFileAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	id := strings.TrimSpace(c.Params("id"))
	var body activateBody
	_ = c.Bind().Body(&body)

	list, _, err := ensureSeededRegistry(cc.app)
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	entry, ok := findFile(list, id)
	if !ok {
		return r.Api(c, r.WithError(fmt.Errorf("kubeconfig %q not found", id)), r.WithStatus(fiber.StatusNotFound))
	}
	if !kubeclient.FileExists(entry.Path) {
		if err := materializeFile(entry); err != nil {
			return r.Api(c, r.WithError(fmt.Errorf("kubeconfig file missing at %s", entry.Path)), r.WithStatus(fiber.StatusBadRequest))
		}
	}

	ctxName := strings.TrimSpace(body.Context)
	if ctxName != "" {
		content, _ := readFileContent(entry)
		contexts, _, err := listContextsForEntry(entry, content)
		if err != nil {
			return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusBadRequest))
		}
		found := false
		for _, cx := range contexts {
			if cx.Name == ctxName {
				found = true
				break
			}
		}
		if !found {
			return r.Api(c, r.WithError(fmt.Errorf("context %q not found in kubeconfig", ctxName)), r.WithStatus(fiber.StatusBadRequest))
		}
	}

	if err := activateFile(cc.app.DB(), entry, ctxName); err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	list, activeID, _ := ensureSeededRegistry(cc.app)
	content, _ := readFileContent(entry)
	contexts, current, _ := listContextsForEntry(entry, content)
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"files":     toRows(list, activeID),
			"active_id": activeID,
			"path":      entry.Path,
			"context":   current,
			"contexts":  contexts,
		},
		"message": "Active kubeconfig updated",
	}))
}

func listContextsForEntry(entry fileMeta, content string) ([]kubeclient.ContextInfo, string, error) {
	if kubeclient.FileExists(entry.Path) {
		return kubeclient.ListContexts(entry.Path, "")
	}
	if strings.TrimSpace(content) == "" {
		content = entry.Secret
	}
	if strings.TrimSpace(content) == "" {
		return nil, "", fmt.Errorf("kubeconfig content is empty")
	}
	tmp, err := os.CreateTemp("", "cws-kubeconfig-*.yaml")
	if err != nil {
		return nil, "", err
	}
	path := tmp.Name()
	_, _ = tmp.WriteString(content)
	_ = tmp.Close()
	defer os.Remove(path)
	return kubeclient.ListContexts(path, "")
}
