package progrock

import "fmt"

// NewLabel is a convenience function for creating a Label, primarily for use
// as a MessageOpt.
func NewLabel(name string, valueAny any) *Label {
	value, ok := valueAny.(string)
	if !ok {
		value = fmt.Sprintf("%s", valueAny)
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
