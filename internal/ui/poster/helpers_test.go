package poster_test

import (
	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/scene"
)

// solid is scene.Solid, shortened for the tests.
func solid(c color.Color) scene.Paint { return scene.Solid(c) }
