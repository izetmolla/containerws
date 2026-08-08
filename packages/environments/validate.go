package environments

import (
	"errors"
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func normalizeName(raw string) string {
	name := strings.ToUpper(strings.TrimSpace(raw))
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}

func formatName(raw string) (string, error) {
	name := normalizeName(raw)
	if name == "" {
		return "", errors.New("name is required")
	}
	if !namePattern.MatchString(name) {
		return "", errors.New("name must start with a letter and contain only letters, numbers, and underscores")
	}
	return name, nil
}

func formatGroup(raw string) string {
	return strings.TrimSpace(raw)
}

func validateCreateName(name string) error {
	if IsCoreName(name) {
		return ErrCoreNameReserved
	}
	return nil
}

func validateUpdateName(current OsEnvironment, newName string) error {
	currentName := normalizeName(current.Name)
	if IsCoreName(currentName) && newName != currentName {
		return errors.New("core server setting names cannot be renamed")
	}
	if !IsCoreName(currentName) && IsCoreName(newName) {
		return ErrCoreNameReserved
	}
	return nil
}

func validateEnvironmentRow(row *OsEnvironment) error {
	name, err := formatName(row.Name)
	if err != nil {
		return err
	}
	row.Name = name
	if row.Source == "" {
		if IsCoreName(name) {
			row.Source = OsEnvironmentSourceServer
			row.IsCore = true
		} else {
			row.Source = OsEnvironmentSourceEnv
		}
	}
	if IsCoreName(name) {
		row.IsCore = true
		row.Source = OsEnvironmentSourceServer
	}
	return nil
}
