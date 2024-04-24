//go:generate go run ./gen
package conf

import (
	"embed"
	"io/fs"
	"os"

	"github.com/l4go/buildfs"
)

//go:embed etc lib www
var defaultConfFS embed.FS
var DefaultConfFS fs.FS = fs.FS(buildfs.BuildInFS(defaultConfFS, BuildTime))

func StartDir() (string, error) {
	return os.UserHomeDir()
}
