# alog
a simple log module written by golang

## how to use

1.install

go get github.com/aisondhs/alog


2.demo

```go

import "time"
import "github.com/aisondhs/alog"

func main() {
	defer alog.Close()
	alog.Init("./",alog.ROTATE_BY_DAY,false)
    alog.Info("Hello alog")

    alog.Info("This is test", "abc", "Golang is awesome!")
    logData := alog.Mrecord{"name":"aison","city":"shenzhen"}
    alog.Info("msg desc",logData)
    time.Sleep(6*time.Second)
}
```

if you want to save the logs to diffent dir due to the diffent business module,you can do below

```go
logger1 := log.New("dir1",alog.ROTATE_BY_DAY,true)
logger2 := log.New("dir2", alog.ROTATE_BY_HOUR,false)
```