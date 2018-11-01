package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
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

func handleConn(conn net.Conn) {
	defer conn.Close()

	go func() {
		for {
			msg := <-announcements // 从通道中读取信息
			io.WriteString(conn, msg)
		}
	}()

	var address string

	// 验证者输入他所拥有的tokens， tokens的值越大，越容易获得新区块的记账权
	io.WriteString(conn, "Enter token balance:")
	scanBalance := bufio.NewScanner(conn) //创建并返回一个从conn中读取数据的scanner
	for scanBalance.Scan() {
		//获取输入的数据，并将输入的值转为int
		balance, err := strconv.Atoi(scanBalance.Text())
		if err != nil {
			log.Printf("%v not a number: %v", scanBalance.Text(), err)
		}
		t := time.Now()
		//生成验证者的地址
		address = calculateHash(t.String())
		//将验证者的地址和token存储到validators
		validators[address] = balance
		fmt.Println(validators)
		break
	}
	io.WriteString(conn, "\n Enter a new BPM")
	scanBPM := bufio.NewScanner(conn)

	go func() {
		for {
			// 接收BPM 并将它加入到区块链中
			for scanBPM.Scan() {
				bpm, err := strconv.Atoi(scanBPM.Text())
				// 如果验证者试图提议一个伪造的block，例如包含一个不是整数的BPM， 那么程序会抛出错误
				// 会立即从验证器列表validators中删除该验证者，不再有资格参与到新块的铸造同时丢失相应的tokens
				if err != nil {
					log.Printf("%v not a number: %v", scanBPM.Text(), err)
					delete(validators, address)
					conn.Close()
				}

				mutex.Lock()
				oldLastIndex := Blockchain[len(Blockchain)-1]
				mutex.Unlock()

				//创建新的区块 然后将其发送到candidateBlocks 通道
				newBlock, err := generateBlock(oldLastIndex, bpm, address)
				if err != nil {
					log.Println(err)
					continue
				}
				if isBlockValid(newBlock, oldLastIndex) {
					candidateBlocks <- newBlock
				}
				io.WriteString(conn, "\n Enter anew BPM")
			}
		}
	}()

	// 周期性打印出最新的区块链信息
	for {
		time.Sleep(time.Minute)
		mutex.Lock()
		output, err := json.Marshal(Blockchain)
		if err != nil {
			log.Fatal(err)
		}
		io.WriteString(conn, string(output)+"\n")
	}
}

// pickWinner 创建一个验证者的幸运池lotteryPool 选择一个验证者来铸造一个新的区块
func pickWinner() {
	time.Sleep(30 * time.Second)
	mutex.Lock()
	temp := tempBlocks
	mutex.Unlock()

	lotteryPool := []string{}
	if len(temp) > 0 {
		// 小小的修改了传统的pos算法
		// 传统的pos， 验证者可以在不不提交区块的情况下参与
	OUTER:
		for _, block := range temp {
			// if already in lottery pool, skip
			for _, node := range lotteryPool {
				if block.Validator == node {
					continue OUTER
				}
			}

			// lock list of validator to prevent data race
			mutex.Lock()
			setValidators := validators
			mutex.Unlock()

			// 获取验证者的tokens
			k, ok := setValidators[block.Validator]
			if ok {
				// 向lotteryPool 追加k条数据, k 代表当前当前验证者的tokens
				for i := 0; i < k; i++ {
					lotteryPool = append(lotteryPool, block.Validator)
				}
			}
		}

		// 通过随机值获取获胜节点的地址
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		lotteryWinner := lotteryPool[r.Intn(r.Intn(len(lotteryPool)))]

		// 把获胜者区块链添加到整条区块链上 然后通知所有节点关于获胜者的消息
		for _, block := range temp {
			if block.Validator == lotteryWinner {
				mutex.Lock()
				Blockchain = append(Blockchain, block)
				mutex.Unlock()
				for _ := range validators {
					announcements <- "\nwinning validator: " + lotteryWinner + "\n"
				}
				break
			}
		}
	}

	mutex.Lock()
	tempBlocks = []Block{}
	mutex.Unlock()
}

func main() {

	err := godotenv.Load() //加载.env文件到环境变量中 通过os.Getenv("key") 的方式获取value
	if err != nil {
		log.Fatal("Load the env error: ", err)
	}

	//创建genesisBlock
	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), 0, calculateBlockHash(genesisBlock), "", ""}
	spew.Dump(genesisBlock) //控制台格式化输出
	Blockchain = append(Blockchain, genesisBlock)

	httpPort := os.Getenv("PORT")

	// start tcp server
	server, err := net.Listen("tcp", ":"+httpPort)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Http Server Listening on port: ", httpPort)
	defer server.Close()

	// 启动一个goroutine 从candidateBlocks 通道中获取提议的区块，然后添加到临时缓冲区tempBlocks中
	go func() {
		for candidate := range candidateBlocks {
			mutex.Lock()
			tempBlocks = append(tempBlocks, candidate)
			mutex.Unlock()
		}
	}()

	// 启动了一个goroutine 完成pickWinner函数
	go func() {
		for {
			pickWinner()
		}
	}()

	//接收验证者的连接
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}
