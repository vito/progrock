package progrock

type Discard struct{}

func (Discard) WriteStatus(*StatusUpdate) error { return nil }
func (Discard) Close() error                    { return nil }
