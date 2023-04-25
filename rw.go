package progrock

type Reader interface {
	ReadStatus() (*StatusUpdate, bool)
}

type Writer interface {
	WriteStatus(*StatusUpdate) error
	Close() error
}
