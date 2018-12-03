package main

import (
	"log"
	"net/url"
)

func urljoin(base, relative string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		log.Fatal(err)
	}
	relativeURL, err := url.Parse(relative)
	if err != nil {
		log.Fatal(err)
	}
	return baseURL.ResolveReference(relativeURL).String()
}
