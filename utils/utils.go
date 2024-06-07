package utils

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"go-w3chain/log"
	"io"
	"os"
	"reflect"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

type AddressInfo struct {
	AddrTable map[common.Address]int // 账户地址 映射到 shardID （monoxide 不需要）
}

var (
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrTest                 = fmt.Errorf("err %s", "test")
)

// used in consensus.go:(39,40,41)
// IntToHex converts an int64 to a byte array
func IntToHex(num int64) []byte {
	buff := new(bytes.Buffer)
	err := binary.Write(buff, binary.BigEndian, num)
	if err != nil {
		log.Error("InToHex err", "err", err)
	}

	return buff.Bytes()
}

// used in utxo_base58.go:30:2
// ReverseBytes reverses a byte array
func ReverseBytes(data []byte) {
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}

func Fatalf(format string, args ...interface{}) {
	w := io.MultiWriter(os.Stdout, os.Stderr)
	outf, _ := os.Stdout.Stat()
	errf, _ := os.Stderr.Stat()
	if outf != nil && errf != nil && os.SameFile(outf, errf) {
		w = os.Stderr
	}
	fmt.Fprintf(w, "Fatal: "+format+"\n", args...)
	os.Exit(1)
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

/**
 * 从给定的结构体指针中获得指定的字段，返回字段值组成的数组
 * structPointer 的类型必须是 *structType
 */
func GetFieldValue(structPointer interface{}, fieldName string) interface{} {
	getType := reflect.TypeOf(structPointer).Elem()
	getVal := reflect.ValueOf(structPointer).Elem()
	// fmt.Println(getType, getVal)

	for i := 0; i < getType.NumField(); i++ {
		field := getType.Field(i)
		value := getVal.Field(i)
		if field.Name == fieldName {
			return value.Interface()
		}
	}
	return nil
}

/**
 * 从给定的结构体数组中获得指定的字段，返回字段值组成的数组
 * list 的类型必须是 []*structType
 */
func GetFieldValueforList(list interface{}, fieldName string) []interface{} {
	val := reflect.ValueOf(list)
	res := make([]interface{}, val.Len())

	for i := 0; i < val.Len(); i++ {
		structPointer := val.Index(i).Interface()
		res[i] = GetFieldValue(structPointer, fieldName)
	}

	return res
}

func LastElem(arr interface{}) interface{} {
	val := reflect.ValueOf(arr)
	len := val.Len()

	return val.Index(len - 1).Interface()
}

func GetFieldValues(structValue interface{}) (fields map[string]interface{}) {
	fields = make(map[string]interface{})

	getType := reflect.TypeOf(structValue)
	getVal := reflect.ValueOf(structValue)

	for i := 0; i < getType.NumField(); i++ {
		name := getType.Field(i)
		value := getVal.Field(i)
		fields[name.Name] = value.Interface()
	}
	return
}

/**
 * 根据 尾数 id 划分
 */
func Addr2Shard(addr string, shardNum int) int {
	// 只取地址后四位已绝对够用
	addr = addr[len(addr)-4:]
	num, err := strconv.ParseInt(addr, 16, 32)
	// num, err := strconv.ParseInt(senderAddr, 10, 32)
	if err != nil {
		log.Error("Parse address to shardID error!", "err:", err)
	}
	return int(num) % shardNum
}

/* 将vrf值映射到分片ID
此方法暂时只适用于基于ecdsa的vrf算法
*/
func VrfValue2Shard(value []byte, shardNum uint32) uint32 {
	return uint32(uint8(value[0])) % shardNum
}

func IsComLeader(nodeId uint32) bool {
	return nodeId == 0
}

func IsShardLeader(nodeId uint32) bool {
	return nodeId == 0
}

func GetHash(val []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(val)
	return hasher.Sum(nil)
}

// 对传入的任意类型进行gob编码
func EncodeAny(any interface{}) []byte {
	var buffer bytes.Buffer
	encoder := gob.NewEncoder(&buffer)

	// 对对象进行编码
	err := encoder.Encode(any)
	if err != nil {
		log.Error("encode fail", "err", err)
	}

	return buffer.Bytes()
}
