package models

type PromoCode struct {
	CampaignID       string `json:"campaignId"`
	Code             string `json:"promocode"`
	//DeviceID         string `json:"deviceId"`
	RedemptionStatus bool   `json:"redemptionStatus"`
}