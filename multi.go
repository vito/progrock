package progrock

import "errors"

type MultiWriter []Writer

func (mw MultiWriter) WriteStatus(v *StatusUpdate) error {
	for _, w := range mw {
		if err := w.WriteStatus(v); err != nil {
			return err
		}
	}
	return nil
}

func (mw MultiWriter) Close() error {
	errs := make([]error, len(mw))
	for i, w := range mw {
		errs[i] = w.Close()
	}
	return errors.Join(errs...)
}
