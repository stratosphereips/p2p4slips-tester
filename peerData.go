package main

import (
	"fmt"
	"github.com/Pallinder/go-randomdata"
	"strings"
)

type PeerData struct {
	Name         string
	MultiAddress string
	Messages     []string
}

func addNewPeer(peerList *map[string]*PeerData, remoteMA string) *PeerData {
	peerData := contains(peerList, remoteMA)
	if peerData == nil{
		peerData = &PeerData{
			Name:         randName(peerList),
			MultiAddress: remoteMA,
		}
		fmt.Println("Adding new peer!")
		fmt.Printf("Name: %s, Address: %s\n", peerData.Name, peerData.MultiAddress)
		(*peerList)[strings.ToLower(peerData.Name)] = peerData
	}
	return peerData
}

func randName(peerList *map[string]*PeerData) string {
	for {
		name := randomdata.FirstName(randomdata.Male)
		_, ok := (*peerList)[strings.ToLower(name)]

		if !ok {
			return name
		}
	}
}

func contains(peerList *map[string]*PeerData, remoteMA string) *PeerData{
	for _, peer := range *peerList {
		if peer.MultiAddress == remoteMA {
			return peer
		}
	}
	return nil
}