// Adapted from jplayer-mobile/internal/sys/present_ios.go at 49de3c2
// (blockType, idType, newBlock, idArg). The build tag ios covers tvOS
// too (GOOS=ios with -tags tvos): the network's path monitor
// (network_ios.go) takes a block on both.

//go:build ios

package sys

import (
	"reflect"

	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// blockType is the type a block's function takes first (purego's
// objc.Block, which gDesk's objc returns from NewBlock but does not name).
var blockType = reflect.TypeOf(objc.NewBlock).Out(0)

// idType is the type of a block's object argument.
var idType = reflect.TypeFor[objc.ID]()

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// newBlock makes an owned Objective-C block of a function taking args
// after the block itself and answering nothing; fn gets those arguments.
// Its C callback is made once per list of argument types (purego caches
// it by the function's type, which reflect.FuncOf makes the same each
// time), so a block made for each request costs no new callback. Release
// it (objc.Release) once the API it is given has its copy.
func newBlock(fn func(args []reflect.Value), args ...reflect.Type) objc.ID {
	in := append([]reflect.Type{blockType}, args...)
	impl := reflect.MakeFunc(reflect.FuncOf(in, nil, false), func(vals []reflect.Value) []reflect.Value {
		fn(vals[1:])

		return nil
	})

	return objc.ID(objc.NewBlock(impl.Interface()))
}

// idArg returns the object of a block's argument.
func idArg(v reflect.Value) objc.ID { return objc.ID(v.Uint()) }
