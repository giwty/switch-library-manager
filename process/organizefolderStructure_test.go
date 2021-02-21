package process

import (
	"robpike.io/nihongo"
	"strings"
	"testing"
)

//var folderIllegalCharsRegex = regexp.MustCompile(`[./\\?%*:;=|"<>]`)

func TestRename(t *testing.T) {
	name := "Pokémon™: Let’s Go, Eevee! 포탈 나이츠"
	name = folderIllegalCharsRegex.ReplaceAllString(name, "")
	safe := cjk.FindAllString(name, -1)
	name = strings.Join(safe, "")
	name = nihongo.RomajiString(name)
}
