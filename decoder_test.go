package alarmdecoder

import (
	"bytes"
	"io"
	"reflect"
	"testing"

	"github.com/pkg/errors"
)

func TestParse(t *testing.T) {
	cases := []struct {
		raw  string
		want Message
	}{
		{
			`[10000601100000003A--],045,[f71f00000045001c28020000000000],"****DISARMED****  READY TO ARM  "`,
			Message{
				Ready:         true,
				Beeps:         6,
				ACPower:       true,
				ChimeEnabled:  true,
				Mode:          "A",
				Zone:          "045",
				RawData:       "[f71f00000045001c28020000000000]",
				KeypadMessage: "****DISARMED****  READY TO ARM",
			},
		},
		{
			`[00000000011000003A--],,,"test"`,
			Message{
				AlarmHasOccured: true,
				AlarmSounding:   true,
				Mode:            "A",
				KeypadMessage:   "test",
			},
		},
		{
			`[00000000000001003A--],,,"test"`,
			Message{
				Fire:          true,
				Mode:          "A",
				KeypadMessage: "test",
			},
		},
		{
			`[00000000000000103A--],,,"test"`,
			Message{
				SystemIssue:   true,
				Mode:          "A",
				KeypadMessage: "test",
			},
		},
	}

	for i, c := range cases {
		c.want.UnparsedMessage = c.raw
		out, err := ParseMessage(c.raw)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(out, c.want) {
			t.Errorf("%d. ParseMessage(%q) = %#v; not %#v", i, c.raw, out, c.want)
		}
	}
}

type dummyRW struct {
	r io.Reader
	w io.Writer
}

func (rw *dummyRW) Read(p []byte) (n int, err error) {
	if rw.r != nil {
		return rw.r.Read(p)
	}
	return 0, errors.Errorf("unimplemented")
}

func (rw *dummyRW) Write(p []byte) (n int, err error) {
	if rw.w != nil {
		return rw.w.Write(p)
	}
	return 0, errors.Errorf("unimplemented")
}

func TestRead(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("[00000000011000003A--],,,\"test\"\n")
	rw := dummyRW{
		r: &buf,
	}
	ad := New(&rw)
	out, err := ad.Read()
	if err != nil {
		t.Fatal(err)
	}
	want := Message{
		UnparsedMessage: "[00000000011000003A--],,,\"test\"",
		AlarmHasOccured: true,
		AlarmSounding:   true,
		Mode:            "A",
		KeypadMessage:   "test",
	}
	if !reflect.DeepEqual(out, want) {
		t.Errorf("got %+v; wanted %+v", out, want)
	}
}
