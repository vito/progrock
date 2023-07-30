package progrock

import "fmt"

// Labelf is a convenience function for creating a Label from a format string
// and values.
//
// If no values are given, format is used verbatim, so Labelf also doubles as a
// plain string value Label constructor.
func Labelf(name string, format string, vals ...any) *Label {
	var value string
	if len(vals) > 0 {
		value = format
	} else {
		value = fmt.Sprintf(format, vals...)
	}

	return &Label{
		Name:  name,
		Value: value,
	}
}

// MessageOpt adds the Label to the Message's labels.
func (l *Label) MessageOpt(m *Message) {
	m.Labels = append(m.Labels, l)
}
