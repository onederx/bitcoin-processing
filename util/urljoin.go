package util

import (
	"net/url"
)

func URLJoin(base, relative string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	relativeURL, err := url.Parse(relative)
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(relativeURL).String(), nil
}

func MustURLJoin(base, relative string) string {
	result, err := URLJoin(base, relative)
	if err != nil {
		panic(err)
	}
	return result
}
