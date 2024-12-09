package alarmdecoder

import (
	"bufio"
	"io"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// ParseMessage parses a message from AD2PI.
func ParseMessage(s string) (Message, error) {
	m := Message{
		UnparsedMessage: s,
	}
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		return Message{}, errors.Errorf("expected 4 parts got: %#v", parts)
	}

	bits := parts[0]
	m.Ready = bits[1] == '1'
	m.ArmedAway = bits[2] == '1'
	m.ArmedHome = bits[3] == '1'
	m.BacklightOn = bits[4] == '1'
	m.ProgrammingMode = bits[5] == '1'
	var err error
	m.Beeps, err = strconv.Atoi(bits[6:7])
	if err != nil {
		return Message{}, err
	}
	m.ZoneBypassed = bits[7] == '1'
	m.ACPower = bits[8] == '1'
	m.ChimeEnabled = bits[9] == '1'
	m.AlarmHasOccured = bits[10] == '1'
	m.AlarmSounding = bits[11] == '1'
	m.BatteryLow = bits[12] == '1'
	m.EntryDelayDisabled = bits[13] == '1'
	m.Fire = bits[14] == '1'
	m.SystemIssue = bits[15] == '1'
	m.PerimeterOnly = bits[16] == '1'
	m.Mode = bits[18:19]

	m.Zone = parts[1]
	m.RawData = parts[2]
	msg := parts[3]
	m.KeypadMessage = strings.TrimSpace(msg[1 : len(msg)-1])
	return m, nil
}

// Message contains
type Message struct {
	UnparsedMessage string

	// Bit field

	// The bit field present on the keypad messages is where you're going to get
	// most of the information on your alarm system's current state. These are
	// all represented by a zero or one aside from the one exception. (beep)

	// 1 Indicates if the panel is READY
	Ready bool
	// 2 Indicates if the panel is ARMED AWAY
	ArmedAway bool
	// 3 Indicates if the panel is ARMED HOME
	ArmedHome bool
	// 4 Indicates if the keypad backlight is on
	BacklightOn bool
	// 5 Indicates if the keypad is in programming mode
	ProgrammingMode bool
	// 6 Number (1-7) indicating how many beeps are associated with the message
	Beeps int
	// 7 Indicates that a zone has been bypassed
	ZoneBypassed bool
	// 8 Indicates if the panel is on AC power
	ACPower bool
	// 9 Indicates if the chime is enabled
	ChimeEnabled bool
	// 10 Indicates that an alarm has occurred. This is sticky and will be cleared after a second disarm.
	AlarmHasOccured bool
	// 11 Indicates that an alarm is currently sounding. This is cleared after the first disarm.
	AlarmSounding bool
	// 12 Indicates that the battery is low
	BatteryLow bool
	// 13 Indicates that entry delay is off (ARMED INSTANT/MAX)
	EntryDelayDisabled bool
	// 14 Indicates that there is a fire
	Fire bool
	// 15 Indicates a system issue
	SystemIssue bool
	// 16 Indicates that the panel is only watching the perimeter (ARMED STAY/NIGHT)
	PerimeterOnly bool
	// 17 System specific bits. 4 bits packed into a HEX Nibble [0-9,A-F]
	// 18 Ademco or DSC Mode A or D
	Mode string
	// 19 Unused
	// 20 Unused

	// Numeric code

	// This number specifies which zone is affected by the message. For example,
	// if this message is for CHECK ZONE 22 then the numeric code would be 022.
	// Most of the time this is zero-padded base10, but there are rare occurrences
	// where this may be base16, such as ECP bus failures.
	Zone string

	// Raw data

	// This is the binary data associated with the message. It includes all of the
	// bit field entries that were separated out for you in the first field, as
	// well as the rest of the message for debugging and exploratory purposes.

	// There is one important piece of data included only in this field: the
	// keypad address mask. The four bytes starting at position 2 (zero-indexed)
	// indicate which keypads this message is intended for.
	RawData string

	// Alphanumeric Keypad Message

	// This section is the data that would be displayed on your keypad's screen.
	KeypadMessage string
}

// AlarmDecoder allows for interacting with an AlarmDecoder device over serial.
type AlarmDecoder struct {
	rw      io.ReadWriter
	scanner *bufio.Scanner
}

// New returns a new AlarmDecoder. Context can be cancelled to terminate the
// background goroutine.
func New(rw io.ReadWriter) *AlarmDecoder {
	return &AlarmDecoder{
		rw:      rw,
		scanner: bufio.NewScanner(rw),
	}
}

// Read returns a single message from the stream.
func (ad *AlarmDecoder) Read() (Message, error) {
	hasMsg := ad.scanner.Scan()
	if err := ad.scanner.Err(); err != nil {
		return Message{}, err
	}
	if hasMsg {
		return ParseMessage(ad.scanner.Text())
	}
	return Message{}, io.EOF
}
