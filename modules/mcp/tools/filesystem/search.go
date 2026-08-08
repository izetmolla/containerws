package filesystem

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SearchFilesInput struct {
	Root           string   `json:"root,omitempty" jsonschema:"root directory to search (default .)"`
	NameGlob       string   `json:"name_glob,omitempty" jsonschema:"optional filename glob e.g. *.go or **/routes.tsx"`
	ContentPattern string   `json:"content_pattern,omitempty" jsonschema:"optional regex matched against file contents"`
	IgnoreDirs     []string `json:"ignore_dirs,omitempty" jsonschema:"directory names to skip (default node_modules,.git,vendor,dist)"`
	MaxResults     int      `json:"max_results,omitempty" jsonschema:"max matches to return (default 50)"`
	MaxFileBytes   int      `json:"max_file_bytes,omitempty" jsonschema:"skip content search for files larger than this (default 1048576)"`
}

type SearchMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Snippet string `json:"snippet,omitempty"`
	Reason  string `json:"reason"`
}

type SearchFilesOutput struct {
	Root      string        `json:"root"`
	Matches   []SearchMatch `json:"matches"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated,omitempty"`
}

func (c *Controller) SearchFilesTool(ctx context.Context, _ *mcp.CallToolRequest, input SearchFilesInput) (*mcp.CallToolResult, any, error) {
	_ = ctx
	root := strings.TrimSpace(input.Root)
	if root == "" {
		root = "."
	}
	abs, err := resolvePath(root)
	if err != nil {
		return nil, nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("root is not a directory: %s", abs)
	}

	nameGlob := strings.TrimSpace(input.NameGlob)
	contentPat := strings.TrimSpace(input.ContentPattern)
	if nameGlob == "" && contentPat == "" {
		return nil, nil, fmt.Errorf("provide name_glob and/or content_pattern")
	}

	var re *regexp.Regexp
	if contentPat != "" {
		re, err = regexp.Compile(contentPat)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid content_pattern: %w", err)
		}
	}

	ignore := map[string]struct{}{
		"node_modules": {},
		".git":         {},
		"vendor":       {},
		"dist":         {},
		".cache":       {},
	}
	for _, d := range input.IgnoreDirs {
		d = strings.TrimSpace(d)
		if d != "" {
			ignore[d] = struct{}{}
		}
	}

	limit := input.MaxResults
	if limit <= 0 {
		limit = 50
	}
	maxBytes := input.MaxFileBytes
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024
	}

	out := SearchFilesOutput{Root: abs, Matches: []SearchMatch{}}

	err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if _, skip := ignore[d.Name()]; skip && p != abs {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out.Matches) >= limit {
			out.Truncated = true
			return fs.SkipAll
		}

		nameMatched := nameGlob == ""
		if nameGlob != "" {
			ok, _ := filepath.Match(filepath.Base(nameGlob), d.Name())
			if strings.Contains(nameGlob, "/") || strings.Contains(nameGlob, "**") {
				rel, _ := filepath.Rel(abs, p)
				ok2, _ := pathMatch(nameGlob, rel)
				ok = ok || ok2
			}
			nameMatched = ok
			if !nameMatched && re == nil {
				return nil
			}
		}

		if re == nil {
			if nameMatched {
				out.Matches = append(out.Matches, SearchMatch{Path: p, Reason: "name"})
			}
			return nil
		}

		fi, err := d.Info()
		if err != nil || fi.Size() > int64(maxBytes) {
			if nameMatched {
				out.Matches = append(out.Matches, SearchMatch{Path: p, Reason: "name"})
			}
			return nil
		}

		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, maxBytes)
		lineNo := 0
		foundContent := false
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if re.MatchString(line) {
				snippet := line
				if len(snippet) > 240 {
					snippet = snippet[:240] + "…"
				}
				out.Matches = append(out.Matches, SearchMatch{
					Path:    p,
					Line:    lineNo,
					Snippet: snippet,
					Reason:  "content",
				})
				foundContent = true
				if len(out.Matches) >= limit {
					out.Truncated = true
					return fs.SkipAll
				}
			}
		}
		if !foundContent && nameMatched {
			out.Matches = append(out.Matches, SearchMatch{Path: p, Reason: "name"})
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	out.Total = len(out.Matches)
	return nil, out, nil
}

// pathMatch supports simple ** segments by converting to filepath.Match against cleaned paths.
func pathMatch(pattern, rel string) (bool, error) {
	pattern = filepath.ToSlash(pattern)
	rel = filepath.ToSlash(rel)
	if strings.Contains(pattern, "**") {
		// Convert ** to * for a best-effort match on full relative path.
		simplified := strings.ReplaceAll(pattern, "**", "*")
		return filepath.Match(simplified, rel)
	}
	return filepath.Match(pattern, rel)
}
