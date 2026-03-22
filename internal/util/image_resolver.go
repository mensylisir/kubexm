package util

import (
	"errors"
	"fmt"
	"strings"
)

type Resolver struct{}

func NewResolver() *Resolver {
	return &Resolver{}
}

func (r *Resolver) Full(img Image) string {
	return img.ImageName()
}

func (r *Resolver) Parse(fullName string) (Image, error) {
	if strings.TrimSpace(fullName) == "" {
		return Image{}, errors.New("cannot parse an empty image name")
	}

	var img Image
	name := fullName

	lastColon := strings.LastIndex(name, ":")
	lastSlash := strings.LastIndex(name, "/")
	if lastColon > lastSlash {
		img.Tag = name[lastColon+1:]
		name = name[:lastColon]
	}

	parts := strings.Split(name, "/")
	if len(parts) == 0 {
		return Image{}, fmt.Errorf("invalid image name format: %q", fullName)
	}

	img.Repo = parts[len(parts)-1]
	path := parts[:len(parts)-1]

	if len(path) > 0 {
		if strings.Contains(path[0], ".") || strings.Contains(path[0], ":") {
			img.RepoAddr = path[0]
			if len(path) > 1 {
				img.Namespace = strings.Join(path[1:], "/")
			}
		} else {
			img.Namespace = strings.Join(path, "/")
		}
	}

	img.Enable = true

	return img, nil
}
