package core

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/trie"
	// "github.com/ethereum/go-ethereum/rlp"
	// "github.com/ethereum/go-ethereum/core/types"
)

func getTransaction() {
	jsonFile, err := os.Open("transaction.json")
	defer jsonFile.Close()
	byteValue, err := ioutil.ReadAll(jsonFile)
	var tx Transaction
	json.Unmarshal(byteValue, &tx)

	rlp, err := tx.GetRLP()

	mpt := new(trie.Trie)
	mpt.Update([]byte{1, 2, 3}, rlp)

	fmt.Println(err)
	fmt.Println(tx)
	fmt.Println(tx.Value)
	// fmt.Println(tx.V1)
	// fmt.Println(tx.V2)
	fmt.Println(rlp)

}

func mytestCreateTransaction() {
	amount := math.BigPow(2, 3)
	ntx := NewTransaction(0, common.Address{}, common.Address{}, 1, amount)

	txs := make([]*Transaction, 70)
	tx := txs[0]

	rval := reflect.ValueOf(tx)
	typ := rval.Type()
	kind := typ.Kind()
	fmt.Println("In func mytestCreateTransaction--------")
	fmt.Println("ntx:", ntx)
	fmt.Println("rval = reflect.ValueOf(tx):", rval)
	fmt.Println("typ = rval.type():", typ)  // *main.Transaction
	fmt.Println("kind = typ.Kind():", kind) // ptr
	fmt.Println("typ.Elem():", typ.Elem())  // main.Transaction

	typ = typ.Elem()
	for i := 0; i < typ.NumField(); i++ {
		rf := typ.Field(i)
		fmt.Println("rf:", rf) // main.Transaction
	}

	fmt.Println("---------------------")

}
