package main

import (
	"os"

	"github.com/88250/gulu"
)

var logger = gulu.Log.NewLogger(os.Stdout)

func main() {
	logger.Infof("bazaar is staging...")

}
