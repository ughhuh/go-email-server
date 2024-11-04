package backend

import (
	"github.com/jhillyerd/enmime"
	"github.com/phires/go-guerrilla/backends"
	"github.com/phires/go-guerrilla/log"
	"github.com/phires/go-guerrilla/mail"
)

// ----------------------------------------------------------------------------------
// Processor Name: MimeParser
// ----------------------------------------------------------------------------------
// Description   : Parse the envelope to enmime Envelope
// ----------------------------------------------------------------------------------
// Config Options: None
// --------------:-------------------------------------------------------------------
// Input         : e *mail.Envelope
// ----------------------------------------------------------------------------------
// Output        : Envelope is saved to e.Values["envelope_mime"]
//
//	: enmime Envelope documentation: https://pkg.go.dev/github.com/jhillyerd/enmime#Envelope
//
// ----------------------------------------------------------------------------------

type MimeParser struct {
	logger log.Logger
}

var MimeParserProcessor = func() backends.Decorator {
	p_mime := &MimeParser{
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
