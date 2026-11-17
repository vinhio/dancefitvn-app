// Command dancefitvn-app is the DanceFITVN app for iPhone, Android and
// Apple TV, built on gDesk. The shell (mobile or TV) is chosen by build
// tag inside internal/app.
package main

import (
	"log"

	_ "github.com/gdeskdev/desk/kits/mobile" // the drivers of iOS and Android

	"dancefitvn-app/internal/app"
)

func main() {
	if err := app.Run(app.Options{Mock: mockAPI()}); err != nil {
		log.Fatal(err)
	}
}
