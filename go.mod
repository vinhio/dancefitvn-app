module dancefitvn-app

go 1.27.0

require (
	github.com/gdeskdev/desk/bridge/backend/android v1.0.0
	github.com/gdeskdev/desk/bridge/gpu v1.0.0
	github.com/gdeskdev/desk/bridge/system v1.0.0
	github.com/gdeskdev/desk/bridge/text v1.0.0
	github.com/gdeskdev/desk/bridge/text/auto v1.0.0
	github.com/gdeskdev/desk/bridge/text/coretext v1.0.0
	github.com/gdeskdev/desk/bridge/text/gotext v1.0.0
	github.com/gdeskdev/desk/bridge/text/typeset v1.0.0
	github.com/gdeskdev/desk/foundation/core v1.0.0
	github.com/gdeskdev/desk/foundation/ffi v1.0.0
	github.com/gdeskdev/desk/foundation/platform v1.0.0
	github.com/gdeskdev/desk/foundation/raster v1.0.0
	github.com/gdeskdev/desk/foundation/scene v1.0.0
	github.com/gdeskdev/desk/framework/components v1.0.0
	github.com/gdeskdev/desk/framework/host v1.0.0
	github.com/gdeskdev/desk/framework/ui v1.0.0
	github.com/gdeskdev/desk/kits/mobile v1.0.0
	github.com/stretchr/testify v1.12.1
)

require (
	github.com/ebitengine/purego v0.11.2 // indirect
	github.com/gdeskdev/desk/bridge/backend/uikit v1.0.0 // indirect
	github.com/gdeskdev/desk/bridge/gpu/gl v1.0.0 // indirect
	github.com/gdeskdev/desk/bridge/gpu/metal v1.0.0 // indirect
	github.com/gdeskdev/desk/bridge/text/directwrite v1.0.0 // indirect
	github.com/gdeskdev/desk/bridge/text/pango v1.0.0 // indirect
	github.com/gdeskdev/desk/foundation/accelerator v1.0.0 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/image v0.47.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

// gDesk, from a checkout.
replace (
	github.com/gdeskdev/desk/bridge/backend/android v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/backend/android
	github.com/gdeskdev/desk/bridge/backend/darwin v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/backend/darwin
	github.com/gdeskdev/desk/bridge/backend/gtk v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/backend/gtk
	github.com/gdeskdev/desk/bridge/backend/uikit v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/backend/uikit
	github.com/gdeskdev/desk/bridge/backend/web v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/backend/web
	github.com/gdeskdev/desk/bridge/backend/win32 v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/backend/win32
	github.com/gdeskdev/desk/bridge/gpu v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/gpu
	github.com/gdeskdev/desk/bridge/gpu/d3d11 v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/gpu/d3d11
	github.com/gdeskdev/desk/bridge/gpu/gl v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/gpu/gl
	github.com/gdeskdev/desk/bridge/gpu/metal v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/gpu/metal
	github.com/gdeskdev/desk/bridge/gpu/webgl v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/gpu/webgl
	github.com/gdeskdev/desk/bridge/model v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/model
	github.com/gdeskdev/desk/bridge/system v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/system
	github.com/gdeskdev/desk/bridge/text v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/text
	github.com/gdeskdev/desk/bridge/text/auto v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/text/auto
	github.com/gdeskdev/desk/bridge/text/chroma v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/text/chroma
	github.com/gdeskdev/desk/bridge/text/coretext v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/text/coretext
	github.com/gdeskdev/desk/bridge/text/directwrite v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/text/directwrite
	github.com/gdeskdev/desk/bridge/text/gotext v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/text/gotext
	github.com/gdeskdev/desk/bridge/text/pango v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/text/pango
	github.com/gdeskdev/desk/bridge/text/typeset v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/text/typeset
	github.com/gdeskdev/desk/bridge/update v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/bridge/update
	github.com/gdeskdev/desk/foundation/accelerator v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/foundation/accelerator
	github.com/gdeskdev/desk/foundation/core v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/foundation/core
	github.com/gdeskdev/desk/foundation/ffi v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/foundation/ffi
	github.com/gdeskdev/desk/foundation/i18n v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/foundation/i18n
	github.com/gdeskdev/desk/foundation/icon v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/foundation/icon
	github.com/gdeskdev/desk/foundation/platform v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/foundation/platform
	github.com/gdeskdev/desk/foundation/raster v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/foundation/raster
	github.com/gdeskdev/desk/foundation/scene v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/foundation/scene
	github.com/gdeskdev/desk/framework/components v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/framework/components
	github.com/gdeskdev/desk/framework/host v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/framework/host
	github.com/gdeskdev/desk/framework/lsp v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/framework/lsp
	github.com/gdeskdev/desk/framework/ui v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/framework/ui
	github.com/gdeskdev/desk/kits/desktop v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/kits/desktop
	github.com/gdeskdev/desk/kits/e2e v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/kits/e2e
	github.com/gdeskdev/desk/kits/mobile v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/kits/mobile
	github.com/gdeskdev/desk/kits/web v1.0.0 => /Users/vinh/Working/gdesk-dancefitvn-app/kits/web
)
