package alog

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

const (
	ROTATE_BY_DAY  int = 1 // rotate by day
	ROTATE_BY_HOUR int = 2 // rotate by hour
	ROTATE_BY_SIZE int = 3 // rotate by size
)

type Mrecord map[string]interface{}

type jsonFile struct {
	file       *os.File
	bufio      *bufio.Writer
	gzip       *gzip.Writer
	json       *json.Encoder
	changed    bool
	maxSize    int64
	curSize    int64
	curFile    string
	rotateMode int
}

func openLogFile(fileName, fileType string, compress bool, rotateMode int, maxSize int64) (*jsonFile, error) {
	fullName := fileName + fileType
	if _, err := os.Stat(fullName); err == nil {
		os.Rename(fullName, fileName+".01"+fileType)
		fullName = fileName + ".02" + fileType
	} else if _, err := os.Stat(fileName + ".01" + fileType); err == nil {
		for fileId := 1; true; fileId++ {
			fullName = fileName + fmt.Sprintf(".%02d", fileId) + fileType
			if _, err := os.Stat(fullName); err != nil {
				break
			}
		}
	}
	file, err := os.OpenFile(fullName, os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return nil, err
	}
	jsonfile := &jsonFile{file: file, curFile: fullName, rotateMode: rotateMode}
	if compress {
		jsonfile.bufio = bufio.NewWriter(jsonfile.file)
		jsonfile.gzip = gzip.NewWriter(jsonfile.bufio)
		jsonfile.json = json.NewEncoder(jsonfile.gzip)
	} else {
		jsonfile.bufio = bufio.NewWriter(jsonfile.file)
		jsonfile.json = json.NewEncoder(jsonfile.bufio)
	}
	if jsonfile.rotateMode == ROTATE_BY_SIZE && maxSize == 0 {
		jsonfile.maxSize = 1024 * 1024 * 1024 * 25
	} else {
		jsonfile.maxSize = maxSize
	}

	return jsonfile, nil
}

func (jsonfile *jsonFile) Put(rec Mrecord) {
	if err := jsonfile.json.Encode(rec); err != nil {
		log.Println("log write failed:", err.Error())
	}
	jsonfile.changed = true
}

func (jsonfile *jsonFile) Maxsize(maxSize int64) {
	jsonfile.maxSize = maxSize
}

func (jsonfile *jsonFile) Flush() error {
	if !jsonfile.changed {
		return nil
	}
	if jsonfile.gzip != nil {
		if err := jsonfile.gzip.Flush(); err != nil {
			return err
		}
	}
	if err := jsonfile.bufio.Flush(); err != nil {
		return err
	}
	if err := jsonfile.file.Sync(); err != nil {
		return err
	}

	jsonfile.changed = false
	if jsonfile.rotateMode == ROTATE_BY_SIZE {
		fileInfo, _ := os.Stat(jsonfile.curFile)
		jsonfile.curSize = fileInfo.Size()
	}
	return nil
}

func (jsonfile *jsonFile) Close() error {
	if jsonfile.gzip != nil {
		if err := jsonfile.gzip.Flush(); err != nil {
			return err
		}
		if err := jsonfile.gzip.Close(); err != nil {
			return err
		}
	}
	if err := jsonfile.bufio.Flush(); err != nil {
		return err
	}
	if err := jsonfile.file.Sync(); err != nil {
		return err
	}
	return jsonfile.file.Close()
}

// Log container
type Logger struct {
	dir       string
	recChan   chan Mrecord
	closeChan chan int
	closeWait sync.WaitGroup
	jsonfile  *jsonFile
}

//create a new Log container
func Create(dir string, rotateMode int, fileType string, compress bool) (*Logger, error) {
	if compress {
		fileType += ".gz"
	}

	logger := &Logger{
		dir:       dir,
		closeChan: make(chan int),
		recChan:   make(chan Mrecord, 1024),
	}
	if err := logger.rotateFile(rotateMode, fileType, compress); err != nil {
		return nil, err
	}

	logger.closeWait.Add(1)
	go func() {
		var (
			fileTimer *time.Timer
			now       = time.Now()
		)
		switch rotateMode {
		case ROTATE_BY_DAY:
			// next day's time line
			fileTimer = time.NewTimer(time.Date(
				now.Year(), now.Month(), now.Day(),
				0, 0, 0, 0, now.Location(),
			).Add(24 * time.Hour).Sub(now))
		case ROTATE_BY_HOUR:
			// next hour's time line
			fileTimer = time.NewTimer(time.Date(
				now.Year(), now.Month(), now.Day(),
				now.Hour(), 0, 0, 0, now.Location(),
			).Add(time.Hour).Sub(now))
		case ROTATE_BY_SIZE:
			// check log file size per 10 minutes
			fileTimer = time.NewTimer(time.Date(
				now.Year(), now.Month(), now.Day(),
				now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location(),
			).Add(time.Second * 10).Sub(now))
		}

		// flush per 5 seconds
		flushTicker := time.NewTicker(5 * time.Second)
		defer func() {
			flushTicker.Stop()
			logger.jsonfile.Close()
			logger.closeWait.Done()
		}()

		for {
			select {
			case rec := <-logger.recChan:
				logger.jsonfile.Put(rec)
			case <-flushTicker.C:
				if err := logger.jsonfile.Flush(); err != nil {
					log.Println("log flush failed:", err.Error())
				}
			case <-fileTimer.C:

				switch rotateMode {
				case ROTATE_BY_DAY:
					if err := logger.rotateFile(rotateMode, fileType, compress); err != nil {
						panic(err)
					}
					fileTimer = time.NewTimer(24 * time.Hour)
				case ROTATE_BY_HOUR:
					if err := logger.rotateFile(rotateMode, fileType, compress); err != nil {
						panic(err)
					}
					fileTimer = time.NewTimer(time.Hour)
				case ROTATE_BY_SIZE:
					if logger.jsonfile.curSize >= logger.jsonfile.maxSize {
						if err := logger.rotateFile(rotateMode, fileType, compress); err != nil {
							panic(err)
						}
					}
					fileTimer = time.NewTimer(time.Second * 10)
				}
			case <-logger.closeChan:
				for {
					select {
					case rec := <-logger.recChan:
						logger.jsonfile.Put(rec)
					default:
						return
					}
				}
			}
		}
	}()

	return logger, nil
}

// rotate new log save file
func (logger *Logger) rotateFile(rotateMode int, fileType string, compress bool) error {
	var (
		dirName  string
		fileName string
		now      = time.Now()
	)

	// get the dir and log file name
	switch rotateMode {
	case ROTATE_BY_DAY:
		dirName = logger.dir + "/" + now.Format("2006-01/")
		fileName = dirName + now.Format("2006-01-02")
	case ROTATE_BY_HOUR:
		dirName = logger.dir + "/" + now.Format("2006-01/2006-01-02/")
		fileName = dirName + now.Format("2006-01-02_03")
	case ROTATE_BY_SIZE:
		dirName = logger.dir + "/logs/"
		fileName = dirName + "alog"
	}

	// make sure the dir is exists
	if err := os.MkdirAll(dirName, 0755); err != nil {
		return err
	}

	// rotate before close the old file
	var maxSize int64
	if logger.jsonfile != nil {
		maxSize = logger.jsonfile.maxSize
		if err := logger.jsonfile.Close(); err != nil {
			return err
		}
	}

	//open or create a new log file
	jsonfile, err := openLogFile(fileName, fileType, compress, rotateMode, maxSize)
	if err != nil {
		return err
	}
	logger.jsonfile = jsonfile

	return nil
}

// close the log container
func (logger *Logger) Close() {
	close(logger.closeChan)
	logger.closeWait.Wait()
}

// put the log msg to the log file
func (logger *Logger) Log(rec Mrecord) {
	logger.recChan <- rec
}
