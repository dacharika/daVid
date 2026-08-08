// Package timeline is cutvec's in-memory model of an MLT XML timeline — the
// declarative, diffable edit artifact an agent mutates and validates, rendered
// to video by `melt`. The Go types marshal to and from the <mlt> document so an
// agent edits a structured object, never an opaque ffmpeg command string.
package timeline

import (
	"encoding/xml"
	"fmt"
)

// Timeline is the root <mlt> element of an MLT document.
type Timeline struct {
	XMLName   xml.Name   `xml:"mlt"`
	Version   string     `xml:"version,attr,omitempty"`
	Profile   *Profile   `xml:"profile,omitempty"`
	Producers []Producer `xml:"producer"`
}

// Profile is MLT's <profile>: the output geometry every producer is rendered
// into. Setting it is what makes a timeline vertical (1080x1920) rather than
// inheriting the source's shape.
type Profile struct {
	Width            int `xml:"width,attr"`
	Height           int `xml:"height,attr"`
	DisplayAspectNum int `xml:"display_aspect_num,attr"`
	DisplayAspectDen int `xml:"display_aspect_den,attr"`
	FrameRateNum     int `xml:"frame_rate_num,attr"`
	FrameRateDen     int `xml:"frame_rate_den,attr"`
	Progressive      int `xml:"progressive,attr"`
	SampleAspectNum  int `xml:"sample_aspect_num,attr"`
	SampleAspectDen  int `xml:"sample_aspect_den,attr"`
}

// Producer is an MLT <producer>: a media source placed on the timeline. In and
// Out trim it to a span (MLT clock format, "HH:MM:SS.mmm"); Filters are applied
// to it in order.
type Producer struct {
	ID         string     `xml:"id,attr"`
	Resource   string     `xml:"resource,attr,omitempty"`
	In         string     `xml:"in,attr,omitempty"`
	Out        string     `xml:"out,attr,omitempty"`
	Properties []Property `xml:"property"`
	Filters    []Filter   `xml:"filter"`
}

// Property is MLT's <property name="...">value</property>.
type Property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:",chardata"`
}

// Filter is an MLT <filter>. The mlt_service property selects the filter (e.g.
// "qtcrop" to reframe, "subtitle" to burn captions); the rest configure it.
type Filter struct {
	ID         string     `xml:"id,attr,omitempty"`
	Properties []Property `xml:"property"`
}

// New returns an empty timeline stamped with the MLT document version.
func New() *Timeline {
	return &Timeline{Version: "7.0"}
}

// Vertical returns the 9:16 profile short-form video is delivered in.
func Vertical(fps int) *Profile {
	if fps <= 0 {
		fps = 30
	}
	return &Profile{
		Width: 1080, Height: 1920,
		DisplayAspectNum: 9, DisplayAspectDen: 16,
		FrameRateNum: fps, FrameRateDen: 1,
		Progressive:     1,
		SampleAspectNum: 1, SampleAspectDen: 1,
	}
}

// Clock formats seconds as the "HH:MM:SS.mmm" MLT uses for in/out points.
func Clock(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := seconds - float64(h*3600+m*60)
	return fmt.Sprintf("%02d:%02d:%06.3f", h, m, s)
}

// NewFilter builds a filter for the given mlt_service with ordered properties
// supplied as name/value pairs.
func NewFilter(service string, kv ...string) Filter {
	f := Filter{Properties: []Property{{Name: "mlt_service", Value: service}}}
	for i := 0; i+1 < len(kv); i += 2 {
		f.Properties = append(f.Properties, Property{Name: kv[i], Value: kv[i+1]})
	}
	return f
}

// CropRect builds the qtcrop filter that reframes a landscape source to the
// timeline's profile by keeping the given source-pixel rectangle.
func CropRect(x, y, w, h int) Filter {
	return NewFilter("qtcrop", "rect", fmt.Sprintf("%d %d %d %d", x, y, w, h))
}

// Subtitles builds the filter that burns an SRT file over the video. geometry
// positions the text within the frame as MLT's "x y w h" rect.
func Subtitles(srtPath, geometry, family, size, colour string) Filter {
	return NewFilter("subtitle",
		"resource", srtPath,
		"geometry", geometry,
		"family", family,
		"size", size,
		"weight", "900",
		"fgcolour", colour,
		"olcolour", "#ff000000",
		"outline", "3",
		"valign", "bottom",
		"halign", "center",
	)
}

// Marshal renders the timeline to MLT XML bytes.
func (t *Timeline) Marshal() ([]byte, error) {
	return xml.MarshalIndent(t, "", "  ")
}

// Validate reports the structural problems that would make melt fail, so an
// agent can fix a timeline before spending a render on it.
func (t *Timeline) Validate() []string {
	var problems []string
	if len(t.Producers) == 0 {
		problems = append(problems, "timeline has no producers: nothing would render")
	}
	seen := map[string]bool{}
	for i, p := range t.Producers {
		if p.ID == "" {
			problems = append(problems, fmt.Sprintf("producer %d has no id", i))
		} else if seen[p.ID] {
			problems = append(problems, fmt.Sprintf("duplicate producer id %q", p.ID))
		}
		seen[p.ID] = true
		if p.Resource == "" && !hasProperty(p.Properties, "resource") {
			problems = append(problems,
				fmt.Sprintf("producer %q has no resource to render", p.ID))
		}
		for j, f := range p.Filters {
			if !hasProperty(f.Properties, "mlt_service") {
				problems = append(problems,
					fmt.Sprintf("filter %d on producer %q has no mlt_service", j, p.ID))
			}
		}
	}
	return problems
}

func hasProperty(props []Property, name string) bool {
	for _, p := range props {
		if p.Name == name && p.Value != "" {
			return true
		}
	}
	return false
}
