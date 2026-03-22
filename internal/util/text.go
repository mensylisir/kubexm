package util

import "github.com/common-nighthawk/go-figure"

func GenerateASCIIArt(text string, font string) string {
	myFigure := figure.NewFigure(text, font, true)
	return myFigure.String()
}
