package process

import (
	"robpike.io/nihongo"
	"testing"
)

//var folderIllegalCharsRegex = regexp.MustCompile(`[./\\?%*:;=|"<>]`)

func TestRename(t *testing.T) {
	name := "鉄道にっぽん！路線たび 叡山電車編"
	name = folderIllegalCharsRegex.ReplaceAllString(name, "")
	name = nihongo.RomajiString(name)
}
