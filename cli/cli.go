package main

import (
	"flag"
	"fmt"
	"github.com/AntonBozhinov/sentinel/blockchain"
	"github.com/AntonBozhinov/sentinel/wallet"
	"log"
	"os"
	"runtime"
)

// CommandLine application
type CommandLine struct {}

func (cli *CommandLine) printUsage() {
	fmt.Println("Usage:")
	fmt.Println(" balance -address ADDRESS - get balance from an address")
	fmt.Println(" create -blockchain ADDRESS - create a blockchain for an address")
	fmt.Println(" print -  prints the blocks in the chain")
	fmt.Println(" send -from ADDRESS -to ADDRESS -amount AMOUNT - Send coins to from one address to another")
	fmt.Println(" reindex - Rebuilds the UTXO set")
}

func (cli *CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) printChain() {
	chain := blockchain.Continue("")
	iter := chain.Iterator()
	for {
		block := iter.Next()
		fmt.Printf("PrevHash: %x\n", block.PrevHash)
		fmt.Printf("Nonce: %d\n", block.Nonce)
		fmt.Printf("Hash: %x\n", block.Hash)
		pow := blockchain.NewProof(block)
		fmt.Printf("Valid: %t\n", pow.Validate())
		for _, tx := range block.Transactions {
			fmt.Println(tx)
		}
		if len(block.PrevHash) == 0 {
			break
		}
	}
}

func (cli *CommandLine) reindexUTXO() {
	chain := blockchain.Continue("")
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{BlockChain: chain}
	UTXOSet.Reindex()
	count := UTXOSet.CountTransactions()
	fmt.Printf("Done! There are %d transactions in the UTXO set. \n", count)
}

func (cli *CommandLine) createBlockChain(address string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("address is not valid")
	}
	chain := blockchain.InitBlockChain(address)
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{chain}
	UTXOSet.Reindex()
	fmt.Println("Finished!")
}

func (cli *CommandLine) getBalance(address string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("address is not valid")
	}
	chain := blockchain.Continue(address)
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{BlockChain: chain}
	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1: len(pubKeyHash) - wallet.ChecksumLength]
	UTXOs := UTXOSet.FindUnspentTransactions(pubKeyHash)
	for _, out := range UTXOs {
		balance += out.Value
	}
	fmt.Printf("Balance of %s: %d\n", address, balance)
}

func (cli *CommandLine) send(from, to string, amount int) {
	if !wallet.ValidateAddress(from) {
		log.Panic("source address is not valid")
	}
	if !wallet.ValidateAddress(to) {
		log.Panic("destination address is not valid")
	}
	chain := blockchain.Continue(from)
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{BlockChain: chain}

	tx := blockchain.NewTransaction(from , to, amount, &UTXOSet)
	block := chain.AddBlock([]*blockchain.CoinTransaction{tx})
	UTXOSet.Update(block)
	fmt.Println("Success!")
}

func (cli *CommandLine) createWallet() {
	wallets, _ := wallet.CreateWallets()
	address := wallets.AddWallet()
	wallets.SaveFile()
	fmt.Printf("New wallet address is: %s\n", address)
}

func (cli *CommandLine) listAddresses() {
	wallets, _ := wallet.CreateWallets()
	addresses := wallets.GetAllAddresses()
	for _, address := range addresses {
		fmt.Println(address)
	}
}

func (cli *CommandLine) run() {
	cli.validateArgs()
	getBalanceCmd := flag.NewFlagSet("balance", flag.ExitOnError)
	getBalanceAddress := getBalanceCmd.String("address", "", "address of the account")

	createCmd := flag.NewFlagSet("create", flag.ExitOnError)
	createBlockChainAddress := createCmd.String("blockchain", "", "address of the blockchain")
	createWallet := createCmd.Bool("wallet", false, "create new wallet")
	
	printCmd := flag.NewFlagSet("print", flag.ExitOnError)

	listCmd := flag.NewFlagSet("list", flag.ExitOnError)
	listWallets := listCmd.Bool("wallets", false, "list wallets")

	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendAmount := sendCmd.Int("amount", 0, "Amount of coins")

	reindexCmd := flag.NewFlagSet("reindex", flag.ExitOnError)

	switch os.Args[1] {
	case "reindex":
		err := reindexCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "list":
		err := listCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "balance":
		err := getBalanceCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "create":
		err := createCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "print":
		err := printCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	}

	if reindexCmd.Parsed() {
		cli.reindexUTXO()
	}

	if listCmd.Parsed() {
		if *listWallets {
			cli.listAddresses()
		} else {
			listCmd.Usage()
			runtime.Goexit()
		}
	}

	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			runtime.Goexit()
		}
		cli.getBalance(*getBalanceAddress)
	}

	if createCmd.Parsed() {
		if *createBlockChainAddress == "" && !*createWallet {
			createCmd.Usage()
			runtime.Goexit()
		}
		if len(*createBlockChainAddress) > 0{
			cli.createBlockChain(*createBlockChainAddress)
		}
		if *createWallet {
			cli.createWallet()
		}
	}
	if printCmd.Parsed() {
		cli.printChain()
	}
	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			runtime.Goexit()
		}

		cli.send(*sendFrom, *sendTo, *sendAmount)
	}

}

func main() {
	defer os.Exit(0)
	cli := CommandLine{}
	cli.run()
}
