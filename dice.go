package main

import (
	"fmt"
	"regexp"

	"github.com/justinian/dice"
)

var multiDicePattern = regexp.MustCompile(fmt.Sprintf("([0-9]+) %s", dice.StdRoller{}.Pattern().String()))
