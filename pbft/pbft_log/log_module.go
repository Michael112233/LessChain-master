package pbft_log

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
)

type PbftLog struct {
	Plog *log.Logger
}

const (
	LogWrite_path string = "./pbftLog"
)

func NewPbftLog(sid, nid uint32) *PbftLog {

	pfx := fmt.Sprintf("S%dN%d: ", sid, nid)
	writer1 := os.Stdout

	dirpath := LogWrite_path + "/S" + strconv.Itoa(int(sid))
	err := os.MkdirAll(dirpath, os.ModePerm)
	if err != nil {
		log.Panic(err)
	}
	writer2, err := os.OpenFile(dirpath+"/N"+strconv.Itoa(int(nid))+".log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		log.Panic(err)
	}
	pl := log.New(io.MultiWriter(writer1, writer2), pfx, log.Lshortfile|log.Ldate|log.Ltime)
	fmt.Println()

	return &PbftLog{
		Plog: pl,
	}
}
