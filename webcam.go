package verhboat

import (
	"context"
	"embed"
	"io/fs"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"

	"github.com/erh/vmodutils"
)

//go:embed web-cam/dist
var staticFS embed.FS

func DistFS() (fs.FS, error) {
	return fs.Sub(staticFS, "web-cam/dist")
}

var WebCamModel = NamespaceFamily.WithModel("www-cams")

func init() {
	resource.RegisterComponent(
		generic.API,
		WebCamModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: newServer,
		})
}

func newServer(ctx context.Context, deps resource.Dependencies, config resource.Config, logger logging.Logger) (resource.Resource, error) {
	fs, err := DistFS()
	if err != nil {
		return nil, err
	}

	return vmodutils.NewWebModuleAndStart(config.ResourceName(), fs, logger, config.Attributes.Int("port", 8888))
}
