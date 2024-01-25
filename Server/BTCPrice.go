package Server

import "time"

type BTCPrice struct {
	Time  time.Time `json:"timedate"`
	Price float64   `json:"price"`
}
