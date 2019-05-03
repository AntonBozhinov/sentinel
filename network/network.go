package network

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/AntonBozhinov/sentinel/blockchain"
	"gopkg.in/vrecan/death.v3"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"
)

const (
	protocol = "tcp"
	version = 1
	commandLength = 12
)

var (
	nodeAddress     string
	minerAddress    string
	blocksInTransit [][]byte
	KnownNodes       = []string{"localhost:3000"}
	memoryPool       = make(map[string]blockchain.CoinTransaction)
)

type Addr struct {
	AddrList []string
}

type Block struct {
	AddrFrom string
	Block []byte
}

type GetBlocks struct {
	AddrFrom string
}

type GetData struct {
	AddrFrom string
	Type string
	ID []byte
}

type Inventory struct {
	AddrFrom string
	Type string
	Items [][]byte
}

type Tx struct {
	AddrFrom string
	Transaction []byte
}

type Version struct {
	Version int
	BestHeight int
	AddrFrom string
}

func CmdToBytes(cmd string) []byte  {
	var bytes [commandLength]byte
	for i, c := range cmd {
		bytes[i] = byte(c)
	}
	return bytes[:]
}

func BytesToCmd(bytes []byte) string  {
	var cmd []byte

	for _, b := range bytes {
		if b != 0x0 {
			cmd = append(cmd, b)
		}
	}

	return fmt.Sprintf("%s", cmd)
}


func CloseDB(chain *blockchain.BlockChain) {
	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	d.WaitForDeathWithFunc(func() {
		defer os.Exit(1)
		defer runtime.Goexit()
		chain.Database.Close()
	})
}

func GobEncode(data interface{}) []byte {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

func HandleConnection(conn net.Conn, chain *blockchain.BlockChain)  {
	req, err := ioutil.ReadAll(conn)
	defer conn.Close()
	if err != nil {
		log.Panic(err)
	}

	command := BytesToCmd(req[commandLength:])
	fmt.Printf("Recieved %s command\n", command)

	switch command {
	default:
		fmt.Println("Unknown command")
	}
}

func SendData(addr string, data []byte)  {
	conn, err := net.Dial(protocol, addr)
	if err != nil {
		fmt.Printf("%s is not available\n", addr)
		var updatedNodes []string
		for _, node := range KnownNodes {
			if node != addr {
				updatedNodes = append(updatedNodes, node)
			}
		}
		KnownNodes = updatedNodes
		return
	}
	defer conn.Close()
	_, err = io.Copy(conn, bytes.NewReader(data))
	if err != nil {
		log.Panic(err)

	}
}

func SendAddr(address string)  {
	nodes := Addr{KnownNodes}
	nodes.AddrList = append(nodes.AddrList, address)
	payload := GobEncode(nodes)
	request := append(CmdToBytes("addr"), payload...)

	SendData(address, request)
}

func SendBlock(addr string, block *blockchain.Block) {
	data := Block{addr, block.Serialize()}
	payload := GobEncode(data)
	request := append(CmdToBytes("block"), payload...)

	SendData(addr, request)
}

func SendInv(address, kind string, items [][]byte)  {
	inventory := Inventory{
		AddrFrom:address,
		Type: kind,
		Items: items,
	}
	payload := GobEncode(inventory)
	request := append(CmdToBytes("inv"), payload...)
	SendData(address, request)
}

func SendTx(address string, transaction *blockchain.CoinTransaction)  {
	data := Tx{
		AddrFrom: address,
		Transaction: transaction.Serialize(),
	}
	payload := GobEncode(data)
	request := append(CmdToBytes("tx"), payload...)
	SendData(address, request)
}

func SendVersion(address string, chain *blockchain.BlockChain) {
	bestHeight := chain.GetBestHeight()
	version := Version{
		AddrFrom: address,
		BestHeight: bestHeight,
		Version: version,
	}
	payload := GobEncode(version)
	request := append(CmdToBytes("version"), payload...)
	SendData(address, request)
}

func SendGetBlocks(address string) {
	payload := GobEncode(GetBlocks{nodeAddress})
	request := append(CmdToBytes("getblocks"), payload...)
	SendData(address, request)
}

func SendGetData(address, kind string, id []byte) {
	payload := GobEncode(GetData{
		AddrFrom: address,
		Type: kind,
		ID: id,
	})
	request := append(CmdToBytes("getdata"), payload...)
	SendData(address, request)
}

func HandleAddr(request []byte) {
	var buff bytes.Buffer
	var payload Addr
	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	KnownNodes = append(KnownNodes, payload.AddrList...)
	fmt.Printf("there are %d known nodes\n", len(KnownNodes))
	RequestBlocks()
}

func HandleBlocks(request []byte, chain *blockchain.BlockChain) {
	var buff bytes.Buffer
	var payload Block
	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	blockData := payload.Block
	block := blockchain.Deserialize(blockData)
	fmt.Printf("received a new block")
	chain.AddBlock(block)
	fmt.Printf("added block %x\n", block.Hash)
	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		SendGetData(payload.AddrFrom, "block", blockHash)
		blocksInTransit = blocksInTransit[1:]
	} else {
		UTXOSet := blockchain.UTXOSet{chain}
		UTXOSet.Reindex()
	}
}

func RequestBlocks() {
	for _, n := range KnownNodes {
		SendGetBlocks(n)
	}
}








