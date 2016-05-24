package main

type cmsPublicationEvent struct {
	ContentURI   string
	UUID         string
	Payload      interface{}
	LastModified string
}
