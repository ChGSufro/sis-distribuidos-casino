package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type HandStatus string

const (
	HandStatusActive     HandStatus = "active"
	HandStatusDealerTurn HandStatus = "dealer_turn"
	HandStatusSettled    HandStatus = "settled"
	HandStatusCancelled  HandStatus = "cancelled"
)

type SubHandStatus string

const (
	SubHandStatusActive    SubHandStatus = "active"
	SubHandStatusStand     SubHandStatus = "stand"
	SubHandStatusBust      SubHandStatus = "bust"
	SubHandStatusBlackjack SubHandStatus = "blackjack"
	SubHandStatusDoubled   SubHandStatus = "doubled"
	SubHandStatusSettled   SubHandStatus = "settled"
	SubHandStatusCancelled SubHandStatus = "cancelled"
)

type HandResult string

const (
	ResultNone         HandResult = ""
	ResultWin          HandResult = "win"
	ResultLose         HandResult = "lose"
	ResultPush         HandResult = "push"
	ResultBlackjackWin HandResult = "blackjack_win"
	ResultSurrender    HandResult = "surrender"
)

type ActionType string

const (
	ActionHit       ActionType = "hit"
	ActionStand     ActionType = "stand"
	ActionDouble    ActionType = "double"
	ActionSplit     ActionType = "split"
	ActionSurrender ActionType = "surrender"
)

type SubHand struct {
	Index          int32         `bson:"index" json:"index"`
	Cards          []Card        `bson:"cards" json:"cards"`
	BetAmountCents int64         `bson:"betAmountCents" json:"bet_amount_cents"`
	HoldID         string        `bson:"holdId" json:"hold_id"`
	DoubleHoldID   string        `bson:"doubleHoldId,omitempty" json:"double_hold_id,omitempty"`
	Status         SubHandStatus `bson:"status" json:"status"`
	Result         HandResult    `bson:"result,omitempty" json:"result,omitempty"`
}

type PlayerAction struct {
	SubHandIndex int32      `bson:"subHandIndex" json:"sub_hand_index"`
	Action       ActionType `bson:"action" json:"action"`
	Timestamp    time.Time  `bson:"timestamp" json:"timestamp"`
}

type HandDocument struct {
	ID                  bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	UserID              string         `bson:"userId" json:"user_id"`
	SubHands            []SubHand      `bson:"subHands" json:"sub_hands"`
	DealerCards         []Card         `bson:"dealerCards" json:"dealer_cards"`
	PlayerActions       []PlayerAction `bson:"playerActions" json:"player_actions"`
	Status              HandStatus     `bson:"status" json:"status"`
	TotalWinAmountCents *int64         `bson:"totalWinAmountCents,omitempty" json:"total_win_amount_cents,omitempty"`
	IdempotencyKey      string         `bson:"idempotencyKey" json:"idempotency_key"`
	CreatedAt           time.Time      `bson:"createdAt" json:"created_at"`
	SettledAt           *time.Time     `bson:"settledAt,omitempty" json:"settled_at,omitempty"`
}

