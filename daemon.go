package main

import (
	"context"
	"os"
	"time"

	"google.golang.org/grpc"

	pb "github.com/OpenPeeDeeP/ChessClock-Daemon/chessclock"
	"github.com/OpenPeeDeeP/ChessClock-Daemon/store"
	"google.golang.org/grpc/codes"
)

//ChessClockDaemon is the daemon that handles tasks
type ChessClockDaemon struct {
	store store.Storer
}

//NewDaemon creates a new daemon using the specified store
func NewDaemon(store store.Storer) *ChessClockDaemon {
	return &ChessClockDaemon{
		store: store,
	}
}

//Start starts a task
func (ccd *ChessClockDaemon) Start(ctx context.Context, req *pb.StartRequest) (*pb.StartResponse, error) {
	if req.GetTimestamp() == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "Must specify a timestamp")
	}
	if req.GetTag() == "" {
		return nil, grpc.Errorf(codes.InvalidArgument, "Must specify a tag")
	}
	err := ccd.store.Start(req.GetTimestamp(), req.GetTag(), req.GetDescription())
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, err.Error())
	}
	return &pb.StartResponse{}, nil
}

//Stop will stop the previous task
func (ccd *ChessClockDaemon) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	if req.GetTimestamp() == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "Must specify a timestamp")
	}
	err := ccd.store.Stop(req.GetTimestamp(), req.GetReason())
	if err != nil {
		return nil, grpc.Errorf(codes.Internal, err.Error())
	}
	return &pb.StopResponse{}, nil
}

//Schedule list all tasks and when they were started
func (ccd *ChessClockDaemon) Schedule(ctx context.Context, req *pb.ScheduleRequest) (*pb.ScheduleResponse, error) {
	if req.GetDate() == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "Must specify a date for the timesheet")
	}
	events, err := ccd.store.Events(req.GetDate())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, grpc.Errorf(codes.NotFound, err.Error())
		}
		return nil, grpc.Errorf(codes.Internal, err.Error())
	}
	schedule := getSchedule(events)
	return &pb.ScheduleResponse{
		Tasks: schedule,
	}, nil
}

//Tally list all the tasks and how long they were worked on
func (ccd *ChessClockDaemon) Tally(ctx context.Context, req *pb.TallyRequest) (*pb.TallyResponse, error) {
	if req.GetDate() == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "Must specify a date for the timesheet")
	}
	events, err := ccd.store.Events(req.GetDate())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, grpc.Errorf(codes.NotFound, err.Error())
		}
		return nil, grpc.Errorf(codes.Internal, err.Error())
	}
	tally := getTally(events)
	return &pb.TallyResponse{
		Tasks: tally,
	}, nil
}

//ListTimeSheets list all the available time sheets
func (ccd *ChessClockDaemon) ListTimeSheets(ctx context.Context, req *pb.ListTimeSheetsRequest) (*pb.ListTimeSheetsResponse, error) {
	timeSheets, err := ccd.store.TimeSheets()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, grpc.Errorf(codes.NotFound, err.Error())
		}
		return nil, grpc.Errorf(codes.Internal, err.Error())
	}
	return &pb.ListTimeSheetsResponse{
		Dates: timeSheets,
	}, nil
}

//ListTags list all the tags for a given time sheet
func (ccd *ChessClockDaemon) ListTags(ctx context.Context, req *pb.ListTagsRequest) (*pb.ListTagsResponse, error) {
	if req.GetDate() == 0 {
		return nil, grpc.Errorf(codes.InvalidArgument, "Must specify a date for the timesheet")
	}
	events, err := ccd.store.Events(req.GetDate())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, grpc.Errorf(codes.NotFound, err.Error())
		}
		return nil, grpc.Errorf(codes.Internal, err.Error())
	}
	tags := getTags(events)
	return &pb.ListTagsResponse{
		Tags: tags,
	}, nil
}

//Version returns the version of the daemon
func (ccd *ChessClockDaemon) Version(context.Context, *pb.VersionRequest) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{
		Version: version,
	}, nil
}

func getSchedule(events []*store.Event) []*pb.ScheduleResponse_Task {
	schedule := make([]*pb.ScheduleResponse_Task, 0, len(events))
	for _, event := range events {
		if event.IsStart() {
			e := event.MustGetStartDetails()
			schedule = append(schedule, &pb.ScheduleResponse_Task{
				Timestamp:   e.StartTime,
				Tag:         e.Tag,
				Description: e.Description,
			})
		}
		if event.IsStop() {
			e := event.MustGetStopDetails()
			schedule = append(schedule, &pb.ScheduleResponse_Task{
				Timestamp: e.StopTime,
				Tag:       e.Reason.String(),
			})
		}
	}
	return schedule
}

func getTally(events []*store.Event) []*pb.TallyResponse_Task {
	tags := make(map[string][]int64, len(events))
	description := make(map[string]string, len(events))
	var prevTag string
	for _, event := range events {
		if event.IsStart() {
			e := event.MustGetStartDetails()
			if prevTag != "" {
				tags[prevTag] = append(tags[prevTag], e.StartTime)
			}
			tags[e.Tag] = append(tags[e.Tag], e.StartTime)
			if e.Description != "" {
				description[e.Tag] = e.Description
			}
			prevTag = e.Tag
		}
		if event.IsStop() {
			e := event.MustGetStopDetails()
			if prevTag != "" {
				tags[prevTag] = append(tags[prevTag], e.StopTime)
			}
			tags[e.Reason.String()] = append(tags[e.Reason.String()], e.StopTime)
			prevTag = e.Reason.String()
		}
	}
	tagSpans := make([]*pb.TallyResponse_Task, 0, len(tags))
	for tag, times := range tags {
		if tag == pb.StopRequest_EndOfDay.String() {
			tagSpans = append(tagSpans, &pb.TallyResponse_Task{
				Timespan: 0,
				Tag:      tag,
			})
			continue
		}
		if len(times)%2 == 1 {
			times = append(times, time.Now().Unix())
		}
		var prevTime int64
		var totalTime int64
		for i, time := range times {
			if i%2 == 0 {
				prevTime = time
			} else {
				totalTime += time - prevTime
			}
		}
		tagSpan := &pb.TallyResponse_Task{
			Timespan: totalTime,
			Tag:      tag,
		}
		if desc, ok := description[tag]; ok {
			tagSpan.Description = desc
		}
		tagSpans = append(tagSpans, tagSpan)
	}
	return tagSpans
}

func getTags(events []*store.Event) []string {
	tags := make(map[string]struct{}, len(events))
	for _, event := range events {
		if event.IsStart() {
			e := event.MustGetStartDetails()
			if _, ok := tags[e.Tag]; !ok {
				tags[e.Tag] = struct{}{}
			}
		}
	}
	t := make([]string, 0, len(tags))
	for tag := range tags {
		t = append(t, tag)
	}
	return t
}
