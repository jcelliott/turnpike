package turnpike

import (
	"errors"
	"fmt"
	glog "log"
	"os"
)

var (
	logFlags = glog.Ldate | glog.Ltime | glog.Lshortfile
	log      *glog.Logger
)

// setup logger for package, writes to /dev/null by default
func init() {
	if devNull, err := os.Create(os.DevNull); err != nil {
		panic("could not create logger: " + err.Error())
	} else {
		log = glog.New(devNull, "", 0)
	}
}

// change log output to stderr
func Debug() {
	log = glog.New(os.Stderr, "", logFlags)
}

func logErr(v ...interface{}) error {
	err := errors.New(fmt.Sprintln(v...))
	log.Println(err)
	return err
}
