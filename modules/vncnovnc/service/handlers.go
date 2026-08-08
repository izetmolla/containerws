package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

func (cc *controller) StatusAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	status := CollectStatus(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": status,
	}))
}

func (cc *controller) StartAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	result, err := StartAll(cc.app.DB())
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	status := CollectStatus(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"action": result,
			"status": status,
		},
		"message": fmt.Sprintf("VNC service started (%d session(s))", result.Started),
	}))
}

func (cc *controller) StopAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	result, err := StopAll(cc.app.DB())
	if err != nil {
		return r.Api(c, r.WithError(err), r.WithStatus(fiber.StatusInternalServerError))
	}
	status := CollectStatus(cc.app.DB())
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"action": result,
			"status": status,
		},
		"message": fmt.Sprintf("VNC service stopped (%d session(s))", result.Stopped),
	}))
}

func (cc *controller) LogsAPI(c fiber.Ctx) error {
	r := cc.app.Render()
	paths := collectLogSources(cc.app.DB())
	var lines []string
	for _, path := range paths {
		chunk, err := tailFile(path, 200)
		if err != nil {
			continue
		}
		if len(chunk) == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("----- %s -----", path))
		lines = append(lines, chunk...)
	}
	return r.Api(c, r.WithStatus(fiber.StatusOK), r.WithData(fiber.Map{
		"data": fiber.Map{
			"paths": paths,
			"lines": lines,
		},
	}))
}

type logStreamEvent struct {
	Type    string `json:"type"`
	Path    string `json:"path,omitempty"`
	Line    string `json:"line,omitempty"`
	Stream  string `json:"stream,omitempty"`
	Message string `json:"message,omitempty"`
}

func (cc *controller) StreamLogsAPI(c fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	db := cc.app.DB()
	reqCtx := c.Context()

	return c.SendStreamWriter(func(w *bufio.Writer) {
		writeEvent := func(ev logStreamEvent) bool {
			payload, err := json.Marshal(ev)
			if err != nil {
				return false
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return false
			}
			return w.Flush() == nil
		}

		if !writeEvent(logStreamEvent{
			Type:    "start",
			Message: "Streaming VNC service logs",
			Stream:  "system",
		}) {
			return
		}

		offsets := map[string]int64{}
		paths := collectLogSources(db)
		for _, path := range paths {
			chunk, size, err := readNewLines(path, 0, 400)
			if err != nil {
				continue
			}
			offsets[path] = size
			for _, line := range chunk {
				if !writeEvent(logStreamEvent{
					Type:   "log",
					Path:   filepath.Base(path),
					Line:   line,
					Stream: "stdout",
				}) {
					return
				}
			}
		}

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-reqCtx.Done():
				_ = writeEvent(logStreamEvent{
					Type:    "done",
					Message: "log stream closed",
					Stream:  "system",
				})
				return
			case <-ticker.C:
				paths = collectLogSources(db)
				for _, path := range paths {
					off := offsets[path]
					chunk, size, err := readNewLines(path, off, 200)
					if err != nil {
						continue
					}
					offsets[path] = size
					for _, line := range chunk {
						if !writeEvent(logStreamEvent{
							Type:   "log",
							Path:   filepath.Base(path),
							Line:   line,
							Stream: "stdout",
						}) {
							return
						}
					}
				}
			}
		}
	})
}

func tailFile(path string, maxLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []string
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		all = append(all, sc.Text())
	}
	if err := sc.Err(); err != nil {
		return all, err
	}
	if maxLines > 0 && len(all) > maxLines {
		all = all[len(all)-maxLines:]
	}
	return all, nil
}

func readNewLines(path string, offset int64, maxLines int) ([]string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, offset, err
	}
	size := info.Size()
	if offset > size {
		offset = 0
	}
	if offset == 0 && size > 0 {
		lines, err := tailFile(path, maxLines)
		return lines, size, err
	}
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, offset, err
		}
	}

	var lines []string
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	if err := sc.Err(); err != nil {
		return lines, size, err
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, size, nil
}
