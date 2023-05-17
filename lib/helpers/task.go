package helpers

import (
	"main/lib/log"
	"time"
)

type GoTask struct {
	Name         string
	StopChan     chan bool
	Timeout      time.Duration
	task         func() error
	onClose      func()
	nextExecTime time.Time
}

func NewAgentTask(Name string, timeOut time.Duration, f func() error, onStop func()) *GoTask {
	return &GoTask{
		Name:         Name,
		StopChan:     make(chan bool),
		Timeout:      timeOut,
		task:         f,
		onClose:      onStop,
		nextExecTime: time.Now().Add(timeOut),
	}
}

func (at *GoTask) tick() {
	at.nextExecTime = time.Now().Add(at.Timeout)
	log.Log.Debug().Time("next", at.nextExecTime).Str("Task", at.Name).Msg("")
}

func (at *GoTask) Run() {
	log.Log.Debug().Str("Task", at.Name).Msg("Start task")
	if at.Timeout > 0 {
		for {
			select {
			case <-at.StopChan:
				log.Log.Debug().Str("Task", at.Name).Msg("Stopped by channel")
				return
			case <-time.After(at.Timeout):
				log.Log.Debug().Str("Task", at.Name).Msg("")
				err := at.task()
				if err != nil {
					log.Log.Error().Err(err).Str("Task", at.Name).Msg("Exec failed")
				}
				at.tick()
			}
		}
	} else {
		select {
		case <-at.StopChan:
			log.Log.Debug().Str("Task", at.Name).Msg("Stopped by channel")
			return
		default:
			err := at.task()
			if err != nil {
				log.Log.Error().Err(err).Str("Task", at.Name).Msg("Exec failed")
			}
			at.tick()
		}
	}
}
