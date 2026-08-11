// Package timeline is cutvec's in-memory model of an MLT XML timeline — the
// declarative, diffable edit artifact an agent mutates and validates, rendered
// to video by `melt`. The Go types marshal to and from the <mlt> document so an
// agent edits a structured object, never an opaque ffmpeg command string.
package timeline

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
)

// Timeline is the root <mlt> element of an MLT document.
type Timeline struct {
	XMLName   xml.Name   `xml:"mlt"`
	Version   string     `xml:"version,attr,omitempty"`
	Profile   *Profile   `xml:"profile,omitempty"`
	Producers []Producer `xml:"producer"`
	Playlists []Playlist `xml:"playlist"`
	Tractor   *Tractor   `xml:"tractor,omitempty"`
}

// Playlist is an MLT <playlist>: producers played in order. A cut sequence — an
// edit that switches between framings — is a playlist of trimmed producers.
//
// A playlist's children are two DIFFERENT MLT elements — <entry producer="..."/>
// for a real producer and <blank length="..."/> for a gap — so Playlist carries
// its own MarshalXML/UnmarshalXML to emit and parse each Entry as the right
// element. A blank must be a real <blank>: MLT treats an <entry length="..."> as
// a malformed producer entry, not the leading silence a delayed sting needs.
type Playlist struct {
	ID      string
	Entries []Entry
}

// Entry places a producer in a playlist, or — when Blank is set — reserves an
// empty span of that length instead. Blank is how MLT delays a producer's entry
// on its track (the seconds of silence before a sting or bed comes in); an entry
// with Blank set marshals to <blank length="..."/>, otherwise to
// <entry producer="..."/>.
type Entry struct {
	Producer string
	Blank    string
}

// MarshalXML writes the playlist and each of its children as the correct MLT
// element: <blank length="..."/> for a gap, <entry producer="..."/> for a real
// producer. Marshaling entries uniformly as <entry> would emit an
// <entry length="..."> for a blank, which MLT does not treat as a gap.
func (pl Playlist) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "playlist"}
	start.Attr = []xml.Attr{{Name: xml.Name{Local: "id"}, Value: pl.ID}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, ent := range pl.Entries {
		var child xml.StartElement
		if ent.Blank != "" {
			child = xml.StartElement{
				Name: xml.Name{Local: "blank"},
				Attr: []xml.Attr{{Name: xml.Name{Local: "length"}, Value: ent.Blank}},
			}
		} else {
			child = xml.StartElement{
				Name: xml.Name{Local: "entry"},
				Attr: []xml.Attr{{Name: xml.Name{Local: "producer"}, Value: ent.Producer}},
			}
		}
		if err := e.EncodeToken(child); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: child.Name}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML is the inverse of MarshalXML: it reads a <playlist> back into the
// model, mapping each <entry producer="..."/> and <blank length="..."/> child to
// an Entry so a round-trip preserves a delayed bed's leading gap.
func (pl *Playlist) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if a.Name.Local == "id" {
			pl.ID = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch el := tok.(type) {
		case xml.StartElement:
			var ent Entry
			switch el.Name.Local {
			case "entry":
				for _, a := range el.Attr {
					if a.Name.Local == "producer" {
						ent.Producer = a.Value
					}
				}
			case "blank":
				for _, a := range el.Attr {
					if a.Name.Local == "length" {
						ent.Blank = a.Value
					}
				}
			default:
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			if err := d.Skip(); err != nil {
				return err
			}
			pl.Entries = append(pl.Entries, ent)
		case xml.EndElement:
			if el.Name.Local == start.Name.Local {
				return nil
			}
		}
	}
}

// Tractor is an MLT <tractor>: what the consumer actually renders. A timeline
// with a playlist needs one, naming the track to play. Transitions combine
// tracks — which is how a second audio track becomes a mix rather than a
// replacement.
type Tractor struct {
	ID          string       `xml:"id,attr"`
	Tracks      []Track      `xml:"track"`
	Transitions []Transition `xml:"transition"`
}

// Transition is an MLT <transition>: it combines two tracks. Like a filter, the
// mlt_service property selects which one.
type Transition struct {
	ID         string     `xml:"id,attr,omitempty"`
	Properties []Property `xml:"property"`
}

// NewTransition builds a transition for the given mlt_service with ordered
// properties supplied as name/value pairs.
func NewTransition(service string, kv ...string) Transition {
	t := Transition{Properties: []Property{{Name: "mlt_service", Value: service}}}
	for i := 0; i+1 < len(kv); i += 2 {
		t.Properties = append(t.Properties, Property{Name: kv[i], Value: kv[i+1]})
	}
	return t
}

// Track names a producer or playlist the tractor plays.
type Track struct {
	Producer string `xml:"producer,attr"`
}

// Sequence wires a playlist of the given producers into a tractor, making the
// timeline a single ordered cut sequence.
func (t *Timeline) Sequence(id string) {
	pl := Playlist{ID: id}
	for _, p := range t.Producers {
		pl.Entries = append(pl.Entries, Entry{Producer: p.ID})
	}
	t.Playlists = []Playlist{pl}
	t.Tractor = &Tractor{ID: id + "_tractor", Tracks: []Track{{Producer: id}}}
}

// DefaultBedGain is the level a music bed sits at when no gain is named: quiet
// enough that dialogue stays audible above it, matching the schema's promise
// that beds are mixed under the edit rather than substituted over it.
const DefaultBedGain = 0.15

// AddAudio lays a sound under the timeline — a music bed, a sting, room tone —
// as a genuine second track combined by MLT's `mix` transition. Mixing is the
// whole point: without a transition the extra track would replace the dialogue
// rather than sit beneath it.
//
// gain is the bed's level relative to full scale, and fadeIn/fadeOut are
// seconds. start offsets the sound into the timeline, so a sting can land on a
// moment rather than at the top. Filters are attached to the sound itself, so
// any MLT audio filter applies to the bed alone.
func (t *Timeline) AddAudio(id, resource string, gain, start, fadeIn, fadeOut float64,
	filters ...Filter) {

	if t.Tractor == nil {
		// Audio needs something to sit under; a bare producer list has no
		// track structure to mix into.
		t.Sequence("main")
	}
	if gain <= 0 {
		// A non-positive gain is unspecified, not a request for full scale:
		// fall back to the documented bed level so the dialogue stays audible
		// beneath. Rewriting to 1 here would drown the speech — the opposite
		// of a bed. Callers that validate their input (see applyAudio) reject
		// an explicit 0 before reaching this point.
		gain = DefaultBedGain
	}

	sound := Producer{ID: id, Resource: resource, Filters: filters}
	if fadeIn > 0 {
		sound.Filters = append(sound.Filters, NewFilter("fadeInVolume",
			"in", Clock(0), "out", Clock(fadeIn), "alpha", "1"))
	}
	if fadeOut > 0 {
		sound.Filters = append(sound.Filters, NewFilter("fadeOutVolume",
			"alpha", "1", "fade_out_duration", Clock(fadeOut)))
	}
	t.Producers = append(t.Producers, sound)

	pl := Playlist{ID: id + "_track"}
	if start > 0 {
		// A blank entry is how MLT delays a producer's entry on its track.
		pl.Entries = append(pl.Entries, Entry{Blank: Clock(start)})
	}
	pl.Entries = append(pl.Entries, Entry{Producer: id})
	t.Playlists = append(t.Playlists, pl)

	track := len(t.Tractor.Tracks)
	t.Tractor.Tracks = append(t.Tractor.Tracks, Track{Producer: pl.ID})
	// sum=1 mixes the tracks together; without it the b-track crossfades over
	// the a-track and the dialogue drops out.
	t.Tractor.Transitions = append(t.Tractor.Transitions, NewTransition("mix",
		"a_track", "0",
		"b_track", strconv.Itoa(track),
		"start", strconv.FormatFloat(gain, 'f', -1, 64),
		"end", strconv.FormatFloat(gain, 'f', -1, 64),
		"sum", "1",
		"always_active", "1",
	))
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
//
// Rounding happens once, up front, in whole milliseconds — before the value is
// split into hours and minutes. Splitting first and rounding second is what
// produced an out-of-range seconds field: h and m came from int() truncation on
// the unrounded value while %06.3f rounded the remainder independently, so
// 59.9996 printed as "00:00:60.000". There is no :60 on a clock, and melt
// cannot parse one, so a time landing within half a millisecond of a minute
// boundary broke the render. Carrying the rounding through the division makes
// that impossible to express.
func Clock(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	const (
		msPerSecond = 1000
		msPerMinute = 60 * msPerSecond
		msPerHour   = 60 * msPerMinute
	)
	ms := int64(math.Round(seconds * msPerSecond))
	return fmt.Sprintf("%02d:%02d:%06.3f",
		ms/msPerHour,
		(ms%msPerHour)/msPerMinute,
		float64(ms%msPerMinute)/msPerSecond)
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

// CropWindow builds the filter that keeps a source-pixel rectangle (x,y,w,h) of a
// srcW×srcH source and scales what remains to fill the profile. It is expressed
// as edge insets in source pixels — the coordinate space MLT's crop filter reads
// — so a window whose aspect matches the profile fills the frame with no bars,
// which is what makes a real head-and-shoulders reframe rather than a padded one.
//
// This is deliberately the plain "crop" service, not "qtcrop" (which renders
// black against the melt build the render worker drives). It omits use_profile
// on purpose: the insets are already in source pixels, so declaring them
// profile-relative would crop the wrong region. use_profile itself is a valid
// crop-and-fill form — PunchIn uses it — so this is a coordinate-space choice,
// not a workaround for a broken filter. The real black-render hazard is
// combining center=1 with explicit edges, which pads to black.
func CropWindow(srcW, srcH, x, y, w, h int) Filter {
	right := srcW - x - w
	if right < 0 {
		right = 0
	}
	bottom := srcH - y - h
	if bottom < 0 {
		bottom = 0
	}
	return NewFilter("crop",
		"left", fmt.Sprintf("%d", x),
		"right", fmt.Sprintf("%d", right),
		"top", fmt.Sprintf("%d", y),
		"bottom", fmt.Sprintf("%d", bottom),
	)
}

// Landscape returns the 16:9 profile a horizontal edit is delivered in.
func Landscape(fps int) *Profile {
	if fps <= 0 {
		fps = 30
	}
	return &Profile{
		Width: 1920, Height: 1080,
		DisplayAspectNum: 16, DisplayAspectDen: 9,
		FrameRateNum: fps, FrameRateDen: 1,
		Progressive:     1,
		SampleAspectNum: 1, SampleAspectDen: 1,
	}
}

// PunchIn builds the crop that pushes into a 16:9 frame and re-centres on a
// subject: keep is the fraction of the frame width retained (0.55 is a close
// single, 1.0 the untouched wide), and cx/cy are the subject's normalised
// centre. Crop amounts are expressed in profile pixels via use_profile, so the
// same timeline works regardless of the source's resolution. The kept window is
// clamped inside the frame, so a subject near an edge yields the closest legal
// framing instead of an invalid crop.
func PunchIn(p *Profile, keep, cx, cy float64) Filter {
	if p == nil {
		p = Landscape(30)
	}
	if keep <= 0 || keep > 1 {
		keep = 1
	}
	w, h := float64(p.Width), float64(p.Height)
	kw := w * keep
	kh := kw * h / w // preserve the profile's aspect

	left := clampf(cx*w-kw/2, 0, w-kw)
	top := clampf(cy*h-kh/2, 0, h-kh)

	return NewFilter("crop",
		"use_profile", "1",
		"left", fmt.Sprintf("%.0f", left),
		"right", fmt.Sprintf("%.0f", w-kw-left),
		"top", fmt.Sprintf("%.0f", top),
		"bottom", fmt.Sprintf("%.0f", h-kh-top),
	)
}

func clampf(v, lo, hi float64) float64 {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
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

// Parse decodes MLT XML bytes back into a Timeline — the inverse of Marshal. It
// is what lets an agent load a timeline it (or an earlier step) already wrote,
// adjust one filter, and write it back, rather than rebuilding the whole edit
// from search and detection just to change a grade.
func Parse(data []byte) (*Timeline, error) {
	var t Timeline
	if err := xml.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Service reports the filter's mlt_service — the value that selects which MLT
// filter this is ("crop", "subtitle", "avfilter.eq", ...). It is the stable name
// an edit targets to add or remove a filter, and is empty when unset (a filter
// Validate would reject).
func (f Filter) Service() string {
	for _, p := range f.Properties {
		if p.Name == "mlt_service" {
			return p.Value
		}
	}
	return ""
}

// Producer returns a pointer to the producer with the given id, or nil when no
// producer carries it. The pointer aliases the timeline's own slice, so mutating
// what it points at edits the timeline in place.
func (t *Timeline) Producer(id string) *Producer {
	for i := range t.Producers {
		if t.Producers[i].ID == id {
			return &t.Producers[i]
		}
	}
	return nil
}

// RemoveFilterService drops every filter with the given mlt_service from the
// producer and reports how many it removed, so an edit can tell a real removal
// from a no-op against a service the producer never carried.
func (p *Producer) RemoveFilterService(service string) int {
	kept := p.Filters[:0]
	removed := 0
	for _, f := range p.Filters {
		if f.Service() == service {
			removed++
			continue
		}
		kept = append(kept, f)
	}
	p.Filters = kept
	return removed
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
