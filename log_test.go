package log

import (
	"os"
	"testing"
)

func TestGetMaxIncrement(t *testing.T) {
	logPath := "./test.log"
	archPath := logPath + ".1"

	err := os.WriteFile(archPath, []byte(""), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = os.Remove(archPath)
		if err != nil {
			t.Error(err)
		}
	}()

	var increment int
	increment, err = getMaxIncrement(logPath)
	if err != nil {
		t.Fatal(err.Error())
	} else if increment != 1 {
		t.Fatal("getMaxIncrement returns incorrect increment")
	}
}
