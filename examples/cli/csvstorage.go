package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/williammoran/economy"
)

func makeCsvStorage() *csvStorage {
	return &csvStorage{}
}

const (
	offerFile = "offers.csv"
	bidFile   = "bids.csv"
	txFile    = "trasnactions.csv"
	priceFile = "prices.csv"
)

type csvStorage struct {
	mutex        sync.Mutex
	offers       map[economy.Symbol]map[uuid.UUID]economy.Offer
	bids         map[economy.BidID]economy.Bid
	transactions []economy.Transaction
	lastBidID    economy.BidID
	lastPrice    map[economy.Symbol]int64
}

func (s *csvStorage) Lock() {
	s.mutex.Lock()
	s.loadAll()
}

func (s *csvStorage) Unlock() {
	s.saveAll()
	s.mutex.Unlock()
}

func (s *csvStorage) AddOffer(o economy.Offer) uuid.UUID {
	o.ID = uuid.New()
	offers := s.offers[o.Symbol]
	if offers == nil {
		offers = make(map[uuid.UUID]economy.Offer)
	}
	offers[o.ID] = o
	s.offers[o.Symbol] = offers
	return o.ID
}

func (s *csvStorage) BestOffer(sym economy.Symbol) (economy.Offer, bool) {
	l := s.offers[sym]
	if len(l) == 0 {
		return economy.Offer{}, false
	}
	o := economy.Offer{Price: 2 ^ 60}
	for _, offer := range l {
		if offer.Price < o.Price {
			o = offer
		}
	}
	return o, true
}

func (s *csvStorage) UpdateOffer(o economy.Offer) {
	l := s.offers[o.Symbol]
	if o.Amount > 0 {
		l[o.ID] = o
	} else {
		delete(l, o.ID)
	}
	s.offers[o.Symbol] = l
}

func (s *csvStorage) AddBid(b economy.Bid) economy.BidID {
	s.lastBidID++
	b.BidID = s.lastBidID
	s.bids[s.lastBidID] = b
	return s.lastBidID
}

func (s *csvStorage) UpdateBid(b economy.Bid) {
	s.bids[b.BidID] = b
}

func (s *csvStorage) GetBid(id economy.BidID) economy.Bid {
	bid, found := s.bids[s.lastBidID]
	if !found {
		log.Panicf("Bid %d not found", id)
	}
	return bid
}

func (s *csvStorage) NewTransaction(t economy.Transaction) {
	t.ID = uuid.New()
	s.transactions = append(s.transactions, t)
}

func (s *csvStorage) LastPrice(symbol economy.Symbol) int64 {
	p, found := s.lastPrice[symbol]
	if found {
		return p
	}
	return 1
}

func (s *csvStorage) SetLastPrice(
	symbol economy.Symbol, price int64,
) {
	s.lastPrice[symbol] = price
}

func (s *csvStorage) AllSymbols() []economy.Symbol {
	l := make(map[economy.Symbol]bool)
	for s := range s.lastPrice {
		l[s] = true
	}
	for s := range s.offers {
		l[s] = true
	}
	var rv []economy.Symbol
	for s := range l {
		rv = append(rv, s)
	}
	return rv
}

func (s *csvStorage) loadAll() {
	offers := loadOffers()
	s.offers = make(map[economy.Symbol]map[uuid.UUID]economy.Offer)
	for _, o := range offers {
		oList := s.offers[o.Symbol]
		if oList == nil {
			oList = make(map[uuid.UUID]economy.Offer)
		}
		oList[o.ID] = o
		s.offers[o.Symbol] = oList
	}
	s.loadBids()
	s.lastPrice = loadPrices()
	s.transactions = loadTransactions()
}

func (s *csvStorage) saveAll() {
	offers := make(map[uuid.UUID]economy.Offer)
	for _, oList := range s.offers {
		for _, o := range oList {
			offers[o.ID] = o
		}
	}
	saveOffers(offers)
	savePrices(s.lastPrice)
	saveBids(s.bids)
	saveTransactions(s.transactions)
	s.offers = nil
	s.bids = nil
	s.transactions = nil
	s.lastPrice = nil
}

func loadOffers() map[uuid.UUID]economy.Offer {
	offers := make(map[uuid.UUID]economy.Offer)
	f, err := os.Open(offerFile)
	if err != nil {
		return offers
	}
	defer f.Close()
	reader := csv.NewReader(f)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return offers
		}
		if err != nil {
			panic(err.Error())
		}
		offer := economy.Offer{
			ID:        uuid.MustParse(record[0]),
			OfferType: economy.OrderType(mustParseByte(record[1])),
			Account:   mustParseInt64(record[2]),
			Symbol:    economy.Symbol(record[3]),
			Price:     mustParseInt64(record[4]),
			Amount:    mustParseInt64(record[5]),
		}
		offers[offer.ID] = offer
	}
}

func saveOffers(offers map[uuid.UUID]economy.Offer) {
	f, err := os.Create(offerFile)
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()
	writer := csv.NewWriter(f)
	defer writer.Flush()
	for _, offer := range offers {
		var r []string
		r = append(r, offer.ID.String())
		r = append(r, fmt.Sprintf("%d", offer.OfferType))
		r = append(r, fmt.Sprintf("%d", offer.Account))
		r = append(r, string(offer.Symbol))
		r = append(r, fmt.Sprintf("%d", offer.Price))
		r = append(r, fmt.Sprintf("%d", offer.Amount))
		writer.Write(r)
	}
}

func loadPrices() map[economy.Symbol]int64 {
	prices := make(map[economy.Symbol]int64)
	f, err := os.Open(priceFile)
	if err != nil {
		return prices
	}
	defer f.Close()
	reader := csv.NewReader(f)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return prices
		}
		if err != nil {
			panic(err.Error())
		}
		symbol := economy.Symbol(record[0])
		prices[symbol] = mustParseInt64(record[1])
	}
}

func savePrices(prices map[economy.Symbol]int64) {
	f, err := os.Create(priceFile)
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()
	writer := csv.NewWriter(f)
	defer writer.Flush()
	for symbol, price := range prices {
		var r []string
		r = append(r, string(symbol))
		r = append(r, fmt.Sprintf("%d", price))
		writer.Write(r)
	}
}

func (s *csvStorage) loadBids() {
	s.bids = make(map[economy.BidID]economy.Bid)
	s.lastBidID = 0
	f, err := os.Open(priceFile)
	if err != nil {
		return
	}
	defer f.Close()
	reader := csv.NewReader(f)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			panic(err.Error())
		}
		id := economy.BidID(mustParseInt64(record[0]))
		if id > s.lastBidID {
			s.lastBidID = id
		}
		bid := economy.Bid{
			BidID:   id,
			BidType: economy.OrderType(mustParseByte(record[1])),
			Account: mustParseInt64(record[2]),
			Symbol:  economy.Symbol(record[3]),
			Price:   mustParseInt64(record[2]),
			Amount:  mustParseInt64(record[2]),
			Status:  mustParseByte(record[1]),
		}
		s.bids[id] = bid
	}
}

func saveBids(bids map[economy.BidID]economy.Bid) {
	f, err := os.Create(bidFile)
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()
	writer := csv.NewWriter(f)
	defer writer.Flush()
	for _, bid := range bids {
		var r []string
		r = append(r, fmt.Sprintf("%d", bid.BidID))
		r = append(r, fmt.Sprintf("%d", bid.BidType))
		r = append(r, fmt.Sprintf("%d", bid.Account))
		r = append(r, string(bid.Symbol))
		r = append(r, fmt.Sprintf("%d", bid.Price))
		r = append(r, fmt.Sprintf("%d", bid.Amount))
		r = append(r, fmt.Sprintf("%d", bid.Status))
		writer.Write(r)
	}
}

func loadTransactions() []economy.Transaction {
	var txs []economy.Transaction
	f, err := os.Open(txFile)
	if err != nil {
		return txs
	}
	defer f.Close()
	reader := csv.NewReader(f)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			return txs
		}
		if err != nil {
			panic(err.Error())
		}
		tx := economy.Transaction{
			ID:      uuid.MustParse(record[0]),
			BidID:   economy.BidID(mustParseInt64(record[1])),
			OfferID: uuid.MustParse(record[2]),
			Price:   mustParseInt64(record[3]),
			Amount:  mustParseInt64(record[4]),
			Date:    time.UnixMilli(mustParseInt64(record[5])),
		}
		txs = append(txs, tx)
	}
}

func saveTransactions(txs []economy.Transaction) {
	f, err := os.Create(txFile)
	if err != nil {
		panic(err.Error())
	}
	defer f.Close()
	writer := csv.NewWriter(f)
	defer writer.Flush()
	for _, tx := range txs {
		var r []string
		r = append(r, tx.ID.String())
		r = append(r, fmt.Sprintf("%d", tx.BidID))
		r = append(r, tx.OfferID.String())
		r = append(r, fmt.Sprintf("%d", tx.Price))
		r = append(r, fmt.Sprintf("%d", tx.Amount))
		r = append(r, fmt.Sprintf("%d", tx.Date.UnixMilli()))
		writer.Write(r)
	}
}

func mustParseByte(v string) byte {
	r, err := strconv.ParseInt(v, 10, 8)
	if err != nil {
		panic(err.Error())
	}
	return byte(r)
}

func mustParseInt64(v string) int64 {
	r, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic(err.Error())
	}
	return r
}