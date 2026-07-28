// Package addrs extracts deployed contract addresses from a Foundry broadcast
// file and writes them into .env files (idempotent upsert). Used by the
// found-hub deploy/render steps and the hub bundle emitter.
package addrs

import (
	"encoding/json"
	"fmt"
	"os"
)

type broadcastFile struct {
	Transactions []struct {
		TransactionType string `json:"transactionType"`
		ContractName    string `json:"contractName"`
		ContractAddress string `json:"contractAddress"`
	} `json:"transactions"`
}

// Deploy is one CREATE deployment (name + address), in broadcast order.
type Deploy struct {
	Name    string
	Address string
}

// ParseBroadcastList reads a Foundry `run-latest.json` and returns the CREATE
// deployments in order (preserving duplicates, e.g. two TokenizedCentralBankMoney).
func ParseBroadcastList(path string) ([]Deploy, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var bf broadcastFile
	if err := json.Unmarshal(raw, &bf); err != nil {
		return nil, fmt.Errorf("addrs: parse broadcast %s: %w", path, err)
	}
	var out []Deploy
	for _, tx := range bf.Transactions {
		if tx.TransactionType == "CREATE" && tx.ContractName != "" && tx.ContractAddress != "" {
			out = append(out, Deploy{Name: tx.ContractName, Address: tx.ContractAddress})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("addrs: no CREATE transactions in %s", path)
	}
	return out, nil
}

// ParseBroadcast reads a Foundry `run-latest.json` and returns a map of
// contractName -> address for CREATE transactions (last wins on duplicates).
func ParseBroadcast(path string) (map[string]string, error) {
	list, err := ParseBroadcastList(path)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(list))
	for _, d := range list {
		out[d.Name] = d.Address
	}
	return out, nil
}
