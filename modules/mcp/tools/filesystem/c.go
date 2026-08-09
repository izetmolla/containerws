package filesystem

import (
	"github.com/izetmolla/containerws/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Controller struct {
	app *config.AppClients
}

func NewController(app *config.AppClients) *Controller {
	return &Controller{app: app}
}

func LoadTools(server *mcp.Server, app *config.AppClients) {
	controller := NewController(app)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "read_file",
		Description: "Read a file from the filesystem. Optional offset/limit (line-based) for large files.",
	}, controller.ReadFileTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "write_file",
		Description: "Create or overwrite a file with the given contents. Creates parent directories when needed.",
	}, controller.WriteFileTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "edit_file",
		Description: "Replace old_string with new_string in a file. Fails if old_string is missing or not unique unless replace_all=true.",
	}, controller.EditFileTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_directory",
		Description: "List entries in a directory. Optional recursive listing with max_depth.",
	}, controller.ListDirectoryTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "make_directory",
		Description: "Create a directory (mkdir -p style). Idempotent if it already exists.",
	}, controller.MakeDirectoryTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_path",
		Description: "Delete a file or directory. For non-empty directories set recursive=true.",
	}, controller.DeletePathTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "move_path",
		Description: "Move or rename a file/directory from source to destination.",
	}, controller.MovePathTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "copy_path",
		Description: "Copy a file or directory from source to destination. Directories require recursive=true.",
	}, controller.CopyPathTool)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "stat_path",
		Description: "Return filesystem metadata for a path (size, mode, mod time, type).",
	}, controller.StatPathTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "search_files",
		Description: "Search for files by name glob and/or content regex under a root path. " +
			"Use for locating code/config before editing.",
	}, controller.SearchFilesTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "zip_paths",
		Description: "Create a .zip archive from one or more files/directories (same behavior as File Manager Zip). " +
			"Optional destination; defaults next to the first path.",
	}, controller.ZipPathsTool)

	mcp.AddTool(server, &mcp.Tool{
		Name: "unzip_path",
		Description: "Extract a .zip archive to a destination directory (defaults to a sibling folder named after the zip).",
	}, controller.UnzipPathTool)
}
