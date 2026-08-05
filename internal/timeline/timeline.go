// Package timeline is cutvec's in-memory model of an MLT XML timeline — the
// declarative, diffable edit artifact an agent mutates and validates, rendered
// to video by `melt`. The Go types marshal to and from the <mlt> document so an
// agent edits a structured object, never an opaque ffmpeg command string.
package timeline

import "encoding/xml"

// Timeline is the root <mlt> element of an MLT document.
type Timeline struct {
	XMLName   xml.Name   `xml:"mlt"`
	Version   string     `xml:"version,attr,omitempty"`
	Producers []Producer `xml:"producer"`
}

// Producer is an MLT <producer>: a media source placed on the timeline.
type Producer struct {
	ID       string `xml:"id,attr"`
	Resource string `xml:"resource,attr,omitempty"`
}

// New returns an empty timeline stamped with the MLT document version.
func New() *Timeline {
	return &Timeline{Version: "7.0"}
}

// Marshal renders the timeline to MLT XML bytes.
func (t *Timeline) Marshal() ([]byte, error) {
	return xml.MarshalIndent(t, "", "  ")
}
