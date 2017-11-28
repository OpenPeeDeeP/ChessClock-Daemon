package store

import (
	"errors"

	pb "github.com/OpenPeeDeeP/ChessClock-Daemon/chessclock"
)

//EventType is the type of event that is being parsed
type EventType int

const (
	//ETStart is a start event
	ETStart EventType = 1 + iota
	//ETStop is a stop event
	ETStop
)

//Eventer is any type that is an event
type Eventer interface{}

//Event is an event in the logs
type Event struct {
	Type    EventType
	Details Eventer
}

func createStartEvent(startEvent *StartEvent) *Event {
	return &Event{
		Type:    ETStart,
		Details: startEvent,
	}
}

func createStopEvent(stopEvent *StopEvent) *Event {
	return &Event{
		Type:    ETStop,
		Details: stopEvent,
	}
}

//MustGetStartDetails returns the details for a start event.
//Panics if it is not a start event.
func (e *Event) MustGetStartDetails() *StartEvent {
	start, ok := e.Details.(*StartEvent)
	if !ok {
		panic(errors.New("Event is not a start event"))
	}
	return start
}

//MustGetStopDetails returns the details for a stop event.
//Panics if it is not a stop event.
func (e *Event) MustGetStopDetails() *StopEvent {
	stop, ok := e.Details.(*StopEvent)
	if !ok {
		panic(errors.New("Event is not a stop event"))
	}
	return stop
}

//IsStart returns true if the event is a start event
func (e *Event) IsStart() bool {
	_, ok := e.Details.(*StartEvent)
	return ok
}

//IsStop returns true if the event is a stop event
func (e *Event) IsStop() bool {
	_, ok := e.Details.(*StopEvent)
	return ok
}

//StartEvent details for a start event
type StartEvent struct {
	StartTime   int64
	Tag         string
	Description string
}

//StopEvent details for a stop event
type StopEvent struct {
	StopTime int64
	Reason   pb.StopRequest_Reason
}

//Storer is any type that can store the event logs and return them
type Storer interface {
	Start(timestamp int64, tag, description string) error
	Stop(timestamp int64, reason pb.StopRequest_Reason) error
	TimeSheets() ([]int64, error)
	Events(date int64) ([]*Event, error)
}
