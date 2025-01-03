package models

type Campaign struct {
    CampaignID    string `json:"campaignId"`
    Advertiser    string `json:"advertiser"`
    BaseURL       string `json:"baseUrl"`
    Active        bool   `json:"active"`
    Expiration    int64  `json:"expiration"`
    CampaignType  string `json:"campaignType"`
    PlatformType  string `json:"platformType"`
}

