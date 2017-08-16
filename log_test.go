package log

import (
	"testing"
	"io/ioutil"
	"os"
)

func TestGetMaxIncrement(t *testing.T) {
	logPath := "./test.log"
	archPath := logPath + ".1"

	ioutil.WriteFile(archPath, []byte(""), os.ModePerm)
	defer os.Remove(archPath)

	increment, err := getMaxIncrement(logPath)
	if err != nil {
		t.Fatal(err.Error())
	} else if increment != 1 {
		t.Fatal("getMaxIncrement returns incorrect increment")
	}
}
