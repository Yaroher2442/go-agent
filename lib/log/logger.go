package log

import (
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path"
	"time"
)

var (
	Log = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
		Level(zerolog.DebugLevel).
		With().
		Timestamp().
		Logger()
)

func OverrideLogger(debug bool, dir string) {
	mw := io.MultiWriter(&lumberjack.Logger{
		Filename:   path.Join(dir, "pca.log"),
		MaxBackups: 5,   // files
		MaxSize:    100, // megabytes
		MaxAge:     7,   // days
	}, zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	Log = zerolog.New(mw).Level(zerolog.DebugLevel).
		With().
		Timestamp().
		Logger()
	if !debug {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
