package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/wallet"
)

type Stats struct {
	Combinations   int
	CheckedWallets int
	InvalidSeeds   int
	sync.Mutex
}

func main() {
	// Load BIP39 words from file
	bip39Words, err := loadBIP39Words("english.txt")
	if err != nil {
		log.Fatalln("Error loading BIP39 words:", err)
		return
	}

	client := liteclient.NewConnectionPool()

	// get config
	cfg, err := liteclient.GetConfigFromUrl(context.Background(), "https://ton.org/global.config.json")
	if err != nil {
		log.Fatalln("get config err: ", err.Error())
		return
	}

	// connect to mainnet lite servers
	err = client.AddConnectionsFromConfig(context.Background(), cfg)
	if err != nil {
		log.Fatalln("connection err: ", err.Error())
		return
	}

	// api client with full proof checks
	api := ton.NewAPIClient(client, ton.ProofCheckPolicyFast).WithRetry()
	api.SetTrustedBlockFromConfig(cfg)

	// bound all requests to single ton node
	ctx := client.StickyContext(context.Background())

	// seed words of account, you can generate them with any wallet or using wallet.NewSeed() method
	seedPhrase := "pride pulp party 0 mail invest guilt race insane 0 humble emerge vacant stadium spray gadget gallery modify soon enemy soft luxury power hope"
	words := strings.Split(seedPhrase, " ")

	// Start a ticker to print stats every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Print starting message
	fmt.Println("Starting search wallet")

	// Initialize stats
	stats := &Stats{}

	// Recursive function to replace all "0" with BIP39 words
	var replaceZero func(words []string, index int)
	replaceZero = func(words []string, index int) {
		if index >= len(words) {
			// Log the current combination
			//log.Printf("Checking combination: %s\n", strings.Join(words, " "))

			// Create wallet from seed
			w, err := wallet.FromSeed(api, words, wallet.V4R2)
			if err != nil {
				stats.Lock()
				stats.InvalidSeeds++
				stats.Unlock()
				return
			}

			stats.Lock()
			stats.CheckedWallets++
			stats.Unlock()

			// Get balance
			block, err := api.CurrentMasterchainInfo(context.Background())
			if err != nil {
				log.Println("get masterchain info err: ", err.Error())
				return
			}

			balance, err := w.GetBalance(ctx, block)
			if err != nil {
				log.Println("GetBalance err:", err.Error())
				return
			}

			if balance.Nano().Uint64() > 0 {
				// Save wallet to file
				saveWallet(w.Address().String(), balance.String(), strings.Join(words, " "))
				log.Printf("Found wallet with balance: Address: %s, Balance: %s, Seed Phrase: %s\n", w.Address().String(), balance.String(), strings.Join(words, " "))
			}

			return
		}

		if words[index] == "0" {
			for _, bip39Word := range bip39Words {
				words[index] = bip39Word
				stats.Lock()
				stats.Combinations++
				stats.Unlock()
				replaceZero(words, index+1)
				words[index] = "0" // Reset the word to "0" for the next iteration
			}
		} else {
			replaceZero(words, index+1)
		}
	}

	// Start the recursive function
	go replaceZero(words, 0)

	// Print stats every second
	for range ticker.C {
		stats.Lock()
		fmt.Printf("Кол-во комбинаций слов: %d\n", stats.Combinations)
		fmt.Printf("Кол-во проверенных кошельков: %d\n", stats.CheckedWallets)
		fmt.Printf("Кол-во невалидных сид фраз: %d\n", stats.InvalidSeeds)
		stats.Unlock()
	}
}

func loadBIP39Words(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var words []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		words = append(words, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return words, nil
}

func saveWallet(address string, balance string, seedPhrase string) {
	file, err := os.OpenFile("wallet.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	_, err = file.WriteString(fmt.Sprintf("Address: %s, Balance: %s, Seed Phrase: %s\n", address, balance, seedPhrase))
	if err != nil {
		log.Println("Error writing to file:", err)
	}
}
