package models

type Campaign struct {
	CampaignID   string `json:"campaignId"`
	Advertiser   string `json:"advertiser"`
	BaseURL      string `json:"baseUrl"`
	Enabled      bool   `json:"enabled"`
	Expiration   string `json:"expiration"`
	CampaignType string `json:"campaignType"`
}
