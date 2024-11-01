package backend

import (
	"github.com/jhillyerd/enmime"
	"github.com/phires/go-guerrilla/backends"
	"github.com/phires/go-guerrilla/log"
	"github.com/phires/go-guerrilla/mail"
)

type MimeParserProcessor struct {
	logger log.Logger
}

func MimeParser() backends.Decorator {
	p_mime := &MimeParserProcessor{
		logger: backends.Log(),
	}

	return func(p backends.Processor) backends.Processor {
		return backends.ProcessWith(
			func(e *mail.Envelope, task backends.SelectTask) (backends.Result, error) {
				if task == backends.TaskSaveMail {
					// read envelope to enmime envelope
					envReader := e.NewReader()

					env, err := enmime.ReadEnvelope(envReader)
					if err != nil {
						p_mime.logger.Warn("Failed to parse email to MIME envelope.")
					}

					// save enmime Envelope to the guerilla envelope
					e.Values["envelope_mime"] = env
					// next processor
					return p.Process(e, task)
				} else {
					// next processor
					return p.Process(e, task)
				}
			})
	}
}
