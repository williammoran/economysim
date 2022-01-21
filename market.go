package market

import "time"

type OrderProcessor interface {
	TryFillBid(MarketStorage, map[OrderType]OrderProcessor, Bid)
	GetAskingPrice(MarketStorage, Offer) int64
}

func MakeMarket(s MarketStorage) *Market {
	return &Market{
		storage: s,
		orderProcessors: map[OrderType]OrderProcessor{
			BidMarket: &marketOrderProcessor{now: time.Now},
		},
	}
}

type Market struct {
	storage         MarketStorage
	orderProcessors map[OrderType]OrderProcessor
}

func (m *Market) Offer(o Offer) {
	m.storage.Lock()
	defer m.storage.Unlock()
	m.storage.AddOffer(o)
}

func (m *Market) Bid(b Bid) BidID {
	m.storage.Lock()
	defer m.storage.Unlock()
	b.BidID = m.storage.AddBid(b)
	m.orderProcessors[b.BidType].TryFillBid(m.storage, m.orderProcessors, b)
	return b.BidID
}

func (m *Market) GetBid(id BidID) Bid {
	m.storage.Lock()
	defer m.storage.Unlock()
	return m.storage.GetBid(id)
}