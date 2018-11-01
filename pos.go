package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// 区块
type Block struct {
	Index     int
	Timestamp string
	BPM       int
	Hash      string
	PrevHash  string
	Validator string
}

// 一系列已经被验证过的区块放入区块链中
var Blockchain []Block

// 临时存储单元，区块被选出来并添加到Blockchain之前，临时存储在这里
var tempBlocks []Block

// candidateBlocks 接收需要确认的区块
var candidateBlocks = make(chan Block)

// announcements 广播获胜的验证人到所有节点
var announcements = make(chan string)

var mutex = &sync.Mutex{}

// validators 节点的存储map 以及每个节点持有的token数
var validators = make(map[string]int)

//  generate new block
func generateBlock(oldBlock Block, BPM int, address string) (Block, error) {
	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.PrevHash
	newBlock.Hash = calculateBlockHash(newBlock)
	newBlock.Validator = address

	return newBlock, nil
}

//sha256 hash
func calculateHash(s string) string {
	hash32 := sha256.Sum256([]byte(s))
	hash := hash32[:]

	return hex.EncodeToString(hash)
}

// 对区块进行hash
func calculateBlockHash(block Block) string {
	record := string(block.Index) + block.Timestamp + string(block.BPM) + block.PrevHash
	return calculateHash(record)
}

// 验证区块
func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if calculateBlockHash(newBlock) != newBlock.Hash {
		return false
	}

	return true
}

func main() {

	err := godotenv.Load() //加载.env文件到环境变量中 通过os.Getenv("key") 的方式获取value
	if err != nil {
		log.Fatal("Load the env error: ", err)
	}
	fmt.Println(os.Getenv("PORT"))
}
