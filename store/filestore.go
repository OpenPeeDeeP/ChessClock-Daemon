package store

import (
	"encoding/csv"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "github.com/OpenPeeDeeP/ChessClock-Daemon/chessclock"
	"github.com/OpenPeeDeeP/xdg"
)

//FileStore does absolutly nothing to store the logs
type FileStore struct {
	xdg      *xdg.XDG
	maxFiles int
}

//NewFileStore creates a new file store based on vendor and application name
func NewFileStore(vendor, application string, maxFiles int) *FileStore {
	x := xdg.New(vendor, application)
	return &FileStore{
		xdg:      x,
		maxFiles: maxFiles,
	}
}

//Start adds a start log to the file
func (s *FileStore) Start(timestamp int64, tag, description string) error {
	file, w, err := s.createWriteFile(timestamp)
	if err != nil {
		return err
	}
	defer func() {
		w.Flush()
		file.Close()
	}()
	err = w.Write([]string{"START", strconv.FormatInt(timestamp, 10), strings.ToUpper(tag), description})
	if err != nil {
		return err
	}
	return nil
}

//Stop adds a stop log to the file
func (s *FileStore) Stop(timestamp int64, reason pb.StopRequest_Reason) error {
	file, w, err := s.createWriteFile(timestamp)
	if err != nil {
		return err
	}
	defer func() {
		w.Flush()
		file.Close()
	}()
	err = w.Write([]string{"STOP", strconv.FormatInt(timestamp, 10), reason.String()})
	if err != nil {
		return err
	}
	return nil
}

//TimeSheets returns all the timesheets the file store is aware of
func (s *FileStore) TimeSheets() ([]int64, error) {
	files, err := ioutil.ReadDir(s.logDir())
	if err != nil {
		return nil, err
	}
	return parseFiles(files)
}

//Events returns all the events for the given date
func (s *FileStore) Events(date int64) ([]*Event, error) {
	file, r, err := s.createReadFile(date)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	logs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	events := make([]*Event, 0, len(logs))
	for _, log := range logs {
		var e *Event
		if strings.ToLower(log[0]) == "start" {
			if len(log) < 3 {
				return nil, errors.New("Start event must have 3 fields")
			}
			t, err := strconv.ParseInt(log[1], 10, 64)
			if err != nil {
				return nil, err
			}
			se := &StartEvent{
				StartTime: t,
				Tag:       log[2],
			}
			if len(log) > 3 {
				se.Description = log[3]
			}
			e = createStartEvent(se)
		}
		if strings.ToLower(log[0]) == "stop" {
			if len(log) < 3 {
				return nil, errors.New("Start event must have 3 fields")
			}
			t, err := strconv.ParseInt(log[1], 10, 64)
			if err != nil {
				return nil, err
			}
			reason, ok := pb.StopRequest_Reason_value[log[2]]
			if !ok {
				return nil, errors.New("Don't know what the reason is for stop event")
			}
			e = createStopEvent(&StopEvent{
				StopTime: t,
				Reason:   pb.StopRequest_Reason(reason),
			})
		}
		events = append(events, e)
	}
	return events, nil
}

func (s *FileStore) createWriteFile(timestamp int64) (*os.File, *csv.Writer, error) {
	if _, err := os.Stat(s.logDir()); os.IsNotExist(err) {
		err = os.MkdirAll(s.logDir(), 0700)
		if err != nil {
			return nil, nil, err
		}
	}
	fp := s.logPath(timestamp, true)
	file, err := os.OpenFile(fp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, nil, err
	}
	w := csv.NewWriter(file)
	w.Comma = '\t'
	err = s.rotateFiles()
	if err != nil {
		return nil, nil, err
	}
	return file, w, nil
}

func (s *FileStore) createReadFile(timestamp int64) (*os.File, *csv.Reader, error) {
	fp := s.logPath(timestamp, false)
	file, err := os.OpenFile(fp, os.O_RDONLY, 0600)
	if err != nil {
		return nil, nil, err
	}
	r := csv.NewReader(file)
	r.TrimLeadingSpace = true
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	return file, r, nil
}

func (s *FileStore) rotateFiles() error {
	files, err := ioutil.ReadDir(s.logDir())
	if err != nil {
		return err
	}
	for len(files) > s.maxFiles {
		sortedFiles, err := parseFiles(files)
		if err != nil {
			return err
		}
		err = os.Remove(s.logPath(sortedFiles[0], false))
		if err != nil {
			return err
		}
		files, err = ioutil.ReadDir(s.logDir())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) logPath(timestamp int64, convert bool) string {
	t := time.Unix(timestamp, 0)
	if convert {
		t = t.UTC()
	}
	return filepath.Join(s.logDir(), t.Format("2006_01_02")+".log")
}

func (s *FileStore) logDir() string {
	return filepath.Join(s.xdg.DataHome(), "logs")
}

type fileRange []int64

func (a fileRange) Len() int           { return len(a) }
func (a fileRange) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a fileRange) Less(i, j int) bool { return a[i] < a[j] }

func parseFiles(files []os.FileInfo) ([]int64, error) {
	times := make([]int64, 0, len(files))
	for _, file := range files {
		fileParts := strings.Split(file.Name(), ".")
		date, err := time.Parse("2006_01_02", fileParts[0])
		if err != nil {
			return nil, err
		}
		times = append(times, date.Unix())
	}
	fr := fileRange(times)
	sort.Sort(fr)
	return fr, nil
}
