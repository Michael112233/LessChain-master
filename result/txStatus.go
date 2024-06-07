package result

// 交易状态
// 新增交易状态后，需要补充下方对应的函数
const (
	DefaultStatus uint64 = iota

	IntraSuccess // 1
	CrossTXType1Success
	CrossTXType2Success
	RollbackSuccess

	Dropped
	CrossTXType2Fail
	RollbackFail
)

func GetStatusString(status uint64) string {
	return getStatusStr(status)
}

func getStatusStr(status uint64) string {
	if status == DefaultStatus {
		return "DefaultStatus"
	} else if status == IntraSuccess { // 1
		return "IntraSuccess"
	} else if status == CrossTXType1Success {
		return "CrossTXType1Success"
	} else if status == CrossTXType2Success {
		return "CrossTXType2Success"
	} else if status == RollbackSuccess {
		return "RollbackSuccess"
	} else if status == Dropped {
		return "Dropped"
	} else if status == CrossTXType2Fail {
		return "CrossTXType2Fail"
	} else if status == RollbackFail {
		return "RollbackFail"
	} else {
		return "Unknown"
	}
}

/* SetTXReceipt functions */
func checkTXFinished(status uint64) bool {
	return status == IntraSuccess ||
		status == CrossTXType2Success
}

/* 打印 statusList */
func getStatusListStr(statusList []uint64) string {
	str := ""
	for _, status := range statusList {
		tmp := getStatusStr(status) + ","
		str = str + tmp
	}
	if len(str) == 0 {
		return "[]"
	}
	str = "[" + str[:len(str)-1] + "]"
	return str
}
