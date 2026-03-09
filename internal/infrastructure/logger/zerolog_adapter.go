package logger

import (
	"os"
	"time"

	"github.com/SilentPlaces/rate_limiter/internal/application/ports"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ZeroLogger struct {
	logger zerolog.Logger
}

func NewZeroLogger() *ZeroLogger {
	zerolog.TimeFieldFormat = time.RFC3339
	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	return &ZeroLogger{logger: l}
}

func (z *ZeroLogger) Info(msg string, fields ...ports.Field) {
	e := z.logger.Info()
	for _, f := range fields {
		e = e.Interface(f.Key, f.Val)
	}
	e.Msg(msg)
}

func (z *ZeroLogger) Error(msg string, fields ...ports.Field) {
	e := z.logger.Error()
	for _, f := range fields {
		e = e.Interface(f.Key, f.Val)
	}
	e.Msg(msg)
}

func (z *ZeroLogger) Debug(msg string, fields ...ports.Field) {
	e := z.logger.Debug()
	for _, f := range fields {
		e = e.Interface(f.Key, f.Val)
	}
	e.Msg(msg)
}
