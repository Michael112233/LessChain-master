package utils

import (
	"math/rand"
	"time"
)

var probability float64

func init() {
	rand.Seed(1)
	probability = 0.3
}

func SetSeed1() {
	rand.Seed(1)
}

func SetTimeNowSeed() {
	rand.Seed(time.Now().UnixNano())
}

func SetProb(prob float64) {
	probability = prob
}

func GetProb() float64 {
	return probability
}

func GetRandWithN(N int) int {
	/* 生成0～N的随机数 */
	return rand.Intn(N)
}

func IsSelectWithProb(prob float64) bool {
	/* 生成0～99的随机数 */
	num := rand.Intn(100)
	fnum := float64(num) / 100
	return fnum < prob
}

func IsSelect() bool {
	/* 生成0～99的随机数 */
	num := rand.Intn(100)
	fnum := float64(num) / 100
	return fnum < probability
}

func GetRand() int {
	return rand.Intn(100)
}
